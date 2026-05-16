# Slopgate Overlap Expansion Roadmap

**Goal:** Increase slopgate overlap with CodeRabbit & Sentry from ~23% → 35–40% while maintaining sub-100ms rule-based performance.

**Performance target:** slopgate runs in ~20ms; CR takes ~15 minutes. Replace CI wait time with instant local feedback.

---

## Phase 1 — Close Sentry Bug Gap (P1, Week 1–2)

These catch production crashes before they reach Sentry.

| Task ID | Rule | Description | Effort | Expected Impact | Status |
|---------|------|-------------|--------|-----------------|--------|
| **SLO-202** | SLP202 | Null-deref before property access — detect `obj.prop` where `obj` is nullable without guard | Low | High — Sentry's #1 bug class (~5–10 overlap/100runs) | ✅ Complete (committed + variable-aware guard fix) |
| **SLO-203** | SLP203 | DB constraint violation risk — INSERT/UPDATE without ON CONFLICT on unique keys | Medium | High — Sentry DB errors #2 (~3–5 overlap) | ✅ Complete (committed) |
| **SLO-204** | SLP204 | Silent promise failure mask — catch returns success masking inner error | Medium-High | Medium — data loss bugs | ✅ Complete (CI passed, awaiting CR re-review) |
| **SLO-207** | SLP207 | Transaction missing explicit rollback — BEGIN without ROLLBACK error path | Medium | Low-Medium | ✅ Complete — detects BEGIN + error return without ROLLBACK across 5 languages |

**Validation:** Verify Sentry-only findings drop by ~30% within 2 weeks of deploying SLO-202 + SLO-203.

---

## Phase 2 — Strengthen Existing CR Parity Rules (P2, Week 2–3)

Rules SLP081–100 were added for CR parity but underperform.

| Task ID | Rule | Current Overlap | Enhancement | Expected Δ |
|---------|------|----------------|-------------|------------|
| **SLO-098-expand** | SLP098 | 3 | Detect new express/next route + missing test file | +3–5 overlap | ✅ Complete — added Fastify, FastAPI, Gin, Echo, Fiber, Django, tRPC patterns + file-based route detection (10 new tests) |
| **SLO-099-expand** | SLP099 | 5 | Track response field rename/removal + test not updated | +2–4 overlap |
| **SLO-100-broaden** | SLP100 | 0 | Add patterns: `return nil`, `return ""`, stubs with TODO comment | +2–6 overlap |

Also: **broaden SLP010** (test no assertion: 9→~30 overlap) — detect test functions that call unimplemented code.

---

## Phase 3 — Add New CR-Flag Patterns (P3, Week 3–4)

Patterns CR commonly flags that slopgate doesn't yet have.

| Task ID | Rule | Pattern | Rationale |
|---------|------|---------|-----------|
| **SLO-151** | SLP151 | Orphaned test — test file references non-existent function/property in target module | High overlap potential, low implementation cost |
| **SLO-152** | SLP152 | Dead code after partial return — detect dead branches within conditionals (SLP019 extension) | CR flags dead code heavily |
| **SLO-153** | SLP153 | Test asserts wrong value — `expect(actual).toBe(wrong_expected)` (tricky, may need diff context) | CR semantic review strength |
| **SLO-154** | SLP154 | Mock over-specification — mock returns fields real API doesn't provide | Prevents brittle tests |

---

## Phase 4 — Tune Low-Overlap High-Volume Rules (P4, Week 4)

Rules that catch lots but have minimal CR overlap. Narrow scope to high-risk contexts to ↑ signal.

| Task ID | Rule | Current | Action |
|---------|------|---------|--------|
| **SLO-017-tune** | SLP017 | 126 ov / 2,712 un (4%) | Scope to public APIs/config values only — matches CR's semantic naming concerns |
| **SLO-148-tune** | SLP148 | 124 ov / 2,014 un (6%) | Scope to module boundaries/exported symbols |
| **SLO-035-narrow** | SLP035 | 152 ov / 1,262 un (11%) | Make severity text-output more CR-like; consider breaking into specific sub-rules |
| **SLO-070-deprio** | SLP070 | 1 ov / 688 un (<1%) | Downgrade severity — CR doesn't care about directory count |

---

## Phase 5 — Preserve Unique Coverage (Do NOT Touch)

These slopgate-only rules are its competitive advantage.

| Rule | Unique/Total | Why Keep |
|------|-------------|----------|
| SLP010 | 706 unique | Test effectiveness — slopgate's differentiator |
| SLP011 | 0 unique but critical | Test structure validation |
| SLP049 | 35 unique | Vacuous test detection |
| SLP056 | 177 unique / 0 overlap | Hardcoded secrets — CR misses |
| SLP064 | 26 unique | Mock without assertion |
| SLP091 | 418 unique | Expiring test dates |
| SLP098–100 | Low overlap but high-value | Test completeness |
| SLP113 | 625 unique / 0 overlap | Test file mismatch — slopgate's killer feature |
| SLP118 | 540 unique / 6 overlap | Array guard — runtime crash prevention |
| SLP127 | 17 unique | Rule-dev test coverage |

---

## Implementation Tracking

**GitHub Project / Tracking:** Create issue labels `area:slopgate`, `type:rule-expansion`, `priority:P1-P3`.

**Benchmark verification:** After each rule change:
1. Run full benchmark suite across all repos
2. Compare `overlap_all` and `coverage_all_pct` deltas
3. Alert if slopgate-only findings drop >10% (means losing unique coverage)

**Success criteria:**
- **~30 days:** Avg overlap ↑ 23% → 30%
- **~60 days:** Avg overlap ↑ 23% → 35%
- **~90 days:** Avg overlap ↑ 23% → 40% (ceiling due to semantic gap)

**Non-goals:**
- Do NOT add semantic analysis (null-flow, race detection, contract drift) — let CR own
- Do NOT sacrifice slopgate-only signal for overlap
- Do NOT increase scan time beyond 100ms

---

## Quick Reference — Task IDs

| ID | Phase | Rule | Status |
|----|-------|------|--------|
| SLO-202 | P1 | Null-deref guard | ✅ Complete (committed + inline guard fix) |
| SLO-203 | P1 | DB constraint violation | ✅ Complete (committed) |
| SLO-204 | P1 | Silent promise mask | ✅ Complete (awaiting CR re-review of PR #26) |
| SLO-207 | P1 | Transaction rollback | ✅ Complete (SLP207 implemented + tests passing) |
| SLO-058-tune | P1 | SQL concat regexp scoping fix | ✅ Complete (committed) |
| SLO-098-expand | P2 | Route w/o test (broadened) | 🚧 Not started |
| SLO-099-expand | P2 | Response field changed test | 🚧 Not started |
| SLO-100-broaden | P2 | Stub returns (broadened) | 🚧 Not started |
| SLO-151 | P3 | Orphaned test | 🚧 Not started |
| SLO-152 | P3 | Dead code after partial return | 🚧 Not started |
| SLO-017-tune | P4 | Magic number scope narrow | 🚧 Not started |
| SLO-148-tune | P4 | Inconsistent naming scope narrow | 🚧 Not started |
| SLO-035-narrow | P4 | General quality specificity | 🚧 Not started |
| SLO-070-deprio | P4 | Too-many-dirs severity downgrade | 🚧 Not started |

**Next session pick-up:** Follow up on CR re-review of **SLO-204** (PR #26 — code complete, CI passed, awaiting CR re-analysis). SLO-207 (transaction rollback) is now complete. SLO-202, SLO-203, and SLO-058 tuning are complete. Next: start **SLO-098-expand** (P2: route w/o test broadened).
