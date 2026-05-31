// Command slopgate runs a pre-commit gate for AI-generated code slop.
//
// Typical usage:
//
//	slopgate --staged                  # pre-commit check on staged diff
//	slopgate --base main               # scan a branch against main
//	slopgate --format json --staged    # machine-readable output
//
// Exit codes:
//
//	0 - no blocking findings (clean or only warn/info)
//	1 - at least one blocking finding
//	2 - git or IO error
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/messagesgoel-blip/slopgate/pkg/config"
	"github.com/messagesgoel-blip/slopgate/pkg/diff"
	"github.com/messagesgoel-blip/slopgate/pkg/report"
	"github.com/messagesgoel-blip/slopgate/pkg/rules"
)

// gitTimeout caps how long readGitDiff waits for git to finish. No
// legitimate git-diff on a local repo takes this long; if it does,
// something is wrong (lock file, broken index, NFS hang).
const gitTimeout = 30 * time.Second

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("slopgate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		staged      bool
		base        string
		repoDir     string
		format      string
		noColor     bool
		configPath  string
		listRules   bool
		minSeverity string
	)
	fs.BoolVar(&staged, "staged", false, "scan the staged diff (pre-commit mode)")
	fs.StringVar(&base, "base", "", "scan the diff against this base revision (e.g. main)")
	fs.StringVar(&repoDir, "C", "", "run git from this directory instead of cwd")
	fs.StringVar(&format, "format", "text", "output format: text or json")
	fs.BoolVar(&noColor, "no-color", false, "disable ANSI colors in text output")
	fs.StringVar(&configPath, "config", "", "path to .slopgate.toml config file")
	fs.BoolVar(&listRules, "list-rules", false, "list all registered rules and exit")
	fs.StringVar(&minSeverity, "min-severity", "", "minimum severity to report: block|warn|info (default all)")

	fs.Usage = func() {
		fmt.Fprintln(stderr, "slopgate: catches AI-generated code slop on staged or branch diffs")
		fmt.Fprintln(stderr, "")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Environment variable overrides flag default.
	if minSeverity == "" {
		minSeverity = os.Getenv("SLOPGATE_MIN_SEVERITY")
	}

	// Handle --list-rules early — no diff or config needed.
	if listRules {
		reg := rules.Default()
		for _, rule := range reg.All() {
			qMark := ""
			if rules.IsQuarantined(rule.ID()) {
				qMark = "\t[QUARANTINED]"
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s%s\n", rule.ID(), rule.DefaultSeverity(), rule.Description(), qMark)
		}
		for _, rule := range reg.AllSemantic() {
			qMark := ""
			if rules.IsQuarantined(rule.ID()) {
				qMark = "\t[QUARANTINED]"
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s [AST]%s\n", rule.ID(), rule.DefaultSeverity(), rule.Description(), qMark)
		}
		return 0
	}

	if staged && base != "" {
		fmt.Fprintln(stderr, "slopgate: --staged and --base are mutually exclusive")
		return 2
	}

	if format != "text" && format != "json" {
		fmt.Fprintf(stderr, "slopgate: unknown format %q (expected text or json)\n", format)
		return 2
	}

	if !staged && base == "" {
		// Default to staged so that pre-commit hook config can just be `slopgate`.
		staged = true
	}

	diffBytes, err := readGitDiff(repoDir, staged, base)
	if err != nil {
		fmt.Fprintf(stderr, "slopgate: %v\n", err)
		return 2
	}

	parsed, err := diff.Parse(bytes.NewReader(diffBytes))
	if err != nil {
		fmt.Fprintf(stderr, "slopgate: diff parse: %v\n", err)
		return 2
	}
	if root, rootErr := repoRoot(repoDir); rootErr == nil {
		parsed.RepoRoot = root
	}
	parsed.Staged = staged
	if staged {
		parsed.SnapshotRef = ":"
	} else if base != "" {
		parsed.SnapshotRef = "HEAD"
	} else {
		parsed.SnapshotWorktree = true
	}

	// Apply .slopgateignore if present at the repo root.
	ignorePatterns, err := loadIgnorePatterns(repoDir)
	if err != nil {
		fmt.Fprintf(stderr, "slopgate: %v\n", err)
		return 2
	}
	parsed = diff.FilterIgnored(parsed, ignorePatterns)

	// Load config if provided or auto-discovered.
	var cfg *config.Config
	if configPath != "" {
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "slopgate: load config %s: %v\n", configPath, err)
			return 2
		}
	} else {
		// Auto-discover .slopgate.toml from the working directory.
		searchDir := repoDir
		if searchDir == "" {
			searchDir, _ = os.Getwd()
		}
		discovered, err := config.Discover(searchDir)
		if err != nil {
			fmt.Fprintf(stderr, "slopgate: discover config: %v\n", err)
			return 2
		}
		if discovered != "" {
			cfg, err = config.Load(discovered)
			if err != nil {
				fmt.Fprintf(stderr, "slopgate: load config %s: %v\n", discovered, err)
				return 2
			}
		}
	}

	// Apply --min-severity flag override (highest precedence).
	if minSeverity != "" {
		if cfg == nil {
			cfg = &config.Config{}
		}
		cfg.MinSeverity = minSeverity
		// Validate the flag value.
		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(stderr, "slopgate: %v\n", err)
			return 2
		}
	}

	findings := rules.Default().Run(parsed, cfg)

	switch format {
	case "json":
		if err := report.WriteJSON(stdout, findings); err != nil {
			fmt.Fprintf(stderr, "slopgate: write json: %v\n", err)
			return 2
		}
	default:
		color := !noColor && isTerminal(stdout)
		report.WriteText(stdout, findings, color)
	}

	for _, f := range findings {
		if f.Severity == rules.SeverityBlock {
			return 1
		}
	}
	return 0
}

// readGitDiff shells out to git for the requested diff, returning the
// raw bytes. We do not depend on a Go git library — git itself is the
// source of truth, and every slopgate user has it.
func readGitDiff(dir string, staged bool, base string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	var gitArgs []string
	if dir != "" {
		gitArgs = append(gitArgs, "-C", dir)
	}
	gitArgs = append(gitArgs, "diff", "--no-color", "-U3")
	switch {
	case staged:
		gitArgs = append(gitArgs, "--cached")
	case base != "":
		gitArgs = append(gitArgs, base+"...HEAD")
	}

	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("git diff timed out after %v", gitTimeout)
		}
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("git diff failed: %s", stderr.String())
		}
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	return out, nil
}

// loadIgnorePatterns reads .slopgateignore from the repo root and
// returns the parsed glob list. A missing file is not an error.
func loadIgnorePatterns(repoDir string) ([]string, error) {
	root, err := repoRoot(repoDir)
	if err != nil {
		// No git / not a repo — fall back to empty ignore list.
		return nil, nil
	}
	path := filepath.Join(root, ".slopgateignore")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open .slopgateignore: %w", err)
	}
	defer f.Close()
	patterns, err := diff.ParseIgnoreFile(f)
	if err != nil {
		return nil, fmt.Errorf("parse .slopgateignore: %w", err)
	}
	return patterns, nil
}

// repoRoot returns the absolute path to the git repo root for the
// given working directory, or an error if the directory is not a git
// repository.
func repoRoot(dir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	var args []string
	if dir != "" {
		args = append(args, "-C", dir)
	}
	args = append(args, "rev-parse", "--show-toplevel")
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("git rev-parse timed out after %v", gitTimeout)
		}
		return "", err
	}
	return string(bytes.TrimSpace(out)), nil
}

// isTerminal reports whether w is a terminal. It is intentionally
// naive — we only use it to decide whether to color output and getting
// it wrong means uglier text in a file, never a functional failure.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
