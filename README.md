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

slopgate ships 151 registered rules. `slopgate --list-rules` prints the authoritative catalog with each rule's ID, severity, and description.

| Family | IDs | Focus |
|---|---|---|
| Core diff checks | `SLP001`–`SLP070` | test quality, code hygiene, safety, API and data smells |
| Go AST checks | `SLP071`–`SLP080` | Go semantic hazards — nil, SQL injection, races, ignored errors |
| Extended checks | `SLP081`–`SLP152` | framework, API, auth, audit, pagination, concurrency, dead-code, and test-completeness patterns |
| Semantic bug checks | `SLP202`, `SLP203`, `SLP204`, `SLP205`, `SLP207` | high-signal runtime bugs — nil dereference, DB constraints, OpenAPI merge-order overrides, swallowed promise failures, missing rollbacks |

Rules `SLP081` and `SLP033` handle React/TypeScript JSX import behavior for the modern automatic runtime. `SLP081` allows plain JSX without a `React` import, but still flags explicit `React.*` namespace usage unless the file imports `React` through a default or namespace binding. `SLP033` checks import availability using visible diff context and the file snapshot when the import sits outside the changed hunk.

`SLP017` magic-number findings are intentionally scoped to public API, configuration, and business-domain literals so incidental local arithmetic does not create review noise.

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
  --output-dir /srv/storage/shared/slopgate-benchmarks/pruning-scorecard
```

This writes `rule_scorecard.csv`, `pr_findings.csv`, `review_misses.csv`, and `pruning_candidates.md`. Review `rule_scorecard.csv` and set `manual_decision` consistently with scorecard outcomes such as `keep`, `watch`, `quarantine`, `disable_candidate`, or `review`.

To verify recent merged PRs are represented in the benchmark archive:

```bash
scripts/benchmark-pr-coverage.py messagesgoel-blip/slopgate --limit 20 --min-pr 63 --fail-on-missing
```

## License

MIT
