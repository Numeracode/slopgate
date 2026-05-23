# Slopgate Memory

This file tracks key information about the slopgate project for future agent sessions.

## Project Overview

**Repository**: `/srv/storage/repo/slopgate`
**Purpose**: Pre-commit gate for catching AI-generated code failure patterns (slop)
**Positioning**: Runs locally in milliseconds before hosted AI review tools like CodeRabbit
**Language**: Go 1.22+
**Key Dependency**: BurntSushi/toml for config parsing

## Current Version

- **v0.0.20** (merged to main, 2026-05-23)
- 151 total rules (141 diff-based + 10 AST semantic)

## Benchmark Status

- **Current overlap rate**: ~10–18% across recent whimsy PRs (re-verified 2026-05-23 from 502 benchmark files)
- **Recent data** (last 6 PRs with CR comments): whimsy #658 6.5%, #659 14.3%, #695 14.3%, #696 50%, #741 33.3%, #835 0% (CR only 2 comments)
- **Note**: The previous 40% figure (2026-05-01) was stale — it reflected a single optimistic benchmark, not the sustained average
- **Maturity threshold**: 80% overlap to consider replacing CodeRabbit
- **Gap analysis**: slopgate catches syntactic patterns; CodeRabbit catches semantic/logic bugs
- **Benchmark archive**: `/srv/storage/shared/slopgate-benchmarks/`
- **Benchmark v2**: `scripts/benchmark_review.py` and the shared `run-slopgate-benchmark` wrapper now benchmark in an isolated worktree, score open PRs against the real PR head, and emit legacy/all-comments plus actionable/Sentry-aware overlap metrics.

## Rule Categories

### Syntactic Slop (v0.0.1-v0.0.4)
- SLP001-SLP023: All syntactic patterns (empty tests, unused imports, debug prints, etc.)
- Fast regex-based detection on staged diff

### Semantic Patterns (v0.0.6-v0.0.7 additions)
- SLP024: Webhook ACK on failure (block) - catches `catch { console.error; res.status(200) }`
- SLP025: URL concatenation without validation (warn) - catches `${URL}${Path}` patterns
- SLP026: SQL NULL check without sentinel (warn) - catches `WHERE hash IS NOT NULL` without exclusion
- SLP027: Async function sync throw (warn) - catches `async function { throw }` patterns
- SLP030: Query sentinel exclusion (warn) - catches `.only/.first/.last()` without `where('hash', '!=', 'marker')`

### Noise Reduction (SLP017 tuning)
- Exempts HTTP status codes (200, 400, 500, etc.) in `.status()` context
- Exempts common pagination limits (10, 50, 100) in LIMIT/batch context
- Still flags truly magic numbers like `86.9` tax rate

## Key Files

- `cmd/slopgate/main.go` - CLI entrypoint
- `pkg/rules/registry.go` - Rule registration
- `pkg/rules/slp*.go` - Individual rule implementations
- `pkg/diff/parser.go` - Unified diff parsing
- `pkg/config/` - TOML config handling

## Integration Points

1. **Pre-commit**: `slopgate --staged --no-color`
2. **Pre-push**: Check if PR exists, run benchmark
3. **Post-merge**: Run benchmark on merged PRs
4. **codero-finish Phase 6.5**: After CodeRabbit poll, run slopgate benchmark

## Benchmark Scripts

- `scripts/benchmark_review.py` - Canonical benchmark implementation
- `scripts/benchmark-coderabbit.sh` - Thin local wrapper around the canonical implementation
- `/srv/storage/shared/agent-toolkit/bin/run-slopgate-benchmark` - Shared wrapper that delegates to the canonical implementation
- `/srv/storage/shared/agent-toolkit/bin/benchmark-aggregator` - Collects legacy plus actionable/Sentry-aware benchmark totals

## Development Workflow

1. Create branch: `feat/semantic-rules-vX.Y.Z` or `fix/slpXXX-description`
2. Add rule in `pkg/rules/slpXXX.go` with `Rule` interface
3. Add tests in `pkg/rules/slpXXX_test.go` using `parseDiff(t, "...")`
4. Register in `pkg/rules/registry.go` `Default()` function
5. Update README rule table and version
6. Run `go test ./pkg/rules/...`
7. Update registry_test.go count

## GitHub Token

Available via gitconfig URL rewrite (see ~/.gitconfig for details)

## Next Improvement Areas

1. **Semantic rules expansion**: More patterns from CodeRabbit gap analysis
2. **Cross-language support**: Python async patterns, Go error handling patterns
3. **Config tuning**: Per-project rule severity profiles
4. **CI integration**: GitHub Actions workflow for PR checks

## Memory Updates

- 2026-04-18: Added SLP024-SLP027, tuned SLP017, version bump to v0.0.6
- 2026-05-01: Added Benchmark v2 and the canonical benchmark scripts; tuned SLP007, SLP017, SLP019, and SLP068; added SLP136-SLP140; bumped version to v0.0.17
- 2026-05-02: Noise reduction wave (SLP003, SLP007, SLP017, SLP059); added SLP141-SLP142; bumped version to v0.0.18
- 2026-05-23: Stamped v0.0.19 (SLP143-SLP148) and v0.0.20 (SLP148 tune, SLP151, SLP152) as released; removed stale `slopgate_new` binary artifact; corrected benchmark overlap from stale 40% to current ~10-18% across recent PRs (re-verified from 502 archived benchmarks)
