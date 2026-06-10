# slopgate

**slopgate** is a fast, local pre-commit gate that catches AI-generated code slop in git diffs.

It flags high-signal failure patterns before hosted review tools run — unfinished stubs, swallowed errors, unsafe SQL construction, missing test updates, weak auth checks, and other recurring issues that are cheap to catch locally. It runs in milliseconds, so it fits in a pre-commit hook without slowing anyone down.

slopgate is not a replacement for hosted review (CodeRabbit and similar) or for human review. It is a quality floor that removes the obvious problems first, so deeper review can focus on what matters.

## How it works

slopgate parses a git diff and runs each changed line through a registry of rules. Every finding has one of three severities:

| Severity | Effect |
|---|---|
| `block` | Fails the run (exit 1) — meant to stop a commit |
| `warn` | Reported; does not fail the run |
| `info` | Reported; does not fail the run |

Because it works from the diff alone, slopgate is fast and broadly language-aware — Go, TypeScript/JavaScript, Python, Java, Rust, and Ruby — with deeper AST-based checks for Go.

## Install

```bash
go install github.com/messagesgoel-blip/slopgate/cmd/slopgate@latest
```

Requires Go 1.22 or newer.

## Usage

```bash
slopgate                 # scan staged changes (default)
slopgate --staged        # same, explicit
slopgate --base main     # scan main...HEAD
slopgate --format json   # machine-readable output
slopgate --list-rules    # print the rule catalog
```

### Flags

| Flag | Description |
|---|---|
| `--staged` | Scan staged changes (`git diff --cached`) — the default |
| `--base <ref>` | Scan `<ref>...HEAD` instead |
| `-C <dir>` | Run git from a specific directory |
| `--format text\|json` | Output format (default `text`) |
| `--no-color` | Disable ANSI colour in text output |
| `--config <path>` | Use a specific config file |
| `--list-rules` | Print the rule catalog and exit |
| `--min-severity info\|warn\|block` | Only report findings at or above this severity |

`--staged` and `--base` are mutually exclusive.

### Exit codes

| Code | Meaning |
|---|---|
| `0` | No blocking findings |
| `1` | One or more blocking findings |
| `2` | Tool, config, or git error |

## Configuration

slopgate works with zero configuration. To adjust rule behaviour, add a `.slopgate.toml` at the repo root:

```toml
# Turn a rule off
[rules.SLP014]
ignore = true

# Change a rule's severity: block | warn | info | off
[rules.SLP012]
severity = "warn"

# Exempt paths from a single rule
[rules.SLP007]
ignore_paths = ["**/*_test.go"]
```

Config is discovered by walking up from the working directory to the repo root (`.git` or `go.mod`). `--config` overrides discovery; if no config is found, defaults apply.

To skip files entirely, list glob patterns in `.slopgateignore`:

```text
vendor/**
**/migrations/**
```

## Integration

### Pre-commit hook

```bash
# .git/hooks/pre-commit
slopgate --staged --no-color
```

### CI

```bash
slopgate --no-color --base origin/main
```

With shallow clones, fetch full history (`fetch-depth: 0`) so the base ref resolves.

## Rules

slopgate ships 164 registered rules (10 quarantined). `slopgate --list-rules` prints the authoritative catalog with each rule's ID, severity, description, and quarantine status.

| Family | IDs | Focus |
|---|---|---|
| Core diff checks | `SLP001`–`SLP070` | test quality, code hygiene, safety, API and data smells |
| Go AST checks | `SLP071`–`SLP080` | Go semantic hazards — nil, SQL injection, races, ignored errors |
| Extended checks | `SLP081`–`SLP162` | framework, API, auth, audit, pagination, concurrency, dead-code, test-completeness, parseInt truncation, useEffect FOUC, code-quality splits (SLP160–162) |
| Reviewer gap closure | `SLP210`–`SLP222` | conflicting Tailwind utilities, setState-before-async, double-submit race, regex empty-match, React Query no-error-check, OpenAPI spec drift, shallow error logging, empty path params, missing chunked transfer handling, data race on shared state, filepath.Walk without ctx cancellation, exec.Command without stderr, UTF-16/BOM without decoding |
| Semantic bug checks | `SLP202`–`SLP209` | high-signal runtime bugs — nil dereference, DB constraints, OpenAPI merge-order, swallowed promises, missing rollbacks, default-param ordering, async arrow missing returns |

### Quarantined rules

Ten rules are quarantined (disabled by default) because they produced zero overlap with reviewer feedback across all benchmark runs. They can be re-enabled via config:

```toml
[rules.SLP068]
ignore = false   # re-enable a quarantined rule
```

Quarantined rules: `SLP010`, `SLP019`, `SLP007`, `SLP033`, `SLP053`, `SLP068`, `SLP081`, `SLP089`, `SLP113`, `SLP118`.

## Contributing

To add a rule:

1. Add `pkg/rules/slpXXX.go` implementing `Rule` (or `SemanticRule` for AST-based checks).
2. Add `pkg/rules/slpXXX_test.go` with table-driven tests built from the `parseDiff` helper.
3. Register it in `Default()` in `pkg/rules/registry.go`.
4. Note the change in `CHANGELOG.md`.

`scripts/benchmark_review.py <repo-path> <pr-number>` measures a rule set's overlap with hosted-review feedback on a real pull request — useful when tuning rule precision.

### Rule pruning benchmark

Use the archived benchmark corpus to decide which rules should stay in the default gate:

```bash
scripts/benchmark-rule-scorecard.py --limit 20 \
  --output-dir ./benchmark-results
```

This writes `rule_scorecard.csv`, `pr_findings.csv`, `review_misses.csv`, and `pruning_candidates.md`. Review `rule_scorecard.csv` and set `manual_decision` consistently with scorecard outcomes such as `keep`, `watch`, `quarantine`, `disable_candidate`, or `review`.

To verify recent merged PRs are represented in the benchmark archive:

```bash
scripts/benchmark-pr-coverage.py messagesgoel-blip/slopgate --limit 20 --min-pr 63 --fail-on-missing
```

## License

MIT
