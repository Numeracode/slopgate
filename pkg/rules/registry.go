package rules

// Default returns a Registry pre-populated with every rule shipped in
// the current slopgate version. The CLI calls this; tests that need
// custom rule sets should use NewRegistry directly.
func Default() *Registry {
	r := NewRegistry()
	r.Register(SLP001{})
	r.Register(SLP002{})
	r.Register(SLP003{})
	r.Register(SLP005{})
	r.Register(SLP006{})
	r.Register(SLP007{})
	r.Register(SLP008{})
	r.Register(SLP009{})
	r.Register(SLP010{})
	r.Register(SLP011{})
	r.Register(SLP012{})
	r.Register(SLP013{})
	r.Register(SLP014{})
	r.Register(SLP015{})
	r.Register(SLP016{})
	r.Register(SLP017{})
	r.Register(SLP018{})
	r.Register(SLP019{})
	r.Register(SLP020{})
	r.Register(SLP021{})
	r.Register(SLP022{})
	r.Register(SLP023{})
	r.Register(SLP024{})
	r.Register(SLP025{})
	r.Register(SLP026{})
	r.Register(SLP027{})
	r.Register(SLP030{})
	r.Register(SLP031{})
	r.Register(SLP032{})
	r.Register(SLP033{})
	r.Register(SLP034{})
	r.Register(SLP035{})
	r.Register(SLP036{})
	r.Register(SLP037{})
	r.Register(SLP038{})
	r.Register(SLP039{})
	r.Register(SLP040{})
	r.Register(SLP041{})
	r.Register(SLP042{})
	r.Register(SLP045{})
	r.Register(SLP046{})
	r.Register(SLP047{})
	r.Register(SLP048{})
	r.Register(SLP049{})
	r.Register(SLP051{})
	r.Register(SLP052{})
	r.Register(SLP053{})
	r.Register(SLP054{})
	r.Register(SLP056{})
	r.Register(SLP057{})
	r.Register(SLP058{})
	r.Register(SLP059{})
	r.Register(SLP060{})
	r.Register(SLP061{})
	r.Register(SLP062{})
	r.Register(SLP063{})
	r.Register(SLP064{})
	r.Register(SLP066{})
	r.Register(SLP067{})
	r.Register(SLP068{})
	r.Register(SLP069{})
	r.Register(SLP070{})
	// Register semantic (AST-aware) rules.
	r.RegisterSemantic(SLP071{})
	r.RegisterSemantic(SLP072{})
	r.RegisterSemantic(SLP073{})
	r.RegisterSemantic(SLP074{})
	r.RegisterSemantic(SLP075{})
	r.RegisterSemantic(SLP076{})
	r.RegisterSemantic(SLP077{})
	r.RegisterSemantic(SLP078{})
	r.RegisterSemantic(SLP079{})
	r.RegisterSemantic(SLP080{})
	// Register diff-based rules SLP081-SLP090 for CodeRabbit parity.
	r.Register(SLP081{})
	r.Register(SLP082{})
	r.Register(SLP083{})
	r.Register(SLP084{})
	r.Register(SLP085{})
	r.Register(SLP086{})
	r.Register(SLP087{})
	r.Register(SLP088{})
	r.Register(SLP089{})
	r.Register(SLP090{})
	// v0.0.12: 21 new rules for ~80% CR parity.
	r.Register(SLP091{})
	r.Register(SLP092{})
	r.Register(SLP093{})
	r.Register(SLP094{})
	r.Register(SLP095{})
	r.Register(SLP096{})
	r.Register(SLP097{})
	r.Register(SLP098{})
	r.Register(SLP099{})
	r.Register(SLP100{})
	r.Register(SLP101{})
	r.Register(SLP102{})
	r.Register(SLP103{})
	r.Register(SLP104{})
	r.Register(SLP106{})
	r.Register(SLP107{})
	r.Register(SLP108{})
	r.Register(SLP109{})
	r.Register(SLP110{})
	r.Register(SLP111{})
	r.Register(SLP112{})
	r.Register(SLP113{})
	r.Register(SLP115{})
	r.Register(SLP116{})
	r.Register(SLP117{})
	r.Register(SLP118{})
	r.Register(SLP119{})
	r.Register(SLP121{})
	r.Register(SLP122{})
	r.Register(SLP123{})
	r.Register(SLP124{})
	r.Register(SLP125{})
	r.Register(SLP126{})
	r.Register(SLP127{})
	r.Register(SLP128{})
	r.Register(SLP129{})
	r.Register(SLP130{})
	r.Register(SLP131{})
	r.Register(SLP132{})
	r.Register(SLP133{})
	r.Register(SLP134{})
	r.Register(SLP135{})
	r.Register(SLP136{})
	r.Register(SLP137{})
	r.Register(SLP138{})
	r.Register(SLP139{})
	r.Register(SLP140{})
	r.Register(SLP141{})
	r.Register(SLP142{})
	// New rules: enhanced bug detection coverage
	r.Register(SLP143{})
	r.Register(SLP144{})
	r.Register(SLP145{})
	r.Register(SLP146{})
	r.Register(SLP147{})
	r.Register(SLP148{})
	// P3: Add new CR-flag patterns.
	r.Register(SLP151{}) // orphaned test detection
	r.Register(SLP152{}) // unreachable code after a terminating if/else
	// Precision improvements: migration safety + JS null-guard noise.
	r.Register(SLP155{}) // ADD COLUMN NOT NULL without DEFAULT
	r.Register(SLP156{}) // redundant null+undefined double-guard
	r.Register(SLP157{}) // parseInt float truncation checker
	r.Register(SLP158{}) // useEffect FOUC theme mutation checker
	r.Register(SLP159{}) // subprocess call in test file without timeout
	// P1: Close Sentry bug gap — null dereference guard.
	r.Register(SLP202{})
	// P1: Close Sentry bug gap — DB constraint violation.
	r.Register(SLP203{})
	// P1: Close Sentry bug gap — silent promise failure mask.
	r.Register(SLP204{})
	// P1: Close Sentry bug gap — OpenAPI path merge-order override.
	r.Register(SLP205{})
	// P1: Close Sentry bug gap — transaction missing explicit rollback.
	r.Register(SLP207{})
	// Close Gemini/DeepSource review gaps — JS/TS correctness.
	r.Register(SLP208{}) // default parameter before required parameter
	r.Register(SLP209{}) // async arrow missing return on some paths
	// Precision split: SLP035 decomposed into focused sub-rules.
	r.Register(SLP160{}) // TODO/FIXME comment without ticket reference
	r.Register(SLP161{}) // trailing whitespace
	r.Register(SLP162{}) // long lines (>180 chars)
	// Reviewer gap closure: new rules targeting patterns reviewers flag but slopgate missed.
	r.Register(SLP210{}) // conflicting Tailwind utilities
	r.Register(SLP211{}) // setState clears data before async operation
	r.Register(SLP212{}) // boolean state guard before async (double-submit risk)
	r.Register(SLP213{}) // regex * quantifier may match empty string
	r.Register(SLP214{}) // React Query data access without error check
	// Reviewer gap closure v2: high-impact patterns from whimsy PR benchmarks.
	r.Register(SLP215{}) // API handler changed without updating OpenAPI contract
	r.Register(SLP216{}) // error logging uses err.message instead of full error object
	r.Register(SLP217{}) // path-like parameter not validated for empty input
	r.Register(SLP218{}) // ContentLength gate without Transfer-Encoding handling
	r.Register(SLP219{}) // concurrent access to shared state field without lock
	r.Register(SLP220{}) // filepath.Walk without context cancellation check
	r.Register(SLP221{}) // exec.Command without capturing stderr on failure
	r.Register(SLP222{}) // treating UTF-16/BOM data as UTF-8 without decoding
	r.Register(SLP223{}) // ignored error return
	r.Register(SLP224{}) // HTTP handler missing request body/param validation
	r.Register(SLP225{}) // goroutine mutating shared state without sync
	r.Register(SLP226{}) // SQL resource/transaction imbalance
	r.Register(SLP227{}) // repeated string literal
	return r
}
