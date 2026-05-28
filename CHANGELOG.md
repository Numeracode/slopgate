# Changelog

## v0.0.23 (2026-05-28)

Precision: stop false-positiving error-handler logging and JSX prose as SQL.

- **SLP014** (debug print) now skips `console.log/debug/trace`, `fmt.Println`, `print()`, etc. inside catch/except handlers. The catch-block detector walks back through the hunk counting braces (JS/TS/Go/Java/Rust) or compares indentation (Python). Real false positive observed on whimsy `useConnections.ts` where `console.debug('VPS connections fetch failed (non-fatal):', err)` was block-flagged inside a `} catch (err) {`. Regression-guarded: debug-prints outside a catch still fire, and Go's `if err != nil` is correctly NOT treated as a catch.
- **SLP058** (SQL injection) split its case-insensitive regex into two passes. The default pattern now requires uppercase SQL keywords, matching the convention real SQL queries follow across the whimsy/codero corpora. Lowercase keywords still fire when the line or hunk shows a recognizable SQL execution context (`pool.query`, `cursor.execute`, `db.raw`, `client.prepare`, `knex`, `sqlx`, etc.). Real false positive observed on whimsy `BackupsPage.tsx:415` — an EmptyState `description` prop containing English prose with a lowercase preposition and a string concat operator was block-flagged as SQL because both tokens were on the same line. Existing pass-cases all preserved; 4 new precision tests cover JSX prose, plain-TS prose, lowercase SQL with `pool.query`, and the uppercase-in-`.ts` regression guard.

## v0.0.22 (2026-05-27)

Noise tuning wave plus 2 new precision rules:

- Tuned **SLP017** (magic numbers) to ignore innocuous numbers `0–10` (previously ignored only `0–2`) in business domain logic to reduce false-positive noise.
- **SLP157**: `parseInt()` on request payload variables without float validation (flags float truncation bugs). Includes robust hunk-wide validation detection.
- **SLP158**: Unsafe theme mutations inside React `useEffect` hooks that can cause a Flash of Unstyled Content (FOUC), recommending `useLayoutEffect`. Hardened with disk-backed file-wide `useEffect` scanner.

Total: 157 rules

## v0.0.21 (2026-05-25)

13 new rules and benchmark improvements:

- **SLP068 / SLP113**: Quarantined to `info` severity to significantly reduce noise in benchmarks and scorecard evaluations.
- **SLP126**: Made the migration foreign key check table-scoped to fix false negatives when multiple tables are modified in the same migration file.
- **SLP143**: Environment variable accessed without validation or default value.
- **SLP144**: Inconsistent error handling patterns in same file or route group.
- **SLP145**: Hardcoded timeout value lacks contextual justification.
- **SLP146**: Unawaited promise in loop or array iteration.
- **SLP147**: Object destructuring from potentially undefined source without guard.
- **SLP148**: Inconsistent naming for the same conceptual variable across modules. Tuned to scope to exported/module-boundary declarations.
- **SLP151**: Orphaned test — flags a test that calls a function removed from a non-test source file.
- **SLP152**: Unreachable code after an if/else chain whose every branch ends with a terminator.
- **SLP155**: New rule checking for `ALTER TABLE ... ADD COLUMN ... NOT NULL` without a `DEFAULT` value.
- **SLP156**: New rule catching redundant JS/TS double-guards checking both `=== null` and `=== undefined`.
- **SLP208**: New rule catching TypeScript/JavaScript default parameters placed before required parameters.
- **SLP209**: New rule catching async arrow functions that return on some paths but not all (missing return at end of body).
- **Benchmark**: Added Gemini Code Assist, DeepSource, Qodo, and universal all-reviewers streams in benchmark_review.py.
- **Benchmark**: Single API call fetches all PR review comments; bot-specific streams derived locally.
- **Repo cleanup**: Removed committed binaries, internal paths, and stale artifacts for public release.

Total: 155 rules

## v0.0.18 (2026-05-02)

Noise reduction wave plus 2 new parity rules:

- Tuned **SLP003** to allow intentional `// ignore` or `// intentional` comments in empty catch/if-err blocks.
- Improved **SLP007** skip logic for complex JS/TS/Python import/export patterns during full-file usage scans.
- Expanded **SLP017** whitelist for common numeric constants (10, 20, 100, 1000, etc.) and added heuristics for "innocuous" function contexts like `setTimeout`.
- Hardened **SLP059** to detect unsanitized `exec.Command` arguments assembled via `strings.Join`.
- **SLP141**: Missing in-flight request guard or `AbortController` in React `useEffect` hooks calling async functions.
- **SLP142**: Unsafe path resolution — `filepath.Join` used in file operations without subsequent `EvalSymlinks` containment checks.

Total: 142 rules

## v0.0.17 (2026-05-01)

Benchmark v2 plus 4 new parity rules:

- Benchmarking now uses isolated worktrees, benchmarks open PRs against real PR heads, and reports legacy/all-comments plus actionable/Sentry-aware overlap.
- **SLP137**: Bot queue uses mixed explicit/default BullMQ priority across sibling call sites
- **SLP138**: Provider call forwards token auth but drops available creds/credentials context
- **SLP139**: S3 hardening helper added but sibling call sites still parse raw credential blobs
- **SLP140**: Credential hardening helper is called on generic token input without JSON/provider guard

Total: 136 rules

## v0.0.16 (2026-05-01)

Noise tuning plus one new Sentry-aligned parity rule:

- Tuned **SLP007** to reuse current-file context when available and to ignore TypeScript `type` import modifiers.
- Tuned **SLP017** to stop flagging descriptive size/duration/validation literals already better covered by specialized rules.
- Tuned **SLP019** to ignore multiline `return` / `throw` / cleanup-callback expressions instead of mislabeling them as unreachable code.
- Tuned **SLP068** to skip test files and collapse overlapping duplicate-window spam into one finding.
- **SLP136**: Caught error wrapped in `AppError` without preserving the original cause

Total: 132 rules
## v0.0.15 (2026-04-30)

Noise tuning plus 8 new mechanical CodeRabbit parity rules:

- Tuned **SLP017**, **SLP035**, **SLP068**, and **SLP117** to avoid broad docs/config/string false positives.
- **SLP128**: Interactive bot queue job uses positive BullMQ priority
- **SLP129**: Tracked `.env` file contains live-looking secret or service binding
- **SLP130**: Hardcoded external origin in browser navigation
- **SLP131**: Nested Link/anchor elements create invalid interactive markup
- **SLP132**: Global keyboard shortcut does not ignore editable controls
- **SLP133**: Express router attaches body parser inline; verify it is not duplicated at app mount
- **SLP134**: Runtime metadata persists full transfer arrays instead of bounded summaries
- **SLP135**: Raw `err.message` persisted into user-visible summary or audit metadata

Total: 131 rules

## v0.0.14 (2026-04-28)

7 new AI-slop detection rules:

- **SLP121**: Sensitive access mutation may be missing tenant/membership authorization guard
- **SLP122**: Async polling/retry logic added without cancellation or in-flight guard
- **SLP123**: Offset pagination on mutable ordering may drift — prefer cursor/keyset or stable tiebreaker
- **SLP124**: External call uses request/input payload without nearby validation guard
- **SLP125**: Share/role/access mutation without nearby audit logging call
- **SLP126**: Migration adds *_id reference without index — add CREATE INDEX for join/cascade performance
- **SLP127**: slopgate rule implementation changed without corresponding test diff update

Total: 123 rules

## v0.0.13 (2026-04-27)

8 new AI-slop detection rules:

- **SLP113**: Source file changed without test update
- **SLP114**: Error-returning function called as statement — check the error return
- **SLP115**: Narrow extension check — add broader extension coverage (e.g., `.js` without `.mjs`/`.cjs`)
- **SLP116**: Regex nested quantifiers — potential ReDoS vulnerability
- **SLP117**: Unanchored regex — add `^`/`$`/`\b` anchor to prevent substring matches
- **SLP118**: Numeric index access without length guard — panic risk on empty collection
- **SLP119**: TrimSuffix/TrimPrefix result used without checking if suffix/prefix was present
- **SLP120**: Value discarded with `_ = expr` — consider using the value

Total: 116 rules

## v0.0.12 (2026-04-26)

21 new rules:

- **SLP091-SLP093**: Test/mock/fixture quality (hardcoded dates, mock envelope mismatches, mock without assertion)
- **SLP094-SLP096**: Silent failure detection (shell `|| true`, empty catch handlers, missing `set -e`)
- **SLP097-SLP099**: API contract/route testing (destructuring vs envelope, route without test, field change without test update)
- **SLP100-SLP102**: Stub/incomplete implementation (no-op functions, dead feature flags, async without await)
- **SLP103-SLP104**: Magic literal expansion (timeout durations, buffer sizes)
- **SLP106-SLP108**: Resource management (acquire without release, cleanup only in error path, open without defer/timeout)
- **SLP109-SLP110**: Code duplication (similar function bodies, similar files)
- **SLP111-SLP112**: Binary/asset hygiene (binary without .gitignore, generated files without source)

Total: 108 rules

## v0.0.11 (2026-04-25)

- CI integration: slopgate runs in GitHub Actions with `--base origin/main`
- Full git history fetched (`fetch-depth: 0`) for diff comparison

## v0.0.10 (2026-04-24)

55 new rules:

- **SLP036-SLP045**: Semantic rules for CodeRabbit parity (query methods, transaction scoping, webhook patterns)
- **SLP046-SLP070**: 25 AI-slop detection rules (file cohesion, redundant comments, SQL injection, hardcoded secrets, dynamic code execution, concurrent maps, resource leaks)
- **SLP071-SLP080**: 10 AST-aware semantic rules (type assertions, nil pointers, defer close, goroutine races, weak crypto, SQL injection, hardcoded credentials, closed channels, ignored errors, single-impl interfaces)
- **SLP081-SLP090**: 10 CodeRabbit parity rules (React imports, keys, hooks, SQL concat, auth checks, webhook timeouts, hardcoded credentials, missing docstrings, API error handling)

Total: 87 rules

## v0.0.9 (2026-04-23)

- **SLP031-035**: React/TSX issues, missing imports, state anti-patterns, code quality
- **SLP036-SLP045**: Semantic rules for API docs, transaction scoping, webhook patterns, query methods
- **SLP046-SLP070**: 25 AI-slop detection rules

## v0.0.7 (2026-04-21)

- **SLP030**: ORM/query methods without sentinel exclusion

## v0.0.1 (2026-04-19)

Initial release with core rules SLP001-SLP029.
