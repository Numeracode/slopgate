# SLO-208 — Benchmark Signal Split and Whimsy Gap Closure Spec

**Date:** 2026-05-17
**Status:** Draft
**Roadmap task:** `SLO-208`
**Arc:** `P4`
**Tracking issue:** `#62`
**Target repo:** `messagesgoel-blip/slopgate`
**Primary benchmark window analyzed:** 2026-05-15 09:27 EDT to 2026-05-17 09:27 EDT

## Problem

Slopgate's recent benchmark output is dominated by low-signal findings and does not align well with hosted review tools on current Whimsy work.

Observed in the last 48 hours:

- 12 benchmark runs across Codero, SkillSwap, and Whimsy
- 1,256 Slopgate findings
- 44 CodeRabbit comments
- 11 actionable CodeRabbit comments
- 9 Sentry findings
- 3 all-comment overlaps
- 1 actionable overlap
- 1 actionable-plus-Sentry overlap

Weighted performance in this window:

- All-comment coverage: `3 / 44 = 6.8%`
- Actionable coverage: `1 / 11 = 9.1%`
- Actionable-plus-Sentry coverage: `1 / 19 = 5.3%`
- All-comment precision proxy: `3 / 1256 = 0.24%`

Whimsy drives nearly all of the problem:

- 10 of the 12 runs were Whimsy
- Those 10 runs produced 1,199 Slopgate findings
- Whimsy produced only 1 all-comment overlap and 0 actionable or Sentry overlaps

Recent rule volume is dominated by:

- `SLP068` duplicate logic block
- `SLP035` style/code-quality bucket
- `SLP053` config-value rationale comments
- `SLP017` magic numbers

The benchmark script also caps `sg_only_details`, `review_only_details`, and `overlap_details` at 50 entries per run, which means the noisiest runs are partially truncated in the archive.

## Goals

1. Make the benchmark scoreboard reflect high-signal rule performance instead of style/advisory noise.
2. Reduce Whimsy benchmark spam from a small number of noisy rules.
3. Preserve real blocker coverage from rules such as `SLP095`, `SLP098`, `SLP102`, `SLP113`, and `SLP118`.
4. Add a small first wave of semantic rules for contract and behavior drift exposed by Whimsy misses.

## Non-Goals

- Replacing CodeRabbit or Sentry in this arc
- Rewriting the whole benchmark framework
- Solving all semantic review gaps in one pass
- Changing repo policy or workspace hygiene rules

## Findings Summary

### Benchmark pollution

The worst recent runs are mostly advisory noise:

- Whimsy PR `#605` generated `246` and `247` findings with zero overlap; the visible findings are almost entirely `SLP035` and a few `SLP017`.
- Whimsy PRs `#602` and `#603` are dominated by `SLP068` duplicate-block warnings, with additional `SLP053` and `SLP017`.
- `SLP056` flags OpenAPI schema text inside generated contract artifacts, which does not appear to represent real credential leakage.
- `SLP033` and `SLP081` produce likely false positives on modern React TSX code using the automatic JSX runtime.

### High-value misses

Whimsy review-only and Sentry-only findings cluster around:

- OpenAPI request/response contract drift
- OpenAPI merge-order and schema-composition bugs
- API and SDK response-shape mismatches
- Runtime behavior bugs in scripts and route code
- Security and route semantics not visible from shallow diff heuristics

## Scope

This arc has three deliverables:

1. Benchmark scoring split into signal tiers
2. Noise reduction for the top offending existing rules
3. First semantic gap-closure pass for Whimsy-exposed contract and behavior drift

## Design

### 1. Benchmark scoring changes

Add signal-tiered scoring to the benchmark JSON and summary flow.

Current benchmark metrics over-count low-value rules because every Slopgate finding contributes equally to precision proxy.

New score groups:

- `all_rules`: existing behavior retained for backwards compatibility
- `block_warn_only`: excludes all `info` findings from precision and overlap denominator
- `benchmark_eligible`: excludes rules explicitly marked advisory or excluded from parity scoring

New metadata additions:

- Per-finding benchmark eligibility derived from rule metadata
- Rule-level benchmark mode: `parity`, `advisory`, or `disabled`
- Count and precision/coverage metrics for each mode

Expected effect:

- Whimsy PR `#605` should stop looking catastrophically bad due solely to `info` noise
- Future summaries should show whether poor performance is caused by signal misses or advisory overproduction

### 2. Rule noise reduction

#### `SLP068` duplicate logic block

Current issue:

- Dominant source of Whimsy spam in PRs `#602`, `#603`, and `#606`
- Very sensitive to repeated schema or route patterns

Required changes:

- Exclude generated contract and schema artifacts
- Exclude JSON and OpenAPI-like artifacts by default
- Raise the matching threshold beyond the current fixed 8-line window, or require more discriminating structure
- Collapse duplicate detections so one repeated pattern yields one finding cluster, not many line-local findings

Acceptance target:

- Reduce visible `SLP068` findings in the recent Whimsy benchmark corpus by at least 70%
- Preserve detection on genuinely duplicated source logic in non-generated application code

#### `SLP035` style/code-quality bucket

Current issue:

- Floods the benchmark with `info` findings that do not represent hosted-review parity

Required changes:

- Mark `SLP035` as advisory-only for benchmark parity
- Exclude `SLP035` from `benchmark_eligible` scoring
- Keep reporting it in CLI output unless locally disabled by config

Acceptance target:

- `SLP035` remains usable as local hygiene feedback
- `SLP035` no longer distorts benchmark precision claims

#### `SLP053` config rationale comments

Current issue:

- Advisory-only rule treated as parity signal

Required changes:

- Mark `SLP053` as advisory-only for benchmark parity
- Keep severity as `info`

#### `SLP017` magic numbers

Current issue:

- Recent overlaps came mostly from `SLP017`, but those matches are weak evidence of true review parity

Required changes:

- Keep `SLP017` active
- Exclude `SLP017` from the main parity score unless its severity is overridden above `info`
- Continue reporting it in advisory metrics

#### `SLP056` hardcoded secrets

Current issue:

- Flags generated OpenAPI artifacts where property names or schema fields resemble secrets

Required changes:

- Ignore generated contract files such as `docs/contracts/openapi/**`
- Add context checks for schema/property declaration contexts before flagging
- Retain strict behavior for source files and obvious credential assignments

Acceptance target:

- Remove the observed false positives in Whimsy OpenAPI artifacts
- Preserve block-level behavior in real source files

#### `SLP033` and `SLP081` React import assumptions

Current issue:

- Likely false positives on TSX/JSX with the automatic JSX runtime and modern type imports

Required changes:

- Detect React automatic runtime patterns and skip legacy-import requirements where appropriate
- Treat type-only imports correctly
- Avoid warning solely because `React` default import is absent in modern TSX

Acceptance target:

- Remove the PR `#609` false-positive cluster on `MDXProvider.tsx`

### 3. Semantic gap-closure rules

Add a small first wave of rules tuned to recurring Whimsy misses.

#### New rule: OpenAPI response/request secret-field split

Detect when a response schema exposes write-only credential fields such as `smtp_pass` while an adjacent `*_set` or status flag indicates the intended safe pattern.

Severity:

- `warn`

#### New rule: response-shape drift between API wrapper and consumer

Detect diffs where a client or SDK function changes from returning unwrapped data to `{ ok, data }`-style wrappers, or vice versa, without corresponding consumer updates in the same change.

Severity:

- `block` or `warn` depending on confidence

#### New rule: OpenAPI merge-order override hazard

Detect object merge patterns where hardcoded path maps overwrite detailed annotations or generated entries due to merge order.

Severity:

- `warn`

#### New rule: order-sensitive schema/test assertions for non-semantic arrays

Detect exact-order assertions on OpenAPI `required` or parameter-name arrays where order is not semantically meaningful.

Severity:

- `info` or `warn`

## File Changes

Likely touched files:

- `scripts/benchmark_review.py`
- `scripts/benchmark-compare.py`
- `pkg/rules/registry.go`
- `pkg/rules/rule.go` or equivalent rule metadata location
- `pkg/rules/slp068.go`
- `pkg/rules/slp035.go`
- `pkg/rules/slp053.go`
- `pkg/rules/slp017.go`
- `pkg/rules/slp056.go`
- `pkg/rules/slp033.go`
- `pkg/rules/slp081.go`
- New `pkg/rules/slpXXX.go` and matching tests for any semantic additions

## Implementation Plan

### Phase 1: benchmark scoring split

1. Add rule metadata for benchmark eligibility
2. Update benchmark aggregation to emit tiered metrics
3. Preserve current top-level metrics for compatibility
4. Add regression tests for benchmark scoring output

### Phase 2: noise reduction

1. Retune `SLP068`
2. Mark `SLP035`, `SLP053`, and `SLP017` advisory for parity scoring
3. Add context/path filtering to `SLP056`
4. Modernize `SLP033` and `SLP081`

### Phase 3: semantic gap closure

1. Implement one to three highest-confidence new semantic rules
2. Benchmark against recent Whimsy PR corpus
3. Keep only rules with acceptable precision on the sampled corpus

## Validation

Validation must include:

1. Unit tests for all touched rules
2. Regression tests for benchmark scoring modes
3. Re-run benchmark archive comparisons against the recent Whimsy corpus:
   - PRs `#602`, `#603`, `#605`, `#606`, `#607`, `#609`, `#610`
4. Report before/after on:
   - total findings
   - `block_warn_only` findings
   - `benchmark_eligible` findings
   - overlap and coverage by tier

## Acceptance Criteria

The arc is complete when all of the following are true:

- Benchmark output reports `all_rules`, `block_warn_only`, and `benchmark_eligible` metrics
- `SLP035`, `SLP053`, and `SLP017` no longer distort the main parity score
- `SLP068` visible noise drops materially on the recent Whimsy corpus
- `SLP056` no longer flags the observed generated OpenAPI artifact cases
- `SLP033` and `SLP081` no longer produce the observed modern-React false positives
- At least one Whimsy-exposed semantic gap has a new targeted rule with tests

## Risks

- Over-excluding rules from benchmark parity could hide real value if the eligibility split is too aggressive
- `SLP068` tuning could suppress legitimate duplication findings
- Semantic rules can easily become brittle if diff context is too shallow

## Open Questions

1. Should advisory rules remain in the default CLI summary, or move behind a flag?
2. Should generated artifact ignores live in rule logic, config defaults, or both?
3. Should benchmark truncation stay at 50 detail rows per stream, or should summaries also include uncapped rule histograms?
