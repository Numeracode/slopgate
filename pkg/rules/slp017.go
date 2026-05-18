package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP017 flags magic numbers — unexplained numeric literals in public API,
// configuration, or business-domain contexts. AI agents frequently sprinkle
// raw literals (tax rates, thresholds, policy values) instead of defining
// named constants, making code fragile and hard to review.
//
// Exempt: 0, 1, 2; hex/octal literals; array index patterns [N];
// constant/define declarations; ALL_CAPS assignments; test files;
// doc files.
type SLP017 struct{}

func (SLP017) ID() string                { return "SLP017" }
func (SLP017) DefaultSeverity() Severity { return SeverityInfo }
func (SLP017) Description() string {
	return "unexplained numeric literal — define a named constant instead"
}

// slp017Number matches decimal integer or float literals (not hex/octal).
var slp017Number = regexp.MustCompile(`(?:^|[^\w.])((?:0|[1-9]\d*)(?:\.\d+)?)(?:[^\w.]|$)`)

// slp017SmallNumber matches 0, 1, or 2 (common innocuous values).
var slp017SmallNumber = regexp.MustCompile(`^(?:[012]|10|20|30|40|50|60|70|80|90|100|200|300|400|500|1000|2000|3000|4000|5000|10000)$`)

// slp017InnocuousFunction matches timer/wait-style function calls where numeric literals are often expected.
var slp017InnocuousFunction = regexp.MustCompile(`(?i)\b(?:setTimeout|setInterval|delay|sleep|wait)\s*\(`)

// slp017HTTPStatus matches common HTTP status codes that are intentional.
// These are not "magic numbers" — they're standard API response codes.
var slp017HTTPStatus = regexp.MustCompile(`^[1-5][0-9][0-9]$`)

// slp017CommonLimit matches typical pagination/query limits.
// 5, 10, 20, 25, 50, 100, 1000 are standard batch sizes.
var slp017CommonLimit = regexp.MustCompile(`^[5-9]$|^1[0-9]$|^2[0-5]$|^50$|^100$|^1000$`)

// slp017HTTPStatusContext matches HTTP status usage context.
// If line contains .status(), res.status, statusCode, etc., treat 2xx/4xx/5xx as intentional.
var slp017HTTPStatusContext = regexp.MustCompile(`(?i)\.status\s*\(|status\s*[=:]\s*\d|statusCode|httpStatus|response\.status`)

// slp017LimitContext matches limit/batch/pagination context.
// If line contains LIMIT, limit, pageSize, batch, etc., treat common limits as intentional.
var slp017LimitContext = regexp.MustCompile(`(?i)LIMIT\s+\d|limit\s*[=:]\s*\d|pageSize|page_size|batchSize|batch_size|max.*=.*\d|take\s*\(\s*\d|top\s*\d|first\s*\d|\.limit\s*\(`)

// slp017BusinessContext narrows SLP017 to literals that look like public API,
// configuration, or business-domain values rather than incidental arithmetic.
var slp017BusinessContext = regexp.MustCompile(`(?i)\b(?:rate|ratio|percent|percentage|threshold|quota|budget|score|weight|multiplier|factor|tax|fee|price|discount|sampling|sample|rollout|retention|version|policy|config|setting|limit|maximum|minimum|timeout|ttl|retry|interval|batch|page\s*size|capacity|concurrency|port)\b`)

var slp017BusinessPathContext = regexp.MustCompile(`(?i)(?:^|/)(?:config|configs|settings|constants|env|policy|policies)/|(?:^|/)[^/]*(?:config|settings|constants|policy)[^/]*\.(?:go|ts|tsx|js|jsx|py|java|rs)$`)

var slp017CamelIdentifierBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

var slp017IdentifierSeparators = strings.NewReplacer("_", " ", "-", " ")

// slp017MeasurementContext matches descriptive size/duration/validation fields.
// These literals are usually schema bounds, UI geometry, or timing knobs already
// covered better by more specific rules than generic magic-number detection.
var slp017MeasurementContext = regexp.MustCompile(`(?i)\b(?:len|length|width|height|size|depth|count|duration|delay|timeout|ttl|retry|retries|interval|capacity|buffer|chunk|offset|page|concurrency|radius|opacity)(?:[A-Z][A-Za-z0-9_]*)?\b|\.length\b|\b(?:max|min)(?:imum)?[A-Z_]|(?:^|[^\w])(?:max|min)(?:[A-Z_]|[a-z])`)

// slp017HexOctal matches hex (0x...) or octal (0o...) literals.
var slp017HexOctal = regexp.MustCompile(`0[xXoO][\da-fA-F]+`)

// slp017ArrayIndex matches [N] where N is a literal number.
var slp017ArrayIndex = regexp.MustCompile(`\[\d+\]`)

// slp017ConstDecl matches constant/define declarations.
var slp017ConstDecl = regexp.MustCompile(`(?:^|\s)(?:const|final|static\s+final|#define)\s`)

// slp017AllCapsAssign matches ALL_CAPS_NAME = or ALL_CAPS_NAME :=.
var slp017AllCapsAssign = regexp.MustCompile(`[A-Z][A-Z0-9_]*\s*:?=`)

// slp017MaskMeasurementContexts replaces measurement context tokens and their
// associated numbers with placeholders, so unrelated literals on the same line
// are still checked for magic numbers.
func slp017MaskMeasurementContexts(s string) string {
	// Find all measurement context tokens.
	tokens := slp017MeasurementContext.FindAllStringIndex(s, -1)
	if len(tokens) == 0 {
		return s
	}

	// Build a set of character ranges to mask.
	mask := make([]bool, len(s))
	for _, tok := range tokens {
		if len(tok) < 2 {
			continue
		}
		// Mask the token itself.
		start, end := tok[0], tok[1]
		if start >= 0 && end <= len(mask) {
			for k := start; k < end; k++ {
				if k < len(mask) {
					mask[k] = true
				}
			}
		}
		// Also mask the number that follows (e.g., "timeout: 200" or "width=800" or ".length > 1024").
		// Skip whitespace and assignment/comparison separators.
		j := end
		for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == ':' || s[j] == '=' || s[j] == ',' || s[j] == '>' || s[j] == '<') {
			if j < len(mask) {
				mask[j] = true
			}
			j++
		}
		// Handle >= and <=
		if j < len(s) && j < len(mask) && s[j] == '=' {
			mask[j] = true
			j++
		}
		// Mask the number.
		for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.') {
			mask[j] = true
			j++
		}
	}

	// Build masked string.
	var b strings.Builder
	for i, c := range s {
		if mask[i] {
			b.WriteByte('X')
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func (r SLP017) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete || isDocFile(f.Path) {
			continue
		}
		if !isSourceLikeFile(f.Path) {
			continue
		}
		isTest := isGoTestFile(f.Path) || isJavaTestFile(f.Path) ||
			isPythonTestFile(f.Path) || isJSTestFile(f.Path) || isRustTestFile(f.Path)
		if isTest {
			continue
		}
		for _, ln := range f.AddedLines() {
			clean := stripCommentAndStrings(ln.Content)
			if clean == "" {
				continue
			}
			trimmed := strings.TrimSpace(clean)
			// Skip constant/define declarations with ALL_CAPS naming.
			if slp017ConstDecl.MatchString(trimmed) && slp017AllCapsAssign.MatchString(trimmed) {
				continue
			}
			// Skip ALL_CAPS assignments (likely constant definitions).
			if slp017AllCapsAssign.MatchString(trimmed) {
				continue
			}
			// Skip hex/octal literals — they're usually bitmasks.
			if slp017HexOctal.MatchString(clean) {
				continue
			}
			// Blank out array index patterns so [3] doesn't count as magic.
			clean = slp017ArrayIndex.ReplaceAllString(clean, "[_]")

			// Check for HTTP status context — exempt status codes.
			isHTTPContext := slp017HTTPStatusContext.MatchString(clean)
			// Check for limit/batch context — exempt common limits.
			isLimitContext := slp017LimitContext.MatchString(clean)
			// Keep SLP017 focused on public API/config/business values instead
			// of local implementation arithmetic.
			isBusinessContext := slp017HasBusinessContext(f.Path, clean)
			// Mask measurement context tokens and their associated numbers
			// so unrelated literals on the same line are still checked.
			clean = slp017MaskMeasurementContexts(clean)

			for _, m := range slp017Number.FindAllStringSubmatchIndex(clean, -1) {
				if len(m) < 2 {
					continue
				}
				num := clean[m[2]:m[3]]
				if slp017SmallNumber.MatchString(num) {
					continue
				}
				// Exempt literals inside innocuous function calls (bounded-span heuristic).
				inInnocuous := false
				for _, fnMatch := range slp017InnocuousFunction.FindAllStringIndex(clean, -1) {
					callEnd := findMatchingParen(clean, fnMatch[1]-1)
					if callEnd < 0 {
						continue
					}
					// Literal must lie entirely inside the call arguments.
					if fnMatch[1] > m[2] || m[3] > callEnd {
						continue
					}
					// No semicolon between the function start and the literal.
					if strings.Contains(clean[fnMatch[0]:m[2]], ";") {
						continue
					}
					// Reject literals nested inside other parens/brackets/braces.
					if strings.ContainsAny(clean[fnMatch[1]:m[2]], "([{") {
						continue
					}
					// For timer functions, require the literal to be a top-level
					// delay argument (0 or 1 top-level commas with no open nesting).
					fnName := strings.TrimSpace(clean[fnMatch[0] : fnMatch[1]-1])
					if isTimerFn(fnName) {
						prefix := clean[fnMatch[1]:m[2]]
						if !isTopLevelDelayArg(prefix) {
							continue
						}
					}
					inInnocuous = true
					break
				}
				if inInnocuous {
					continue
				}
				// Exempt HTTP status codes in HTTP context.
				if isHTTPContext && slp017HTTPStatus.MatchString(num) {
					continue
				}
				// Exempt common limits in limit context.
				if isLimitContext && slp017CommonLimit.MatchString(num) {
					continue
				}
				if !isBusinessContext {
					continue
				}
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.DefaultSeverity(),
					File:     f.Path,
					Line:     ln.NewLineNo,
					Message:  fmt.Sprintf("magic number %s — define a named constant for clarity", num),
					Snippet:  strings.TrimSpace(ln.Content),
				})
				break
			}
		}
	}
	return out
}

func slp017HasBusinessContext(filePath, content string) bool {
	normalized := slp017CamelIdentifierBoundary.ReplaceAllString(content, "$1 $2")
	normalized = slp017IdentifierSeparators.Replace(normalized)
	return slp017BusinessContext.MatchString(normalized) || slp017BusinessPathContext.MatchString(filePath)
}

// findMatchingParen returns the index of the ')' that matches the '(' at openParenPos.
// Returns -1 if no matching paren is found.
func findMatchingParen(s string, openParenPos int) int {
	if openParenPos < 0 || openParenPos >= len(s) || s[openParenPos] != '(' {
		return -1
	}
	depth := 1
	for i := openParenPos + 1; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// isTimerFn reports whether fnName is a timer function (setTimeout/setInterval).
func isTimerFn(fnName string) bool {
	lower := strings.ToLower(fnName)
	return strings.Contains(lower, "timeout") || strings.Contains(lower, "interval")
}

// isTopLevelDelayArg reports whether prefix (text between the function's opening paren
// and the numeric literal) indicates the literal is a top-level delay argument:
// 0 or 1 top-level commas with no open nesting. Arrow functions short-circuit to true
// since their parens make simple depth tracking unreliable.
func isTopLevelDelayArg(prefix string) bool {
	if strings.Contains(prefix, "=>") {
		return true
	}
	depth := 0
	commas := 0
	for _, ch := range prefix {
		switch ch {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				commas++
			}
		}
	}
	return depth == 0 && commas <= 1
}
