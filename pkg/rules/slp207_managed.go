package rules

import (
	"path"
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP207 managed-transaction recognition (JavaScript/TypeScript, Drizzle).
//
// Drizzle's `db.transaction(async (tx) => { ... })` is callback-managed: the
// driver issues BEGIN, awaits the callback, COMMITs on success and issues
// ROLLBACK and rethrows when the callback rejects (drizzle-orm 0.45.2,
// node-postgres/session.js NodePgSession.transaction; nested calls on a
// transaction use SAVEPOINT / ROLLBACK TO SAVEPOINT). The application never
// writes `rollback()`, so the line-oriented SLP207 heuristic reports every such
// call as "transaction started without explicit rollback".
//
// This recognizer proves, for one begin line at a time, that the begin is such
// a managed call. Every step is bound to the exact source snapshot the scanner
// is analysing (the same snapshot SLP007 reads) and every unproven step fails
// closed, leaving the original SLP207 finding in place:
//
//   - the added diff line must equal the snapshot line and contain exactly one
//     JS/TS transaction begin and no other begin form;
//   - the call must be `await|return R.transaction(async (tx) => { ... })` with
//     at most one data-only options object, and its result must not be chained;
//   - the callback must not intercept failures (try/catch/finally, .catch,
//     .then, .finally), issue transaction-control SQL itself, start a nested
//     transaction, use the outer receiver, or let `tx` escape: every use of
//     `tx` is an awaited/returned query chain or a direct argument of an
//     awaited call, and never inside a nested function;
//   - the receiver R must have exactly one unmutated binding whose value is
//     proven, through a closed set of provenance forms, to be a Drizzle
//     database (or a verified forwarding proxy whose substitutes are typed as
//     Drizzle executors, or an inert stub that cannot open a transaction).
//
// Anything outside those forms (other ORMs, aliases through tsconfig paths,
// barrels, unknown factories, non-forwarding proxies, shadowed or overridden
// receivers) is not recognized.

const slp207MaxResolveDepth = 8

// slp207ManagedDrizzleAdapters are Drizzle adapter entry points whose
// `transaction` implementation was verified to be callback-managed.
var slp207ManagedDrizzleAdapters = map[string]bool{
	"drizzle-orm/node-postgres": true,
}

var slp207AsyncLocalStorageModules = map[string]bool{
	"node:async_hooks": true,
	"async_hooks":      true,
}

// String or template literals in a managed callback that begin with
// transaction-control SQL mean the callback manages the transaction itself.
var slp207TxControlLiteral = regexp.MustCompile(`(?i)^\s*(begin|commit|rollback|savepoint|release|end|abort|start\s+transaction)\b`)

// Identifiers that are never callees or value primaries in the positions the
// recognizer inspects.
var slp207Keywords = map[string]bool{
	"if": true, "else": true, "for": true, "while": true, "do": true, "switch": true,
	"case": true, "default": true, "break": true, "continue": true, "return": true,
	"throw": true, "try": true, "catch": true, "finally": true, "function": true,
	"class": true, "const": true, "let": true, "var": true, "new": true, "delete": true,
	"typeof": true, "instanceof": true, "in": true, "void": true, "yield": true,
	"await": true, "import": true, "export": true, "extends": true, "with": true,
	"debugger": true, "enum": true,
}

func slp207IsKeyword(s string) bool { return slp207Keywords[s] }

type slp207Kind int

const (
	slp207Unknown slp207Kind = iota
	slp207Inert              // every call through the value throws; it cannot open a transaction
	slp207Managed            // Drizzle callback-managed transaction API
)

func slp207Combine(a, b slp207Kind) slp207Kind {
	if a == slp207Unknown || b == slp207Unknown {
		return slp207Unknown
	}
	if a == slp207Managed || b == slp207Managed {
		return slp207Managed
	}
	return slp207Inert
}

type slp207Import struct {
	local    string
	imported string // "default", "*", or the exported name
	source   string // module specifier; "" for non-ESM import forms
	typeOnly bool
	idx      int // token index of the local binding
}

// slp207Recognizer caches lexed snapshot files and resolution results for a
// single Check call.
type slp207Recognizer struct {
	d         *diff.Diff
	srcs      map[string]*slp207Src
	receivers map[string]slp207Kind
	factories map[string]string
	active    map[string]bool
}

func newSLP207Recognizer(d *diff.Diff) *slp207Recognizer {
	return &slp207Recognizer{
		d:         d,
		srcs:      map[string]*slp207Src{},
		receivers: map[string]slp207Kind{},
		factories: map[string]string{},
		active:    map[string]bool{},
	}
}

// slp207RecognizableExt limits recognition to plain JS/TS modules. JSX/TSX and
// declaration files are never recognized.
func slp207RecognizableExt(p string) bool {
	lower := strings.ToLower(p)
	if strings.HasSuffix(lower, ".d.ts") {
		return false
	}
	switch path.Ext(lower) {
	case ".ts", ".js", ".mts", ".cts", ".mjs", ".cjs":
		return true
	}
	return false
}

func (r *slp207Recognizer) load(rel string) *slp207Src {
	if s, ok := r.srcs[rel]; ok {
		return s
	}
	var s *slp207Src
	if slp207RecognizableExt(rel) {
		if content, ok := slp007FileContent(r.d, rel); ok {
			if lexed, ok := slp207Lex(rel, content); ok {
				s = lexed
			}
		}
	}
	r.srcs[rel] = s
	return s
}

// resolveModule resolves a relative module specifier against the importing
// file using TypeScript's file-before-directory order. Only .ts/.js targets are
// accepted; a .tsx, .jsx or declaration-only target fails closed.
func (r *slp207Recognizer) resolveModule(fromPath, spec string) (string, bool) {
	if !strings.HasPrefix(spec, "./") && !strings.HasPrefix(spec, "../") {
		return "", false
	}
	joined, ok := slp007CleanRelativePath(path.Join(path.Dir(fromPath), spec))
	if !ok {
		return "", false
	}
	var candidates []string
	switch ext := path.Ext(joined); ext {
	case ".ts", ".mts", ".cts":
		candidates = []string{joined}
	case ".js", ".mjs", ".cjs":
		// ESM-style TypeScript specifiers name the emitted .js file.
		candidates = []string{strings.TrimSuffix(joined, ext) + map[string]string{".js": ".ts", ".mjs": ".mts", ".cjs": ".cts"}[ext], joined}
	default:
		candidates = []string{
			joined + ".ts", joined + ".tsx", joined + ".d.ts", joined + ".js", joined + ".jsx",
			joined + "/index.ts", joined + "/index.tsx", joined + "/index.d.ts", joined + "/index.js", joined + "/index.jsx",
		}
	}
	for _, c := range candidates {
		if _, ok := slp007FileContent(r.d, c); ok {
			if !slp207RecognizableExt(c) {
				return "", false
			}
			return c, true
		}
	}
	return "", false
}

// slp207SingleJSBegin reports whether the trimmed added line contains exactly
// one JS/TS transaction begin and no other SLP207 begin form.
func slp207SingleJSBegin(content string) bool {
	for _, pat := range txBeginPatterns {
		if pat == slp207JSTransactionBegin {
			if len(pat.FindAllStringIndex(content, -1)) != 1 {
				return false
			}
			continue
		}
		if pat.MatchString(content) {
			return false
		}
	}
	return true
}

// managedSpan returns the 1-based source line span of a proven callback-managed
// Drizzle transaction that begins on the given added line.
func (r *slp207Recognizer) managedSpan(filePath string, bl addedLine) (int, int, bool) {
	if r == nil || r.d == nil || r.d.RepoRoot == "" || !slp207RecognizableExt(filePath) {
		return 0, 0, false
	}
	if !slp207SingleJSBegin(bl.content) {
		return 0, 0, false
	}
	s := r.load(filePath)
	if s == nil || bl.newLineNo < 1 || bl.newLineNo > len(s.lines) {
		return 0, 0, false
	}
	if strings.TrimSpace(s.lines[bl.newLineNo-1]) != bl.content {
		return 0, 0, false
	}
	t := -1
	for k := range s.toks {
		if s.toks[k].line != bl.newLineNo {
			continue
		}
		if s.isIdent(k, "transaction") && s.afterMemberDot(k) && s.isPunct(k+1, "(") {
			if t >= 0 {
				return 0, 0, false
			}
			t = k
		}
	}
	if t < 2 || !s.isPunct(t-1, ".") {
		return 0, 0, false
	}
	recv := t - 2
	if !s.isAnyIdent(recv) || slp207IsKeyword(s.toks[recv].text) || s.afterMemberDot(recv) {
		return 0, 0, false
	}
	if !s.isIdent(recv-1, "await") && !s.isIdent(recv-1, "return") {
		return 0, 0, false
	}
	open := t + 1
	closeIdx := s.match[open]
	if next := closeIdx + 1; next < len(s.toks) {
		if !(s.isPunct(next, ";") || s.isPunct(next, ")") || s.isPunct(next, "]") || s.isPunct(next, "}") || s.isPunct(next, ",")) {
			return 0, 0, false
		}
	}
	args, ok := s.splitTopLevel(open+1, closeIdx-1)
	if !ok || len(args) < 1 || len(args) > 2 {
		return 0, 0, false
	}
	if len(args) == 2 && !s.isDataObjectLiteral(args[1][0], args[1][1]) {
		return 0, 0, false
	}
	param, bodyOpen, ok := s.asyncCallback(args[0][0], args[0][1])
	if !ok {
		return 0, 0, false
	}
	recvName := s.toks[recv].text
	if !r.callbackIsContained(s, recvName, param, bodyOpen, s.match[bodyOpen]) {
		return 0, 0, false
	}
	if r.receiverKind(s, recvName, 0) != slp207Managed {
		return 0, 0, false
	}
	return s.toks[recv].line, s.toks[closeIdx].line, true
}

// ---------------------------------------------------------------------------
// Callback shape and containment
// ---------------------------------------------------------------------------

// asyncCallback matches `async (p[: T]) => { ... }`, `async p => { ... }` or
// `async function [name](p[: T]) { ... }` spanning exactly [lo, hi].
func (s *slp207Src) asyncCallback(lo, hi int) (string, int, bool) {
	if !s.isIdent(lo, "async") {
		return "", 0, false
	}
	k := lo + 1
	var param string
	var body int
	switch {
	case s.isIdent(k, "function"):
		k++
		if s.isAnyIdent(k) {
			k++
		}
		if !s.isPunct(k, "(") {
			return "", 0, false
		}
		p, ok := s.singleSimpleParam(k)
		if !ok {
			return "", 0, false
		}
		param, body = p, s.findBodyBrace(s.match[k]+1)
	case s.isAnyIdent(k) && s.isPunct(k+1, "=>"):
		param, body = s.toks[k].text, k+2
	case s.isPunct(k, "("):
		p, ok := s.singleSimpleParam(k)
		if !ok {
			return "", 0, false
		}
		arrow := s.match[k] + 1
		if s.isPunct(arrow, ":") {
			for arrow < hi && !(s.isPunct(arrow, "=>") && s.depth[arrow] == s.depth[k]) {
				if s.isOpener(arrow) {
					arrow = s.match[arrow]
				}
				arrow++
			}
		}
		if !s.isPunct(arrow, "=>") {
			return "", 0, false
		}
		param, body = p, arrow+1
	default:
		return "", 0, false
	}
	if param == "" || slp207IsKeyword(param) || param == "this" || param == "arguments" {
		return "", 0, false
	}
	if body <= 0 || !s.isPunct(body, "{") || s.match[body] != hi {
		return "", 0, false
	}
	return param, body, true
}

// singleSimpleParam accepts exactly one plain identifier parameter with an
// optional type annotation: no defaults, rest, destructuring or extra params.
func (s *slp207Src) singleSimpleParam(open int) (string, bool) {
	c := s.match[open]
	if !s.isAnyIdent(open + 1) {
		return "", false
	}
	if open+2 == c {
		return s.toks[open+1].text, true
	}
	if !s.isPunct(open+2, ":") {
		return "", false
	}
	for k := open + 3; k < c; k++ {
		if s.isOpener(k) {
			k = s.match[k]
			continue
		}
		if s.isPunct(k, ",") || s.isPunct(k, "=") {
			return "", false
		}
	}
	return s.toks[open+1].text, true
}

// isDataObjectLiteral accepts a Drizzle transaction config object containing
// only keys and literal-ish values (no calls, functions or spreads).
func (s *slp207Src) isDataObjectLiteral(lo, hi int) bool {
	if !s.isPunct(lo, "{") || s.match[lo] != hi {
		return false
	}
	for k := lo + 1; k < hi; k++ {
		t := s.toks[k]
		switch t.kind {
		case slp207TokIdent:
			if t.text == "function" || t.text == "async" || t.text == "new" || t.text == "await" {
				return false
			}
		case slp207TokPunct:
			switch t.text {
			case ":", ",", "{", "}", "[", "]", ".":
			default:
				return false
			}
		case slp207TokTemplate, slp207TokRegex:
			return false
		}
	}
	return true
}

// callbackIsContained proves that the managed callback leaves failure handling
// to Drizzle and keeps all work bound to its transaction handle.
func (r *slp207Recognizer) callbackIsContained(s *slp207Src, recvName, param string, bodyOpen, bodyClose int) bool {
	regions := s.nestedFunctionRegions(bodyOpen, bodyClose)
	sameModule := s.namesFromSameModule(recvName)
	for k := bodyOpen + 1; k < bodyClose; k++ {
		t := s.toks[k]
		switch t.kind {
		case slp207TokString, slp207TokTemplate:
			if slp207TxControlLiteral.MatchString(t.text) {
				return false
			}
			continue
		case slp207TokIdent:
		default:
			continue
		}
		switch t.text {
		case "try", "catch", "finally", "class", "yield", "eval", "arguments", "with":
			return false
		}
		if s.afterMemberDot(k) {
			if t.text == "then" || t.text == "transaction" && s.isPunct(k+1, "(") {
				return false
			}
			continue
		}
		if sameModule[t.text] {
			return false
		}
		if t.text != param {
			continue
		}
		if slp207InSpans(k, regions) || !s.handleUseIsAwaited(k) {
			return false
		}
	}
	return true
}

// handleUseIsAwaited accepts `await tx.q()...`, `return tx.q()...;` and a bare
// `tx` passed directly as an argument to an awaited (or returned) call.
func (s *slp207Src) handleUseIsAwaited(k int) bool {
	if s.isPunct(k+1, ".") && s.isAnyIdent(k+2) {
		return s.chainIsAwaited(k)
	}
	if (s.isPunct(k-1, "(") || s.isPunct(k-1, ",")) && (s.isPunct(k+1, ")") || s.isPunct(k+1, ",")) {
		o := s.parent[k]
		if o < 0 || !s.isPunct(o, "(") || !s.isCallParen(o) {
			return false
		}
		start := s.chainStartFromCallee(o)
		return start >= 0 && s.chainIsAwaited(start)
	}
	return false
}

func (s *slp207Src) chainIsAwaited(start int) bool {
	end := s.chainEnd(start)
	if end < 0 {
		return false
	}
	if s.isIdent(start-1, "await") {
		return true
	}
	if s.isIdent(start-1, "return") {
		return s.isPunct(end+1, ";") || s.isPunct(end+1, "}")
	}
	return false
}

// chainEnd returns the last token of the member/call chain whose primary
// expression starts at start.
func (s *slp207Src) chainEnd(start int) int {
	var end int
	switch {
	case s.isAnyIdent(start) && !slp207IsKeyword(s.toks[start].text):
		end = start
	case s.isPunct(start, "(") || s.isPunct(start, "["):
		end = s.match[start]
	default:
		return -1
	}
	for {
		n := end + 1
		switch {
		case s.isAnyIdent(n+1) && (s.isPunct(n, ".") || s.isPunct(n, "?.")):
			end = n + 1
		case s.isPunct(n, "?.") && (s.isPunct(n+1, "(") || s.isPunct(n+1, "[")):
			end = s.match[n+1]
		case s.isPunct(n, "(") || s.isPunct(n, "["):
			end = s.match[n]
		case s.isPunct(n, "!"):
			end = n
		default:
			return end
		}
	}
}

// chainStartFromCallee walks back from a call's '(' to the start of the
// callee chain.
func (s *slp207Src) chainStartFromCallee(open int) int {
	k := open - 1
	if s.isPunct(k, "?.") {
		k--
	}
	for steps := 0; steps < 256; steps++ {
		switch {
		case s.isAnyIdent(k) && !slp207IsKeyword(s.toks[k].text):
			if s.isPunct(k-1, ".") || s.isPunct(k-1, "?.") {
				k -= 2
				continue
			}
			return k
		case s.isPunct(k, ")") || s.isPunct(k, "]"):
			m := s.match[k]
			p := m - 1
			if (s.isAnyIdent(p) && !slp207IsKeyword(s.toks[p].text)) || s.isPunct(p, ")") || s.isPunct(p, "]") || s.isPunct(p, "!") {
				k = p
				continue
			}
			if s.isPunct(p, "?.") {
				k = p - 1
				continue
			}
			return m
		case s.isPunct(k, "!"):
			k--
		default:
			return -1
		}
	}
	return -1
}

// isCallParen reports whether '(' at o is a call's argument list.
func (s *slp207Src) isCallParen(o int) bool {
	if !s.isPunct(o, "(") || s.isParamList(o) {
		return false
	}
	p := o - 1
	switch {
	case s.isAnyIdent(p):
		return !slp207IsKeyword(s.toks[p].text)
	case s.isPunct(p, ")") || s.isPunct(p, "]") || s.isPunct(p, "?.") || s.isPunct(p, "!"):
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Structural helpers
// ---------------------------------------------------------------------------

// splitTopLevel splits [lo, hi] at commas outside nested groups. A trailing
// comma is allowed; an empty element is not.
func (s *slp207Src) splitTopLevel(lo, hi int) ([][2]int, bool) {
	var out [][2]int
	if lo > hi {
		return nil, true
	}
	start := lo
	for k := lo; k <= hi; k++ {
		if s.isOpener(k) {
			k = s.match[k]
			continue
		}
		if s.isPunct(k, ",") {
			if k == start {
				return nil, false
			}
			out = append(out, [2]int{start, k - 1})
			start = k + 1
		}
	}
	if start <= hi {
		out = append(out, [2]int{start, hi})
	}
	return out, true
}

// matchAngle returns the '>' closing the type-argument list opened at lt.
func (s *slp207Src) matchAngle(lt int) int {
	count := 0
	for k := lt; k < len(s.toks); k++ {
		switch {
		case s.isPunct(k, "<"):
			count++
		case s.isPunct(k, ">"):
			count--
			if count == 0 {
				return k
			}
		case s.isPunct(k, ";") || s.isPunct(k, ")") || s.isPunct(k, "]") || s.isPunct(k, "}"):
			return -1
		case s.isOpener(k):
			k = s.match[k]
		}
	}
	return -1
}

// findBodyBrace returns the '{' opening a function body that follows a
// parameter list ending just before j, skipping an optional return type.
func (s *slp207Src) findBodyBrace(j int) int {
	if s.isPunct(j, "{") {
		return j
	}
	if !s.isPunct(j, ":") {
		return -1
	}
	for k := j + 1; k < len(s.toks); k++ {
		switch {
		case s.isPunct(k, "{"):
			p := k - 1
			if s.isPunct(p, ":") || s.isPunct(p, "|") || s.isPunct(p, "&") || s.isPunct(p, "<") ||
				s.isPunct(p, ",") || s.isPunct(p, "=>") || s.isPunct(p, "(") || s.isPunct(p, "[") {
				k = s.match[k]
				continue
			}
			return k
		case s.isPunct(k, "(") || s.isPunct(k, "["):
			k = s.match[k]
		case s.isPunct(k, ";") || s.isPunct(k, "=") || s.isPunct(k, ")") || s.isPunct(k, "]") || s.isPunct(k, "}"):
			return -1
		}
	}
	return -1
}

func (s *slp207Src) isObjectLiteralBrace(b int) bool {
	p := b - 1
	if p < 0 {
		return false
	}
	if s.toks[p].kind == slp207TokPunct {
		switch s.toks[p].text {
		case "=", "(", ",", ":", "?", "[", "||", "&&", "??", "...":
			return true
		}
		return false
	}
	return s.isIdent(p, "return") || s.isIdent(p, "yield") || s.isIdent(p, "await")
}

func (s *slp207Src) isClassBodyBrace(b int) bool {
	for k, n := b-1, 0; k >= 0 && n < 24; k, n = k-1, n+1 {
		if s.isIdent(k, "class") {
			return true
		}
		if !(s.isAnyIdent(k) || s.isPunct(k, ".") || s.isPunct(k, "<") || s.isPunct(k, ">") || s.isPunct(k, ",")) {
			return false
		}
	}
	return false
}

// isParamList reports whether '(' at o opens a function, method, arrow or
// catch parameter list.
func (s *slp207Src) isParamList(o int) bool {
	if !s.isPunct(o, "(") {
		return false
	}
	c := s.match[o]
	if s.isPunct(c+1, "=>") {
		return true
	}
	if s.isPunct(c+1, ":") {
		for k := c + 2; k < len(s.toks); k++ {
			if s.isOpener(k) && !s.isPunct(k, "{") {
				k = s.match[k]
				continue
			}
			if s.isPunct(k, "=>") && s.depth[k] == s.depth[o] {
				return true
			}
			if s.isPunct(k, ";") || s.isPunct(k, ",") || s.isPunct(k, "=") || s.depth[k] < s.depth[o] || s.isPunct(k, "{") {
				break
			}
		}
	}
	p := o - 1
	if s.isPunct(p, ">") { // generic type parameters: f<T>(…)
		for k, count := p, 0; k >= 0; k-- {
			if s.isPunct(k, ">") {
				count++
			} else if s.isPunct(k, "<") {
				count--
				if count == 0 {
					p = k - 1
					break
				}
			} else if s.isPunct(k, ";") || s.isPunct(k, "{") || s.isPunct(k, "}") {
				return false
			}
		}
	}
	if s.isIdent(p, "function") || s.isIdent(p, "catch") || (s.isAnyIdent(p) && s.isIdent(p-1, "function")) ||
		(s.isAnyIdent(p) && s.isPunct(p-1, "*") && s.isIdent(p-2, "function")) {
		return true
	}
	if (s.isAnyIdent(p) && !slp207IsKeyword(s.toks[p].text)) || s.isPunct(p, "]") {
		if s.findBodyBrace(c+1) >= 0 {
			if enc := s.parent[o]; enc >= 0 && s.isPunct(enc, "{") && (s.isObjectLiteralBrace(enc) || s.isClassBodyBrace(enc)) {
				return true
			}
		}
	}
	return false
}

// nestedFunctionRegions returns token ranges of functions defined strictly
// inside (lo, hi): arrow bodies, function expressions/declarations and
// object/class methods. Expression-bodied arrows conservatively extend to the
// end of their enclosing list element or statement.
func (s *slp207Src) nestedFunctionRegions(lo, hi int) [][2]int {
	var out [][2]int
	for k := lo + 1; k < hi; k++ {
		switch {
		case s.isPunct(k, "=>"):
			if s.isPunct(k+1, "{") {
				out = append(out, [2]int{k + 1, s.match[k+1]})
				continue
			}
			j := k + 1
			for j < hi {
				if s.isOpener(j) {
					j = s.match[j] + 1
					continue
				}
				if s.depth[j] < s.depth[k] || (s.depth[j] == s.depth[k] && (s.isPunct(j, ",") || s.isPunct(j, ";"))) {
					break
				}
				j++
			}
			out = append(out, [2]int{k + 1, j - 1})
		case s.isPunct(k, "(") && s.isParamList(k) && !s.isPunct(s.match[k]+1, "=>"):
			if b := s.findBodyBrace(s.match[k] + 1); b >= 0 && b < hi {
				out = append(out, [2]int{k, s.match[b]})
			}
		}
	}
	return out
}

// statements splits the block content (lo, hi) into top-level statements.
func (s *slp207Src) statements(lo, hi int) ([][2]int, bool) {
	var out [][2]int
	for k := lo; k < hi; {
		if s.isPunct(k, ";") {
			k++
			continue
		}
		end, ok := s.statementEnd(k, hi)
		if !ok || end < k {
			return nil, false
		}
		out = append(out, [2]int{k, end})
		k = end + 1
	}
	return out, true
}

func (s *slp207Src) statementEnd(k, hi int) (int, bool) {
	switch {
	case s.isIdent(k, "if"):
		if !s.isPunct(k+1, "(") {
			return 0, false
		}
		e, ok := s.statementEnd(s.match[k+1]+1, hi)
		if ok && s.isIdent(e+1, "else") {
			return s.statementEnd(e+2, hi)
		}
		return e, ok
	case s.isIdent(k, "for") || s.isIdent(k, "while"):
		j := k + 1
		if s.isIdent(j, "await") {
			j++
		}
		if !s.isPunct(j, "(") {
			return 0, false
		}
		return s.statementEnd(s.match[j]+1, hi)
	case s.isPunct(k, "{"):
		return s.match[k], true
	}
	for j := k; j < hi; j++ {
		if s.isOpener(j) {
			j = s.match[j]
			continue
		}
		if s.isPunct(j, ";") {
			return j, true
		}
	}
	return hi - 1, true
}

// ---------------------------------------------------------------------------
// Bindings, imports and mutation
// ---------------------------------------------------------------------------

func (s *slp207Src) parseImports() []slp207Import {
	if s.importsParsed {
		return s.imports
	}
	s.importsParsed = true
	for k := range s.toks {
		if !s.isIdent(k, "import") || s.depth[k] != 0 || s.afterMemberDot(k) || s.isPunct(k+1, "(") || s.isPunct(k+1, ".") {
			continue
		}
		j := k + 1
		declTypeOnly := false
		if s.isIdent(j, "type") && (s.isPunct(j+1, "{") || s.isPunct(j+1, "*") || (s.isAnyIdent(j+1) && !s.isIdent(j+1, "from"))) {
			declTypeOnly = true
			j++
		}
		if s.isString(j) {
			continue
		}
		var specs []slp207Import
		if s.isAnyIdent(j) && !s.isPunct(j, "{") {
			specs = append(specs, slp207Import{local: s.toks[j].text, imported: "default", typeOnly: declTypeOnly, idx: j})
			j++
			if s.isPunct(j, "=") { // TypeScript `import x = require(...)`
				specs[0].imported = ""
				s.imports = append(s.imports, specs...)
				continue
			}
			if s.isPunct(j, ",") {
				j++
			}
		}
		if s.isPunct(j, "*") && s.isIdent(j+1, "as") && s.isAnyIdent(j+2) {
			specs = append(specs, slp207Import{local: s.toks[j+2].text, imported: "*", typeOnly: declTypeOnly, idx: j + 2})
			j += 3
		}
		if s.isPunct(j, "{") {
			c := s.match[j]
			parts, _ := s.splitTopLevel(j+1, c-1)
			for _, pr := range parts {
				a, b := pr[0], pr[1]
				typeOnly := declTypeOnly
				if s.isIdent(a, "type") && a < b {
					typeOnly = true
					a++
				}
				name := s.toks[a].text
				local, idx := name, a
				if a+2 == b && s.isIdent(a+1, "as") {
					local, idx = s.toks[b].text, b
				} else if a != b {
					local, idx = "", -1
				}
				specs = append(specs, slp207Import{local: local, imported: name, typeOnly: typeOnly, idx: idx})
			}
			j = c + 1
		}
		source := ""
		specTok := j + 1
		if s.isIdent(j, "from") && s.isString(specTok) {
			source = s.toks[specTok].text
		}
		for i := range specs {
			specs[i].source = source
		}
		s.imports = append(s.imports, specs...)
	}
	return s.imports
}

// valueImport returns the single value import binding name, if any.
func (s *slp207Src) valueImport(name string) (slp207Import, bool) {
	var found []slp207Import
	for _, imp := range s.parseImports() {
		if imp.local == name {
			found = append(found, imp)
		}
	}
	if len(found) != 1 || found[0].typeOnly || found[0].source == "" {
		return slp207Import{}, false
	}
	return found[0], true
}

// namesFromSameModule returns the receiver plus every other value binding
// imported from the receiver's module.
func (s *slp207Src) namesFromSameModule(recv string) map[string]bool {
	out := map[string]bool{recv: true}
	imp, ok := s.valueImport(recv)
	if !ok {
		return out
	}
	for _, other := range s.parseImports() {
		if other.source == imp.source && !other.typeOnly && other.local != "" {
			out[other.local] = true
		}
	}
	return out
}

// bindingSites returns every token position that binds or assigns name:
// imports, declarations, destructuring, parameters and plain assignment.
func (s *slp207Src) bindingSites(name string) []int {
	var out []int
	for _, imp := range s.parseImports() {
		if imp.local == name {
			out = append(out, imp.idx)
		}
	}
	for k := range s.toks {
		if !s.isIdent(k, name) || s.afterMemberDot(k) || s.isImportToken(k) {
			continue
		}
		p := k - 1
		switch {
		case s.isIdent(p, "const") || s.isIdent(p, "let") || s.isIdent(p, "var") || s.isIdent(p, "function") ||
			s.isIdent(p, "class") || s.isIdent(p, "enum") || s.isIdent(p, "namespace") ||
			(s.isPunct(p, "*") && s.isIdent(p-1, "function")):
			out = append(out, k)
			continue
		case s.isPunct(k+1, "=>"):
			out = append(out, k)
			continue
		case s.isAssignOp(k+1) || s.isPunct(k+1, "++") || s.isPunct(k+1, "--") || s.isPunct(p, "++") || s.isPunct(p, "--"):
			out = append(out, k)
			continue
		}
		for o := s.parent[k]; o >= 0; o = s.parent[o] {
			if s.isPunct(o, "(") {
				if s.isParamList(o) {
					out = append(out, k)
				}
				break
			}
			if (s.isPunct(o, "{") || s.isPunct(o, "[")) &&
				(s.isIdent(o-1, "const") || s.isIdent(o-1, "let") || s.isIdent(o-1, "var")) {
				out = append(out, k)
				break
			}
		}
	}
	return out
}

func (s *slp207Src) isAssignOp(k int) bool {
	if k < 0 || k >= len(s.toks) || s.toks[k].kind != slp207TokPunct {
		return false
	}
	switch s.toks[k].text {
	case "=", "+=", "-=", "*=", "/=", "%=", "**=", "<<=", "&=", "|=", "^=", "&&=", "||=", "??=":
		return true
	}
	return false
}

var slp207MutatorCalls = map[string]bool{
	"Object.defineProperty": true, "Object.defineProperties": true, "Object.assign": true,
	"Object.setPrototypeOf": true, "Reflect.set": true, "Reflect.defineProperty": true,
	"Reflect.deleteProperty": true, "Reflect.setPrototypeOf": true,
}

// mutated reports whether any member of name is assigned, deleted or
// redefined in this file.
func (s *slp207Src) mutated(name string) bool {
	for k := range s.toks {
		if !s.isIdent(k, name) || s.afterMemberDot(k) {
			continue
		}
		end := k
		for {
			if s.isPunct(end+1, ".") && s.isAnyIdent(end+2) {
				end += 2
			} else if s.isPunct(end+1, "[") {
				end = s.match[end+1]
			} else {
				break
			}
		}
		if end > k && (s.isAssignOp(end+1) || s.isPunct(end+1, "++") || s.isPunct(end+1, "--") ||
			s.isPunct(k-1, "++") || s.isPunct(k-1, "--") || s.isIdent(k-1, "delete")) {
			return true
		}
		if (s.isPunct(k-1, "(") || s.isPunct(k-1, ",")) && s.parent[k] >= 3 {
			o := s.parent[k]
			if s.isPunct(o, "(") && s.isAnyIdent(o-1) && s.isPunct(o-2, ".") && s.isAnyIdent(o-3) &&
				slp207MutatorCalls[s.toks[o-3].text+"."+s.toks[o-1].text] {
				return true
			}
		}
	}
	return false
}

// findConst finds the single `const name [: T] = init;` declared at baseDepth
// within [lo, hi) and returns the initializer range and the `const` index.
func (s *slp207Src) findConst(name string, lo, hi, baseDepth int) (int, int, int, bool) {
	found := -1
	for k := lo; k < hi; k++ {
		if s.depth[k] != baseDepth || !s.isIdent(k, "const") || !s.isIdent(k+1, name) {
			continue
		}
		if found >= 0 {
			return 0, 0, 0, false
		}
		found = k
	}
	if found < 0 {
		return 0, 0, 0, false
	}
	j := found + 2
	if s.isPunct(j, ":") {
		for j < hi && !(s.depth[j] == baseDepth && s.isPunct(j, "=")) {
			if s.isOpener(j) {
				j = s.match[j]
			}
			if s.isPunct(j, ";") {
				return 0, 0, 0, false
			}
			j++
		}
	}
	if !s.isPunct(j, "=") {
		return 0, 0, 0, false
	}
	e := j + 1
	for e < hi {
		if s.isOpener(e) {
			e = s.match[e] + 1
			continue
		}
		if s.isPunct(e, ";") {
			break
		}
		if s.isPunct(e, ",") {
			return 0, 0, 0, false
		}
		e++
	}
	if e >= hi || !s.isPunct(e, ";") || e-1 < j+1 {
		return 0, 0, 0, false
	}
	return j + 1, e - 1, found, true
}

// findFunction finds the single top-level, non-async, non-generator
// `[export] function name[<…>](…)[: T] { … }`.
func (s *slp207Src) findFunction(name string) (paramsOpen, bodyOpen int, ok bool) {
	found := -1
	for k := range s.toks {
		if s.depth[k] == 0 && s.isIdent(k, "function") && s.isIdent(k+1, name) {
			if found >= 0 {
				return 0, 0, false
			}
			found = k
		}
	}
	if found < 0 || s.isIdent(found-1, "async") {
		return 0, 0, false
	}
	j := found + 2
	if s.isPunct(j, "<") {
		gt := s.matchAngle(j)
		if gt < 0 {
			return 0, 0, false
		}
		j = gt + 1
	}
	if !s.isPunct(j, "(") {
		return 0, 0, false
	}
	b := s.findBodyBrace(s.match[j] + 1)
	if b < 0 {
		return 0, 0, false
	}
	return j, b, true
}

// stripTypeWrappers removes enclosing parentheses, trailing `as T` /
// `satisfies T` assertions and non-null assertions, none of which change the
// runtime value.
func (s *slp207Src) stripTypeWrappers(lo, hi int) (int, int) {
	for changed := true; changed && lo <= hi; {
		changed = false
		if s.isPunct(lo, "(") && s.match[lo] == hi {
			lo, hi, changed = lo+1, hi-1, true
			continue
		}
		// Only assertions at the expression's top level count; groups
		// (including a leading object literal or call) are skipped whole.
		for k := lo; k <= hi; k++ {
			if s.isOpener(k) {
				k = s.match[k]
				continue
			}
			if k > lo && (s.isIdent(k, "as") || s.isIdent(k, "satisfies")) {
				hi, changed = k-1, true
				break
			}
		}
		if s.isPunct(hi, "!") {
			hi, changed = hi-1, true
		}
	}
	return lo, hi
}

// objectProperty returns the value range of the single non-computed property
// key in the object literal opened at lo. Spreads and computed keys fail.
func (s *slp207Src) objectProperty(lo int, key string) (int, int, bool) {
	parts, ok := s.splitTopLevel(lo+1, s.match[lo]-1)
	if !ok {
		return 0, 0, false
	}
	vlo, vhi, n := 0, 0, 0
	for _, pr := range parts {
		a, b := pr[0], pr[1]
		if s.isPunct(a, "...") || s.isPunct(a, "[") {
			return 0, 0, false
		}
		if !(s.isAnyIdent(a) || s.isString(a)) || s.toks[a].text != key {
			continue
		}
		n++
		switch {
		case a == b && s.isAnyIdent(a):
			vlo, vhi = a, a
		case s.isPunct(a+1, ":") && a+2 <= b:
			vlo, vhi = a+2, b
		default:
			return 0, 0, false
		}
	}
	return vlo, vhi, n == 1
}

// ---------------------------------------------------------------------------
// Receiver provenance
// ---------------------------------------------------------------------------

func (r *slp207Recognizer) receiverKind(s *slp207Src, name string, depth int) slp207Kind {
	key := s.path + "\x00" + name
	if k, ok := r.receivers[key]; ok {
		return k
	}
	if r.active[key] || depth > slp207MaxResolveDepth {
		return slp207Unknown
	}
	r.active[key] = true
	k := r.computeReceiverKind(s, name, depth)
	delete(r.active, key)
	r.receivers[key] = k
	return k
}

func (r *slp207Recognizer) computeReceiverKind(s *slp207Src, name string, depth int) slp207Kind {
	if s.mutated(name) {
		return slp207Unknown
	}
	sites := s.bindingSites(name)
	if len(sites) != 1 {
		return slp207Unknown
	}
	if imp, ok := s.valueImport(name); ok && imp.idx == sites[0] {
		if imp.imported == "default" || imp.imported == "*" || imp.imported == "" {
			return slp207Unknown
		}
		modPath, ok := r.resolveModule(s.path, imp.source)
		if !ok {
			return slp207Unknown
		}
		m := r.load(modPath)
		if m == nil {
			return slp207Unknown
		}
		return r.exportKind(m, imp.imported, depth+1)
	}
	lo, hi, decl, ok := s.findConst(name, 0, len(s.toks), 0)
	if !ok || decl+1 != sites[0] {
		return slp207Unknown
	}
	return r.classifyExpr(s, lo, hi, nil, depth+1)
}

func (r *slp207Recognizer) exportKind(m *slp207Src, name string, depth int) slp207Kind {
	if m.mutated(name) {
		return slp207Unknown
	}
	sites := m.bindingSites(name)
	if len(sites) != 1 {
		return slp207Unknown
	}
	lo, hi, decl, ok := m.findConst(name, 0, len(m.toks), 0)
	if !ok || decl+1 != sites[0] || !m.isIdent(decl-1, "export") {
		return slp207Unknown
	}
	// A second export of the same name (e.g. `export { other as db }`) fails.
	for k := range m.toks {
		if m.depth[k] == 0 && m.isIdent(k, "export") && m.isPunct(k+1, "{") {
			for j := k + 2; j < m.match[k+1]; j++ {
				if m.isIdent(j, name) {
					return slp207Unknown
				}
			}
		}
	}
	return r.classifyExpr(m, lo, hi, nil, depth)
}

// classifyExpr classifies the runtime value of the expression [lo, hi] in s.
// scope, when set, is the `{` index of the enclosing function body used for
// resolving function-local constants.
func (r *slp207Recognizer) classifyExpr(s *slp207Src, lo, hi int, scope *int, depth int) slp207Kind {
	if depth > slp207MaxResolveDepth || lo > hi {
		return slp207Unknown
	}
	lo, hi = s.stripTypeWrappers(lo, hi)
	switch {
	case lo == hi && s.isAnyIdent(lo):
		return r.classifyIdent(s, s.toks[lo].text, scope, depth)
	case s.isIdent(lo, "new") && s.isIdent(lo+1, "Proxy") && s.isPunct(lo+2, "(") && s.match[lo+2] == hi:
		if len(s.bindingSites("Proxy")) != 0 {
			return slp207Unknown
		}
		return r.classifyInertProxy(s, lo+2)
	case s.isAnyIdent(lo) && s.isPunct(lo+1, "(") && s.match[lo+1] == hi:
		name := s.toks[lo].text
		imp, ok := s.valueImport(name)
		if ok && imp.imported == "drizzle" && slp207ManagedDrizzleAdapters[imp.source] &&
			len(s.bindingSites(name)) == 1 && !s.mutated(name) {
			return slp207Managed
		}
		return slp207Unknown
	case s.isAnyIdent(lo) && s.isPunct(lo+1, ".") && s.isAnyIdent(lo+2) && lo+2 == hi:
		return r.classifyMember(s, s.toks[lo].text, s.toks[lo+2].text, scope, depth)
	}
	return slp207Unknown
}

// lookupConst resolves a constant in the function scope, then at top level.
func (s *slp207Src) lookupConst(name string, scope *int) (int, int, *int, bool) {
	if scope != nil {
		b := *scope
		if lo, hi, _, ok := s.findConst(name, b+1, s.match[b], s.depth[b]+1); ok {
			return lo, hi, scope, true
		}
	}
	if lo, hi, _, ok := s.findConst(name, 0, len(s.toks), 0); ok {
		return lo, hi, nil, true
	}
	return 0, 0, nil, false
}

func (r *slp207Recognizer) classifyIdent(s *slp207Src, name string, scope *int, depth int) slp207Kind {
	if s.mutated(name) || len(s.bindingSites(name)) != 1 {
		return slp207Unknown
	}
	if _, ok := s.valueImport(name); ok {
		return r.receiverKind(s, name, depth+1)
	}
	lo, hi, sc, ok := s.lookupConst(name, scope)
	if !ok {
		return slp207Unknown
	}
	return r.classifyExpr(s, lo, hi, sc, depth+1)
}

// classifyMember classifies `X.p` where X is a constant initialised by a call
// to a local factory function or to a verified scoped-executor factory.
func (r *slp207Recognizer) classifyMember(s *slp207Src, x, p string, scope *int, depth int) slp207Kind {
	if s.mutated(x) || len(s.bindingSites(x)) != 1 {
		return slp207Unknown
	}
	ilo, ihi, sc, ok := s.lookupConst(x, scope)
	if !ok {
		return slp207Unknown
	}
	lo, hi := s.stripTypeWrappers(ilo, ihi)
	if !s.isAnyIdent(lo) {
		return slp207Unknown
	}
	callee := s.toks[lo].text
	j := lo + 1
	typeLo, typeHi := -1, -1
	if s.isPunct(j, "<") {
		gt := s.matchAngle(j)
		if gt < 0 {
			return slp207Unknown
		}
		typeLo, typeHi, j = j+1, gt-1, gt+1
	}
	if !s.isPunct(j, "(") || s.match[j] != hi {
		return slp207Unknown
	}
	if s.mutated(callee) || len(s.bindingSites(callee)) != 1 {
		return slp207Unknown
	}
	// Local factory: every top-level return yields an object literal whose
	// property p is classified independently.
	if _, body, ok := s.findFunction(callee); ok && typeLo < 0 {
		return r.classifyReturnedProperty(s, body, p, depth+1)
	}
	// Imported scoped-executor factory.
	imp, ok := s.valueImport(callee)
	if !ok || typeLo < 0 {
		return slp207Unknown
	}
	modPath, ok := r.resolveModule(s.path, imp.source)
	if !ok {
		return slp207Unknown
	}
	m := r.load(modPath)
	if m == nil || r.forwardingFactoryKey(m, imp.imported) != p {
		return slp207Unknown
	}
	if !s.isDrizzleExecutorType(typeLo, typeHi, 0) {
		return slp207Unknown
	}
	args, ok := s.splitTopLevel(j+1, hi-1)
	if !ok || len(args) != 1 {
		return slp207Unknown
	}
	return slp207Combine(r.classifyExpr(s, args[0][0], args[0][1], sc, depth+1), slp207Managed)
}

func (r *slp207Recognizer) classifyReturnedProperty(s *slp207Src, bodyOpen int, p string, depth int) slp207Kind {
	bodyClose := s.match[bodyOpen]
	regions := s.nestedFunctionRegions(bodyOpen, bodyClose)
	result, seen := slp207Inert, false
	for k := bodyOpen + 1; k < bodyClose; k++ {
		if s.isIdent(k, "yield") || s.isIdent(k, "await") {
			return slp207Unknown
		}
		if !s.isIdent(k, "return") {
			continue
		}
		nested := false
		for _, rg := range regions {
			if k >= rg[0] && k <= rg[1] {
				nested = true
				break
			}
		}
		if nested {
			continue
		}
		e := k + 1
		for e < bodyClose && !(s.isPunct(e, ";") && s.depth[e] == s.depth[k]) {
			if s.isOpener(e) {
				e = s.match[e]
			}
			e++
		}
		if e >= bodyClose {
			return slp207Unknown
		}
		lo, hi := s.stripTypeWrappers(k+1, e-1)
		if !s.isPunct(lo, "{") || s.match[lo] != hi {
			return slp207Unknown
		}
		vlo, vhi, ok := s.objectProperty(lo, p)
		if !ok {
			return slp207Unknown
		}
		bo := bodyOpen
		result = slp207Combine(result, r.classifyExpr(s, vlo, vhi, &bo, depth+1))
		seen = true
		k = e
	}
	if !seen {
		return slp207Unknown
	}
	return result
}

// isDrizzleExecutorType verifies a type expression is a union of Drizzle
// database types (`ReturnType<typeof drizzle…>`) and their transaction handle
// types (`Parameters<Parameters<DB["transaction"]>[0]>[0]`), possibly through
// local type aliases.
func (s *slp207Src) isDrizzleExecutorType(lo, hi, depth int) bool {
	kinds, ok := s.drizzleTypeKinds(lo, hi, depth)
	return ok && kinds != 0
}

const (
	slp207TypeDB = 1 << iota
	slp207TypeTx
)

func (s *slp207Src) drizzleTypeKinds(lo, hi, depth int) (int, bool) {
	if depth > slp207MaxResolveDepth || lo > hi {
		return 0, false
	}
	var members [][2]int
	start, angle := lo, 0
	for k := lo; k <= hi; k++ {
		switch {
		case s.isPunct(k, "<"):
			angle++
		case s.isPunct(k, ">"):
			angle--
		case s.isOpener(k):
			k = s.match[k]
		case s.isPunct(k, "|") && angle == 0:
			if k > start {
				members = append(members, [2]int{start, k - 1})
			}
			start = k + 1
		}
	}
	members = append(members, [2]int{start, hi})
	kinds := 0
	for _, mb := range members {
		a, b := mb[0], mb[1]
		switch {
		case a == b && s.isAnyIdent(a):
			alo, ahi, ok := s.typeAlias(s.toks[a].text)
			if !ok {
				return 0, false
			}
			k, ok := s.drizzleTypeKinds(alo, ahi, depth+1)
			if !ok {
				return 0, false
			}
			kinds |= k
		case s.isIdent(a, "ReturnType") && s.isPunct(a+1, "<") && s.isIdent(a+2, "typeof") && s.isAnyIdent(a+3):
			imp, ok := s.valueImport(s.toks[a+3].text)
			if !ok || imp.imported != "drizzle" || !slp207ManagedDrizzleAdapters[imp.source] {
				return 0, false
			}
			end := a + 4
			if s.isPunct(end, "<") {
				if end = s.matchAngle(end); end < 0 {
					return 0, false
				}
				end++
			}
			if !s.isPunct(end, ">") || end != b {
				return 0, false
			}
			kinds |= slp207TypeDB
		case b-a == 15 && s.isIdent(a, "Parameters") && s.isPunct(a+1, "<") && s.isIdent(a+2, "Parameters") &&
			s.isPunct(a+3, "<") && s.isAnyIdent(a+4) && s.isPunct(a+5, "[") && s.isString(a+6) &&
			s.toks[a+6].text == "transaction" && s.isPunct(a+7, "]") && s.isPunct(a+8, ">") &&
			s.isPunct(a+9, "[") && s.toks[a+10].text == "0" && s.isPunct(a+11, "]") && s.isPunct(a+12, ">") &&
			s.isPunct(a+13, "[") && s.toks[a+14].text == "0" && s.isPunct(a+15, "]"):
			alo, ahi, ok := s.typeAlias(s.toks[a+4].text)
			if !ok {
				return 0, false
			}
			if k, ok := s.drizzleTypeKinds(alo, ahi, depth+1); !ok || k != slp207TypeDB {
				return 0, false
			}
			kinds |= slp207TypeTx
		default:
			return 0, false
		}
	}
	return kinds, true
}

// typeAlias returns the right-hand side of the single top-level, non-generic
// `[export] type name = …;`.
func (s *slp207Src) typeAlias(name string) (int, int, bool) {
	found := -1
	for k := range s.toks {
		if s.depth[k] == 0 && (s.isIdent(k, "type") || s.isIdent(k, "interface")) && s.isIdent(k+1, name) && !s.afterMemberDot(k) {
			if found >= 0 || s.isIdent(k, "interface") || !s.isPunct(k+2, "=") {
				return 0, 0, false
			}
			found = k
		}
	}
	if found < 0 {
		return 0, 0, false
	}
	e := found + 3
	for e < len(s.toks) && !(s.isPunct(e, ";") && s.depth[e] == 0) {
		if s.isOpener(e) {
			e = s.match[e]
		}
		e++
	}
	if e >= len(s.toks) || e-1 < found+3 {
		return 0, 0, false
	}
	return found + 3, e - 1, true
}

// ---------------------------------------------------------------------------
// Proxy forms
// ---------------------------------------------------------------------------

// singleMethod returns the parameter list and body of the only member of the
// object literal opened at lo, which must be the shorthand method `name(...)`.
func (s *slp207Src) singleMethod(lo int, name string) (int, int, bool) {
	parts, ok := s.splitTopLevel(lo+1, s.match[lo]-1)
	if !ok || len(parts) != 1 {
		return 0, 0, false
	}
	return s.methodMember(parts[0][0], parts[0][1], name)
}

func (s *slp207Src) methodMember(a, b int, name string) (int, int, bool) {
	if !s.isIdent(a, name) || !s.isPunct(a+1, "(") {
		return 0, 0, false
	}
	body := s.findBodyBrace(s.match[a+1] + 1)
	if body < 0 || s.match[body] != b {
		return 0, 0, false
	}
	return a + 1, body, true
}

// paramNames returns plain parameter names (type annotations allowed).
func (s *slp207Src) paramNames(open int) ([]string, bool) {
	parts, ok := s.splitTopLevel(open+1, s.match[open]-1)
	if !ok {
		return nil, false
	}
	var names []string
	for _, pr := range parts {
		if !s.isAnyIdent(pr[0]) || !(pr[0] == pr[1] || s.isPunct(pr[0]+1, ":")) {
			return nil, false
		}
		names = append(names, s.toks[pr[0]].text)
	}
	return names, true
}

// classifyInertProxy recognises a stub `new Proxy(target, { get(...) { ... } })`
// whose get trap only returns undefined or a callable proxy whose apply trap
// throws, so invoking any property (including `transaction`) throws before a
// transaction can begin.
func (r *slp207Recognizer) classifyInertProxy(s *slp207Src, open int) slp207Kind {
	args, ok := s.splitTopLevel(open+1, s.match[open]-1)
	if !ok || len(args) != 2 || !s.isPunct(args[1][0], "{") || s.match[args[1][0]] != args[1][1] {
		return slp207Unknown
	}
	_, body, ok := s.singleMethod(args[1][0], "get")
	if !ok {
		return slp207Unknown
	}
	stmts, ok := s.statements(body+1, s.match[body])
	if !ok {
		return slp207Unknown
	}
	handlers := map[string]int{}
	for _, st := range stmts {
		a, b := st[0], st[1]
		switch {
		case s.isIdent(a, "if") && s.isPunct(a+1, "("):
			c := s.match[a+1]
			if !s.conditionIsPure(a+2, c-1) || !s.isIdent(c+1, "return") || !s.isIdent(c+2, "undefined") ||
				!s.isPunct(c+3, ";") || c+3 != b {
				return slp207Unknown
			}
		case s.isIdent(a, "const") && s.isAnyIdent(a+1) && s.isPunct(a+2, "=") && s.isPunct(a+3, "{") &&
			s.isPunct(s.match[a+3]+1, ";") && s.match[a+3]+1 == b:
			handlers[s.toks[a+1].text] = a + 3
		case s.isIdent(a, "return") && s.isIdent(a+1, "undefined") && a+2 == b && s.isPunct(b, ";"):
		case s.isIdent(a, "return") && s.isIdent(a+1, "new") && s.isIdent(a+2, "Proxy") && s.isPunct(a+3, "(") &&
			s.isPunct(s.match[a+3]+1, ";") && s.match[a+3]+1 == b:
			inner, ok := s.splitTopLevel(a+4, s.match[a+3]-1)
			if !ok || len(inner) != 2 || !s.isFunctionLiteral(inner[0][0], inner[0][1]) {
				return slp207Unknown
			}
			h := -1
			if hl, hh := inner[1][0], inner[1][1]; hl == hh && s.isAnyIdent(hl) {
				if idx, ok := handlers[s.toks[hl].text]; ok {
					h = idx
				}
			} else if s.isPunct(hl, "{") && s.match[hl] == hh {
				h = hl
			}
			if h < 0 || !s.applyTrapThrows(h) {
				return slp207Unknown
			}
		default:
			return slp207Unknown
		}
	}
	return slp207Inert
}

func (s *slp207Src) conditionIsPure(lo, hi int) bool {
	for k := lo; k <= hi; k++ {
		if s.isOpener(k) || s.isAssignOp(k) || s.isPunct(k, "++") || s.isPunct(k, "--") {
			return false
		}
	}
	return lo <= hi
}

func (s *slp207Src) isFunctionLiteral(lo, hi int) bool {
	if s.isPunct(lo, "(") && s.isPunct(s.match[lo]+1, "=>") {
		b := s.match[lo] + 2
		return s.isPunct(b, "{") && s.match[b] == hi
	}
	if s.isIdent(lo, "function") && s.isPunct(lo+1, "(") {
		b := s.findBodyBrace(s.match[lo+1] + 1)
		return b >= 0 && s.match[b] == hi
	}
	return false
}

// applyTrapThrows verifies the handler object literal opened at h has an
// `apply` trap whose body is a single throw statement.
func (s *slp207Src) applyTrapThrows(h int) bool {
	parts, ok := s.splitTopLevel(h+1, s.match[h]-1)
	if !ok {
		return false
	}
	n := 0
	for _, pr := range parts {
		if !s.isIdent(pr[0], "apply") {
			continue
		}
		_, body, ok := s.methodMember(pr[0], pr[1], "apply")
		if !ok {
			return false
		}
		stmts, ok := s.statements(body+1, s.match[body])
		if !ok || len(stmts) != 1 || !s.isIdent(stmts[0][0], "throw") {
			return false
		}
		n++
	}
	return n == 1
}

// forwardingFactoryKey verifies that the exported function name in m returns
// an object whose returned property (the key, e.g. "executor") is a Proxy
// that forwards every property read to the AsyncLocalStorage-selected
// executor of the factory's type parameter, or to the fallback argument
// outside a scope, binding methods to that executor. It returns "" if the
// shape is not proven.
func (r *slp207Recognizer) forwardingFactoryKey(m *slp207Src, name string) string {
	cacheKey := m.path + "\x00" + name
	if key, ok := r.factories[cacheKey]; ok {
		return key
	}
	key := m.computeForwardingFactoryKey(name)
	r.factories[cacheKey] = key
	return key
}

func (s *slp207Src) computeForwardingFactoryKey(name string) string {
	if len(s.bindingSites(name)) != 1 || s.mutated(name) || len(s.bindingSites("Proxy")) != 0 || len(s.bindingSites("Reflect")) != 0 {
		return ""
	}
	params, body, ok := s.findFunction(name)
	if !ok || !s.isPunct(params-1, ">") {
		return ""
	}
	// The factory must be generic: `function name<T …>(fallback[: T])`.
	lt := -1
	for k := params - 1; k >= 1; k-- {
		if s.isIdent(k, name) && s.isIdent(k-1, "function") {
			lt = k + 1
			break
		}
	}
	if lt < 0 || !s.isPunct(lt, "<") || !s.isAnyIdent(lt+1) || s.matchAngle(lt) != params-1 {
		return ""
	}
	typeParam := s.toks[lt+1].text
	c := s.match[params]
	if !s.isAnyIdent(params+1) || !(params+2 == c || (s.isPunct(params+2, ":") && s.isIdent(params+3, typeParam) && params+4 == c)) {
		return ""
	}
	fallback := s.toks[params+1].text
	if s.mutated(fallback) {
		return ""
	}
	stmts, ok := s.statements(body+1, s.match[body])
	if !ok || len(stmts) != 3 {
		return ""
	}
	// 1. const ctx = new AsyncLocalStorage<{ executor: T; ... }>();
	a, b := stmts[0][0], stmts[0][1]
	if !s.isIdent(a, "const") || !s.isAnyIdent(a+1) || !s.isPunct(a+2, "=") || !s.isIdent(a+3, "new") ||
		!s.isAnyIdent(a+4) || !s.isPunct(a+5, "<") {
		return ""
	}
	ctx := s.toks[a+1].text
	als, ok := s.valueImport(s.toks[a+4].text)
	if !ok || als.imported != "AsyncLocalStorage" || !slp207AsyncLocalStorageModules[als.source] {
		return ""
	}
	gt := s.matchAngle(a + 5)
	if gt < 0 || !s.isPunct(a+6, "{") || s.match[a+6] != gt-1 || !s.isPunct(gt+1, "(") || s.match[gt+1] != gt+2 ||
		!s.isPunct(gt+3, ";") || gt+3 != b || s.mutated(ctx) {
		return ""
	}
	storeTyped := false
	for k := a + 7; k < gt-1; k++ {
		if s.depth[k] == s.depth[a+6]+1 && s.isIdent(k, "executor") && s.isPunct(k+1, ":") && s.isIdent(k+2, typeParam) &&
			(s.isPunct(k+3, ";") || s.isPunct(k+3, ",") || k+3 == gt-1) {
			storeTyped = true
		}
	}
	if !storeTyped {
		return ""
	}
	// 2. const px = new Proxy(fallback, { get(target, property) { ... } });
	a, b = stmts[1][0], stmts[1][1]
	if !s.isIdent(a, "const") || !s.isAnyIdent(a+1) || !s.isPunct(a+2, "=") || !s.isIdent(a+3, "new") ||
		!s.isIdent(a+4, "Proxy") || !s.isPunct(a+5, "(") || !s.isPunct(s.match[a+5]+1, ";") || s.match[a+5]+1 != b {
		return ""
	}
	px := s.toks[a+1].text
	if s.mutated(px) {
		return ""
	}
	args, ok := s.splitTopLevel(a+6, s.match[a+5]-1)
	if !ok || len(args) != 2 || args[0][0] != args[0][1] || !s.isIdent(args[0][0], fallback) ||
		!s.isPunct(args[1][0], "{") || s.match[args[1][0]] != args[1][1] {
		return ""
	}
	getParams, getBody, ok := s.singleMethod(args[1][0], "get")
	if !ok {
		return ""
	}
	names, ok := s.paramNames(getParams)
	if !ok || len(names) != 2 || !s.forwardingGetBody(getBody, ctx, names[0], names[1]) {
		return ""
	}
	// 3. return { executor: px, ... } — the key mapping to px is returned.
	a, b = stmts[2][0], stmts[2][1]
	if !s.isIdent(a, "return") || !s.isPunct(a+1, "{") || !s.isPunct(s.match[a+1]+1, ";") || s.match[a+1]+1 != b {
		return ""
	}
	parts, ok := s.splitTopLevel(a+2, s.match[a+1]-1)
	if !ok {
		return ""
	}
	key := ""
	for _, pr := range parts {
		x, y := pr[0], pr[1]
		if s.isPunct(x, "...") || s.isPunct(x, "[") {
			return ""
		}
		switch {
		case x == y && s.isIdent(x, px):
			if key != "" {
				return ""
			}
			key = px
		case s.isAnyIdent(x) && s.isPunct(x+1, ":") && x+2 == y && s.isIdent(y, px):
			if key != "" {
				return ""
			}
			key = s.toks[x].text
		}
	}
	return key
}

// forwardingGetBody verifies the get trap body. The statements must appear in
// this order: a const binding of the scope read from the context store; zero
// or more guards whose pure condition leads only to a throw statement; a const
// binding of the current executor taken from the scope executor with the trap
// target as the nullish fallback, or the trap target alone; a const binding of
// Reflect.get on that executor with the trap property and the same executor
// as receiver; and a final return of that value, either unchanged or bound to
// the executor when the value is a function.
func (s *slp207Src) forwardingGetBody(body int, ctx, target, prop string) bool {
	stmts, ok := s.statements(body+1, s.match[body])
	if !ok || len(stmts) == 0 {
		return false
	}
	scopeVar, current, value := "", "", ""
	for i, st := range stmts {
		a, b := st[0], st[1]
		last := i == len(stmts)-1
		isConst := s.isIdent(a, "const") && s.isAnyIdent(a+1) && s.isPunct(a+2, "=") && s.isPunct(b, ";")
		switch {
		case isConst && scopeVar == "" && s.isIdent(a+3, ctx) && s.isPunct(a+4, ".") && s.isIdent(a+5, "getStore") &&
			s.isPunct(a+6, "(") && s.isPunct(a+7, ")") && a+8 == b:
			scopeVar = s.toks[a+1].text
		case s.isIdent(a, "if") && s.isPunct(a+1, "("):
			c := s.match[a+1]
			if !s.conditionIsPure(a+2, c-1) {
				return false
			}
			t := c + 1
			if s.isPunct(t, "{") {
				inner, ok := s.statements(t+1, s.match[t])
				if !ok || len(inner) != 1 || !s.isIdent(inner[0][0], "throw") || s.match[t] != b {
					return false
				}
			} else if !s.isIdent(t, "throw") || !s.isPunct(b, ";") {
				return false
			}
		case isConst && current == "" && scopeVar != "" && s.isIdent(a+3, scopeVar) && s.isPunct(a+4, "?.") &&
			s.isIdent(a+5, "executor") && s.isPunct(a+6, "??") && s.isIdent(a+7, target) && a+8 == b:
			current = s.toks[a+1].text
		case isConst && current == "" && s.isIdent(a+3, target) && a+4 == b:
			current = s.toks[a+1].text
		case isConst && value == "" && current != "" && s.isIdent(a+3, "Reflect") && s.isPunct(a+4, ".") &&
			s.isIdent(a+5, "get") && s.isPunct(a+6, "(") && s.isIdent(a+7, current) && s.isPunct(a+8, ",") &&
			s.isIdent(a+9, prop) && s.isPunct(a+10, ",") && s.isIdent(a+11, current) && s.isPunct(a+12, ")") && a+13 == b:
			value = s.toks[a+1].text
		case last && value != "" && s.isIdent(a, "return") && s.isIdent(a+1, value) && a+2 == b && s.isPunct(b, ";"):
			return true
		case last && value != "" && s.isIdent(a, "return") && s.isIdent(a+1, "typeof") && s.isIdent(a+2, value) &&
			s.isPunct(a+3, "===") && s.isString(a+4) && s.toks[a+4].text == "function" && s.isPunct(a+5, "?") &&
			s.isIdent(a+6, value) && s.isPunct(a+7, ".") && s.isIdent(a+8, "bind") && s.isPunct(a+9, "(") &&
			s.isIdent(a+10, current) && s.isPunct(a+11, ")") && s.isPunct(a+12, ":") && s.isIdent(a+13, value) &&
			a+14 == b && s.isPunct(b, ";"):
			return true
		default:
			return false
		}
	}
	return false
}

// slp207InSpans reports whether token index k lies inside any inclusive span.
func slp207InSpans(k int, spans [][2]int) bool {
	for _, sp := range spans {
		if k >= sp[0] && k <= sp[1] {
			return true
		}
	}
	return false
}

// isImportToken reports whether token index k is the local binding of a
// parsed import specifier.
func (s *slp207Src) isImportToken(k int) bool {
	for _, imp := range s.imports {
		if imp.idx == k {
			return true
		}
	}
	return false
}
