package rules

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// This file contains a deliberately small JavaScript/TypeScript lexer used
// only by SLP207's managed-transaction recognizer (slp207_managed.go). It is
// not a parser. It produces a token stream with comments removed, string and
// template literals isolated (template substitutions are lexed as ordinary
// tokens), and bracket pairs matched. Any input it does not understand makes
// it fail, and every caller treats failure as "not proven", so the original
// SLP207 heuristic still applies.

type slp207TokKind int

const (
	slp207TokIdent slp207TokKind = iota
	slp207TokPunct
	slp207TokString
	slp207TokTemplate
	slp207TokNumber
	slp207TokRegex
)

type slp207Tok struct {
	kind slp207TokKind
	text string // identifier/punctuator text; literal body for strings/templates
	line int    // 1-based source line of the token's first character
}

// slp207Src is a lexed source file plus the derived bracket structure.
type slp207Src struct {
	path   string
	lines  []string
	toks   []slp207Tok
	match  []int // index of the matching bracket token, or -1
	depth  []int // number of open brackets enclosing each token
	parent []int // index of the innermost enclosing opener, or -1

	imports       []slp207Import
	importsParsed bool
}

// slp207Puncts is ordered longest-first. '>' is intentionally always emitted
// as a single-character token so nested TypeScript generics close cleanly.
var slp207Puncts = []string{
	"...", "===", "!==", "**=", "<<=", "&&=", "||=", "??=",
	"=>", "==", "!=", "<=", "&&", "||", "??", "?.", "++", "--",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "**", "<<", "${",
}

// Keywords after which a '/' starts a regular expression literal.
var slp207RegexAfterKeyword = map[string]bool{
	"return": true, "typeof": true, "case": true, "do": true, "else": true,
	"in": true, "of": true, "new": true, "delete": true, "void": true,
	"throw": true, "instanceof": true, "yield": true, "await": true,
}

func slp207IsIdentStart(r rune) bool {
	return r == '_' || r == '$' || r == '#' || unicode.IsLetter(r)
}

func slp207IsIdentPart(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// slp207Lexer holds the mutable scanning state for slp207Lex.
type slp207Lexer struct {
	s    *slp207Src
	src  string
	i    int
	line int
	// braceKinds tracks open '{' (value '{') versus template substitutions
	// (value '$') so a '}' can resume template scanning.
	braceKinds []byte
}

func (lx *slp207Lexer) emit(kind slp207TokKind, text string, ln int) {
	lx.s.toks = append(lx.s.toks, slp207Tok{kind: kind, text: text, line: ln})
}

// regexAllowed reports whether a '/' at the current position starts a regex
// literal rather than a division operator, based on the previous token.
func (lx *slp207Lexer) regexAllowed() bool {
	if len(lx.s.toks) == 0 {
		return true
	}
	p := lx.s.toks[len(lx.s.toks)-1]
	switch p.kind {
	case slp207TokIdent:
		return slp207RegexAfterKeyword[p.text]
	case slp207TokPunct:
		return p.text != ")" && p.text != "]" && p.text != "}"
	default:
		return false
	}
}

// scanTemplate scans template characters starting at lx.i (just after '`'
// or after the '}' closing a substitution). It emits the literal chunk and
// either finishes the template or opens a substitution.
func (lx *slp207Lexer) scanTemplate() bool {
	src := lx.src
	start, startLine := lx.i, lx.line
	for lx.i < len(src) {
		c := src[lx.i]
		switch {
		case c == '\\':
			if lx.i+1 < len(src) && src[lx.i+1] == '\n' {
				lx.line++
			}
			lx.i += 2
		case c == '`':
			lx.emit(slp207TokTemplate, src[start:lx.i], startLine)
			lx.i++
			return true
		case c == '$' && lx.i+1 < len(src) && src[lx.i+1] == '{':
			lx.emit(slp207TokTemplate, src[start:lx.i], startLine)
			lx.emit(slp207TokPunct, "${", lx.line)
			lx.braceKinds = append(lx.braceKinds, '$')
			lx.i += 2
			return true
		default:
			if c == '\n' {
				lx.line++
			}
			lx.i++
		}
	}
	return false
}

// slp207Lex lexes src. It returns ok=false for anything it cannot handle
// (unterminated literals, unbalanced brackets, JSX-like input, etc.).
func slp207Lex(path, src string) (*slp207Src, bool) {
	s := &slp207Src{path: path, lines: strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")}
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lx := &slp207Lexer{s: s, src: src, line: 1}
	if strings.HasPrefix(src, "#!") { // hashbang
		for lx.i < len(src) && src[lx.i] != '\n' {
			lx.i++
		}
	}

	for lx.i < len(src) {
		c := src[lx.i]
		switch {
		case c == '\n':
			lx.line++
			lx.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			lx.i++
		case c == '/' && lx.i+1 < len(src) && src[lx.i+1] == '/':
			for lx.i < len(src) && src[lx.i] != '\n' {
				lx.i++
			}
		case c == '/' && lx.i+1 < len(src) && src[lx.i+1] == '*':
			end := strings.Index(src[lx.i+2:], "*/")
			if end < 0 {
				return nil, false
			}
			lx.line += strings.Count(src[lx.i:lx.i+2+end+2], "\n")
			lx.i += 2 + end + 2
		case c == '"' || c == '\'':
			start := lx.i + 1
			lx.i++
			for lx.i < len(src) && src[lx.i] != c {
				if src[lx.i] == '\n' {
					return nil, false
				}
				if src[lx.i] == '\\' {
					if lx.i+1 < len(src) && src[lx.i+1] == '\n' {
						lx.line++
					}
					lx.i++
				}
				lx.i++
			}
			if lx.i >= len(src) {
				return nil, false
			}
			lx.emit(slp207TokString, src[start:lx.i], lx.line)
			lx.i++
		case c == '`':
			lx.i++
			if !lx.scanTemplate() {
				return nil, false
			}
		case c == '/' && lx.regexAllowed():
			start, inClass := lx.i, false
			lx.i++
			for lx.i < len(src) {
				ch := src[lx.i]
				if ch == '\n' {
					return nil, false
				}
				if ch == '\\' {
					lx.i += 2
					continue
				}
				if ch == '[' {
					inClass = true
				} else if ch == ']' {
					inClass = false
				} else if ch == '/' && !inClass {
					break
				}
				lx.i++
			}
			if lx.i >= len(src) {
				return nil, false
			}
			lx.i++
			for lx.i < len(src) && src[lx.i] >= 'a' && src[lx.i] <= 'z' {
				lx.i++
			}
			lx.emit(slp207TokRegex, src[start:lx.i], lx.line)
		case c >= '0' && c <= '9' || (c == '.' && lx.i+1 < len(src) && src[lx.i+1] >= '0' && src[lx.i+1] <= '9'):
			start := lx.i
			for lx.i < len(src) && (src[lx.i] == '.' || src[lx.i] == '_' || src[lx.i] >= '0' && src[lx.i] <= '9' ||
				src[lx.i] >= 'a' && src[lx.i] <= 'z' || src[lx.i] >= 'A' && src[lx.i] <= 'Z') {
				lx.i++
			}
			lx.emit(slp207TokNumber, src[start:lx.i], lx.line)
		default:
			r, size := utf8.DecodeRuneInString(src[lx.i:])
			if slp207IsIdentStart(r) {
				start := lx.i
				lx.i += size
				for lx.i < len(src) {
					r2, sz := utf8.DecodeRuneInString(src[lx.i:])
					if !slp207IsIdentPart(r2) {
						break
					}
					lx.i += sz
				}
				lx.emit(slp207TokIdent, src[start:lx.i], lx.line)
				continue
			}
			if c == '}' && len(lx.braceKinds) > 0 && lx.braceKinds[len(lx.braceKinds)-1] == '$' {
				lx.braceKinds = lx.braceKinds[:len(lx.braceKinds)-1]
				lx.emit(slp207TokPunct, "}", lx.line)
				lx.i++
				if !lx.scanTemplate() {
					return nil, false
				}
				continue
			}
			p := ""
			for _, cand := range slp207Puncts {
				if strings.HasPrefix(src[lx.i:], cand) {
					p = cand
					break
				}
			}
			if p == "?." && lx.i+2 < len(src) && src[lx.i+2] >= '0' && src[lx.i+2] <= '9' {
				p = "?" // `a?.5:b` is a conditional, not optional chaining
			}
			if p == "" {
				if !strings.ContainsRune("{}()[];,<>+-*%&|^!~?:=.@/", rune(c)) {
					return nil, false
				}
				p = string(c)
			}
			switch p {
			case "{":
				lx.braceKinds = append(lx.braceKinds, '{')
			case "}":
				if len(lx.braceKinds) == 0 {
					return nil, false
				}
				lx.braceKinds = lx.braceKinds[:len(lx.braceKinds)-1]
			}
			lx.emit(slp207TokPunct, p, lx.line)
			lx.i += len(p)
		}
	}
	if !s.computeStructure() {
		return nil, false
	}
	return s, true
}

// computeStructure fills match and depth; it fails on unbalanced brackets.
func (s *slp207Src) computeStructure() bool {
	s.match = make([]int, len(s.toks))
	s.depth = make([]int, len(s.toks))
	s.parent = make([]int, len(s.toks))
	var stack []int
	for k, t := range s.toks {
		s.match[k] = -1
		s.depth[k] = len(stack)
		s.parent[k] = -1
		if len(stack) > 0 {
			s.parent[k] = stack[len(stack)-1]
		}
		if t.kind != slp207TokPunct {
			continue
		}
		switch t.text {
		case "(", "[", "{", "${":
			stack = append(stack, k)
		case ")", "]", "}":
			if len(stack) == 0 {
				return false
			}
			o := stack[len(stack)-1]
			want := map[string]string{")": "(", "]": "[", "}": "{"}[t.text]
			if s.toks[o].text != want && !(t.text == "}" && s.toks[o].text == "${") {
				return false
			}
			stack = stack[:len(stack)-1]
			s.match[o], s.match[k] = k, o
			s.depth[k] = len(stack)
			s.parent[k] = s.parent[o]
		}
	}
	return len(stack) == 0
}

func (s *slp207Src) isPunct(k int, p string) bool {
	return k >= 0 && k < len(s.toks) && s.toks[k].kind == slp207TokPunct && s.toks[k].text == p
}

func (s *slp207Src) isIdent(k int, name string) bool {
	return k >= 0 && k < len(s.toks) && s.toks[k].kind == slp207TokIdent && s.toks[k].text == name
}

func (s *slp207Src) isAnyIdent(k int) bool {
	return k >= 0 && k < len(s.toks) && s.toks[k].kind == slp207TokIdent
}

func (s *slp207Src) isString(k int) bool {
	return k >= 0 && k < len(s.toks) && s.toks[k].kind == slp207TokString
}

// afterMemberDot reports whether token k is a property name (`x.k`, `x?.k`).
func (s *slp207Src) afterMemberDot(k int) bool {
	return s.isPunct(k-1, ".") || s.isPunct(k-1, "?.")
}

// isOpener reports whether token k opens a bracket group.
func (s *slp207Src) isOpener(k int) bool {
	return k >= 0 && k < len(s.toks) && s.toks[k].kind == slp207TokPunct && s.match[k] > k
}
