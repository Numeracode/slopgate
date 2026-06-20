# Retired & Consolidated Rules

## Policy

Rules are retired from the default rule set when benchmarked data shows
they produce high noise-to-signal ratios on real repositories. Retired
source files are kept in the repo with a `//go:build ignore_retired`
build tag — they compile only when explicitly requested (`-tags ignore_retired`).

Rules are **consolidated** when their concern is fully covered by a
newer, more precise rule. Consolidated files are deleted from the repo.

## Curation Log

### 2026-06-20 — Whimsy benchmark curation

Trigger: Whimsy 15-day benchmark showed 7 rules with <5% useful findings.

**Retired (build-tagged):**

| Rule | Previous findings/10 PRs | Reason |
|------|------------------------|--------|
| SLP043 | 244 | Duplicate JSON key heuristic fires on any struct with >2 fields sharing a prefix. Only 1 real bug pattern confirmed, 243 noise. |
| SLP050 | 175 | Fires on every new function without param validation, including trivial getters/setters. No real bugs found. |
| SLP055 | 45 | "Function has N conditionals without comments" — pure style heuristic. No bugs. |

**Consolidated into SLP223 (deleted):**

| Rule | Previous findings/10 PRs | Reason |
|------|------------------------|--------|
| SLP044 | 26 | `_ = err` pattern — fully covered by SLP223's targeted ignored-error detection |
| SLP065 | 117 | Generic "ignored error return" — overlaps SLP044/SLP114/SLP120; SLP223 covers this with better precision (Close/Remove/MarkRun* etc.) |
| SLP114 | 96 | "Error-returning function called as statement" — same class as SLP065 |
| SLP120 | 14 | "Discarded value with `_ =`" — fully covered by SLP223 |

**Narrowed:**

| Rule | Change | Reason |
|------|--------|--------|
| SLP227 | Exclude strings <6 chars and HTTP method literals | Was flagging "GET"/"POST"/"run"/"config" etc. — 17 of 18 findings were noise from test helper/OpenAPI code |

## Restoring a Retired Rule

To temporarily re-enable a retired rule for a specific scan:

```bash
go run -tags ignore_retired ./cmd/slopgate --base origin/main
```

To permanently restore, remove the `//go:build ignore_retired` tag and
add the rule registration back to `registry.go`.
