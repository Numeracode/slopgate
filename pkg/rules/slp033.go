package rules

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/messagesgoel-blip/slopgate/pkg/diff"
)

// SLP033 flags missing or improper import statements in TypeScript/JavaScript files.
//
// Pattern: Files using types/functions without proper imports.
//
// Rationale: Missing imports cause runtime errors and type checking failures.
type SLP033 struct{}

func (SLP033) ID() string                { return "SLP033" }
func (SLP033) DefaultSeverity() Severity { return SeverityWarn }
func (SLP033) Description() string {
	return "missing import statement for referenced type/function"
}

// slp033CommonTypes lists common types that should be imported.
var slp033CommonTypes = []string{
	"React", "Component", "FunctionComponent", "ReactNode", "ReactElement", "ComponentProps",
	"MouseEvent", "KeyboardEvent", "ChangeEvent", "FormEvent",
	"ComponentType", "PropsWithChildren", "Dispatch", "SetStateAction",
	"RefObject", "MutableRefObject", "ForwardedRef",
	"CSSProperties", "HTMLElement", "HTMLAttributes", "DetailedHTMLProps",
}

// slp033ReactHooks lists React hooks that should be imported from React.
var slp033ReactHooks = []string{
	"useState", "useEffect", "useContext", "useReducer", "useCallback",
	"useMemo", "useRef", "useImperativeHandle", "useLayoutEffect", "useDebugValue",
	"useDeferredValue", "useId", "useSyncExternalStore", "useTransition",
}

// slp033NamespaceImport matches namespace imports like "import * as React from 'react'".
var slp033NamespaceImport = regexp.MustCompile(`(?i)import\s+(?:type\s+)?\*\s+as\s+(\w+)\s+from\s+["']`)

func (r SLP033) Check(d *diff.Diff) []Finding {
	var out []Finding
	for _, f := range d.Files {
		if f.IsDelete {
			continue
		}

		// Only check TypeScript/JavaScript files
		lowerPath := strings.ToLower(f.Path)
		if !strings.HasSuffix(lowerPath, ".ts") &&
			!strings.HasSuffix(lowerPath, ".tsx") &&
			!strings.HasSuffix(lowerPath, ".js") &&
			!strings.HasSuffix(lowerPath, ".jsx") {
			continue
		}

		importedItems := slp033CollectImportedItems(f)
		slp033CollectImportedItemsFromFile(d, f.Path, importedItems)

		// Now check for usage of common types/hooks without imports across all hunks
		for _, h := range f.Hunks {
			for _, ln := range h.Lines {
				if ln.Kind != diff.LineAdd {
					continue
				}

				content := ln.Content

				// Check for React hooks usage without import
				for _, hook := range slp033ReactHooks {
					if containsWholeWord(content, hook) && !importedItems[hook] {
						if slp033HasImportedNamespaceReference(content, hook, importedItems) {
							continue
						}
						out = append(out, Finding{
							RuleID:   r.ID(),
							Severity: r.DefaultSeverity(),
							File:     f.Path,
							Line:     ln.NewLineNo,
							Message:  "React hook " + hook + " used without import - add import { " + hook + " } from 'react'",
							Snippet:  strings.TrimSpace(content),
						})
						break
					}
				}

				// Check for common types usage without import
				for _, typ := range slp033CommonTypes {
					if containsWholeWord(content, typ) && !importedItems[typ] {
						if slp033HasImportedNamespaceReference(content, typ, importedItems) {
							continue
						}
						if isTypeContext(content, typ) {
							out = append(out, Finding{
								RuleID:   r.ID(),
								Severity: r.DefaultSeverity(),
								File:     f.Path,
								Line:     ln.NewLineNo,
								Message:  "Type " + typ + " used without import - add import { " + typ + " } from 'react'",
								Snippet:  strings.TrimSpace(content),
							})
							break
						}
					}
				}
			}
		}
	}
	return out
}

func slp033CollectImportedItems(f diff.File) map[string]bool {
	importedItems := make(map[string]bool)
	for _, h := range f.Hunks {
		var visibleLines []string
		for _, ln := range h.Lines {
			if ln.Kind != diff.LineAdd && ln.Kind != diff.LineContext {
				continue
			}
			visibleLines = append(visibleLines, ln.Content)
		}
		slp033CollectImportedItemsFromLines(visibleLines, importedItems)
	}

	return importedItems
}

func slp033CollectImportedItemsFromFile(d *diff.Diff, relPath string, importedItems map[string]bool) {
	if d == nil || relPath == "" || importedItems == nil {
		return
	}

	content, ok := slp007FileContent(d, relPath)
	if !ok || content == "" {
		return
	}

	slp033CollectImportedItemsFromLines(strings.Split(content, "\n"), importedItems)
}

func slp033CollectImportedItemsFromLines(lines []string, importedItems map[string]bool) {
	if len(lines) == 0 || importedItems == nil {
		return
	}

	var currentImport strings.Builder
	collecting := false

	for _, line := range lines {
		content := strings.TrimSpace(line)
		if content == "" {
			continue
		}

		if !collecting {
			if !strings.HasPrefix(strings.ToLower(content), "import") {
				continue
			}
			currentImport.Reset()
			currentImport.WriteString(content)
			collecting = !strings.Contains(strings.ToLower(content), " from ")
			if !collecting {
				slp033RecordImportItems(currentImport.String(), importedItems)
			}
			continue
		}

		currentImport.WriteByte(' ')
		currentImport.WriteString(content)
		if strings.Contains(strings.ToLower(currentImport.String()), " from ") {
			slp033RecordImportItems(currentImport.String(), importedItems)
			currentImport.Reset()
			collecting = false
		}
	}
}

func slp033RecordImportItems(statement string, importedItems map[string]bool) {
	statement = strings.TrimSpace(statement)
	if statement == "" || importedItems == nil {
		return
	}

	if matches := slp033NamespaceImport.FindStringSubmatch(statement); len(matches) >= 2 {
		importedItems[matches[1]] = true
		return
	}

	lowerStatement := strings.ToLower(statement)
	fromIndex := strings.LastIndex(lowerStatement, " from ")
	if fromIndex == -1 {
		return
	}

	importPart := strings.TrimSpace(strings.TrimPrefix(statement[:fromIndex], "import"))
	importPart = strings.TrimSpace(strings.TrimPrefix(importPart, "type "))
	if importPart == "" || strings.HasPrefix(importPart, `"`) || strings.HasPrefix(importPart, `'`) {
		return
	}

	if braceStart := strings.Index(importPart, "{"); braceStart != -1 {
		defaultPart := strings.TrimSpace(strings.TrimSuffix(importPart[:braceStart], ","))
		slp033RecordImportSpecifier(defaultPart, importedItems)

		braceEnd := strings.LastIndex(importPart, "}")
		if braceEnd == -1 || braceEnd <= braceStart {
			return
		}
		for _, spec := range strings.Split(importPart[braceStart+1:braceEnd], ",") {
			slp033RecordImportSpecifier(spec, importedItems)
		}
		return
	}

	slp033RecordImportSpecifier(importPart, importedItems)
}

func slp033RecordImportSpecifier(spec string, importedItems map[string]bool) {
	spec = strings.TrimSpace(spec)
	if spec == "" || importedItems == nil {
		return
	}

	spec = strings.TrimPrefix(spec, "type ")
	spec = strings.TrimSpace(strings.Trim(spec, "{}* "))
	if spec == "" {
		return
	}

	if aliasIndex := strings.LastIndex(spec, " as "); aliasIndex != -1 {
		spec = strings.TrimSpace(spec[aliasIndex+4:])
	}
	if spec == "" {
		return
	}

	importedItems[spec] = true
}

func slp033HasImportedNamespaceReference(content, ident string, importedItems map[string]bool) bool {
	if content == "" || ident == "" || len(importedItems) == 0 {
		return false
	}

	for ns := range importedItems {
		if strings.Contains(content, ns+"."+ident) {
			return true
		}
	}

	return false
}

// isTypeContext checks if a type name appears in a type annotation context
func isTypeContext(content, typeName string) bool {
	typePatterns := []string{
		":" + typeName,
		": " + typeName,
		"as " + typeName,
		typeName + "<",
		"extends " + typeName,
		"type " + typeName,
		"interface " + typeName,
	}

	contentLower := strings.ToLower(content)
	typeNameLower := strings.ToLower(typeName)

	for _, pattern := range typePatterns {
		patternLower := strings.ToLower(pattern)
		if strings.Contains(contentLower, patternLower) {
			return true
		}
	}

	if strings.Contains(contentLower, "extends") || strings.Contains(contentLower, "implements") {
		words := strings.Fields(contentLower)
		for i, word := range words {
			if word == "extends" || word == "implements" {
				if i+1 < len(words) && words[i+1] == typeNameLower {
					return true
				}
			}
		}
	}

	return false
}

// containsWholeWord checks if the needle appears as a whole word in the haystack
func containsWholeWord(haystack, needle string) bool {
	haystackLower := strings.ToLower(haystack)
	needleLower := strings.ToLower(needle)

	parts := strings.FieldsFunc(haystackLower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})

	for _, part := range parts {
		if part == needleLower {
			return true
		}
	}

	return false
}
