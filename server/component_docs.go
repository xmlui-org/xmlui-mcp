package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// parentComponentMap maps variant component names to their parent component.
// When a component's docs are thin, we look up the parent for supplemental content.
var parentComponentMap = map[string]string{
	"CVStack":        "Stack",
	"CHStack":        "Stack",
	"VStack":         "Stack",
	"HStack":         "Stack",
	"ModalDialog":    "Dialog",
	"AlertDialog":    "Dialog",
	"DropdownButton": "Button",
	"IconButton":     "Button",
	"ToggleButton":   "Button",
	"NumberBox":      "TextBox",
	"PasswordBox":    "TextBox",
	"SearchBox":      "TextBox",
}

func NewComponentDocsTool(homeDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {

	tool := mcp.NewTool("xmlui_component_docs",
		mcp.WithDescription("Returns the Markdown documentation for a given XMLUI component. "+
			"By default returns the full reference page (can be 60KB+ for large components). "+
			"Use 'section' to scope to one part of the page (overview, properties/props, "+
			"events, methods/apis/exposed/exposed-methods, styling/theme-vars/theming), or "+
			"'member' to fetch just one named property/event/method's block (optionally "+
			"combined with 'section' to search within it)."),
		mcp.WithString("component",
			mcp.Required(),
			mcp.Description("Component name, e.g. 'Button', 'Avatar', or 'Stack/VStack'"),
		),
		mcp.WithString("section",
			mcp.Description("Optional: scope the result to one section of the page. One of "+
				"'overview', 'properties' (alias 'props'), 'events', 'methods' (aliases "+
				"'apis', 'exposed', 'exposed-methods'), or 'styling' (aliases 'theme-vars', "+
				"'theming'). Case-insensitive."),
		),
		mcp.WithString("member",
			mcp.Description("Optional: fetch just the block for one named property, event, "+
				"or exposed method (e.g. 'scrollToTop'). Case-insensitive. If 'section' is "+
				"also given, only that section is searched."),
		),
	)

	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		componentName, ok := req.Params.Arguments["component"].(string)
		if !ok || componentName == "" {
			return mcp.NewToolResultError("Missing or invalid 'component' parameter"), nil
		}

		// Normalize slash-qualified or extension-qualified args (e.g. "Stack/VStack",
		// "VStack.md") down to the bare component name the docs directory is keyed by (#29).
		componentName = normalizeComponentArg(componentName)
		if componentName == "" {
			return mcp.NewToolResultError("Missing or invalid 'component' parameter"), nil
		}

		paths := GetRepoPaths(homeDir)
		mdxPath := filepath.Join(homeDir, paths.ComponentDocs, componentName+".md")

		content, err := os.ReadFile(mdxPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return mcp.NewToolResultError(componentNotFoundMessage(homeDir, paths, componentName)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read %s under %s: %v", componentName, paths.ComponentDocs, errWithoutPath(err))), nil
		}

		contentStr := string(content)

		section, _ := req.Params.Arguments["section"].(string)
		member, _ := req.Params.Arguments["member"].(string)
		section = strings.TrimSpace(section)
		member = strings.Trim(strings.TrimSpace(member), "`")

		// No section/member requested: preserve today's full-page behavior
		// byte-for-byte (issue #26 requires the no-parameter default to be
		// unchanged).
		if section == "" && member == "" {
			// Supplement thin docs (Rec #3)
			if len(contentStr) < 500 {
				supplement := getComponentSupplement(homeDir, componentName)
				if supplement != "" {
					contentStr += "\n\n---\n## Additional Context\n\n" + supplement
				}
			}

			// Add source URL
			componentURL := ComponentURL(componentName)
			contentWithURL := contentStr + "\n\n**Source:** " + componentURL

			return mcp.NewToolResultText(contentWithURL), nil
		}

		componentURL := ComponentURL(componentName)
		lines := strings.Split(contentStr, "\n")

		if member != "" {
			rangeStart, rangeEnd := 0, len(lines)
			contextHeading := ""
			if section != "" {
				term, isOverview := resolveSectionTerm(section)
				if isOverview {
					return mcp.NewToolResultError(fmt.Sprintf(
						"The 'overview' section of %s has no '### ' members. Omit 'section' "+
							"to search the whole page, or use 'properties', 'events', "+
							"'methods', or 'styling'.", componentName)), nil
				}
				s, e, h, found := h2Range(lines, term)
				if !found {
					return mcp.NewToolResultError(sectionNotFoundMessage(componentName, section, lines)), nil
				}
				rangeStart, rangeEnd, contextHeading = s, e, h
			}

			s, e, _, found := h3Range(lines, rangeStart, rangeEnd, member)
			if !found {
				return mcp.NewToolResultError(memberNotFoundMessage(componentName, member, lines, rangeStart, rangeEnd)), nil
			}
			if contextHeading == "" {
				contextHeading = enclosingH2At(lines, s)
			}

			body := strings.TrimRight(strings.Join(lines[s:e], "\n"), "\n")
			var b strings.Builder
			b.WriteString("From " + componentName)
			if contextHeading != "" {
				b.WriteString(" › " + contextHeading)
			}
			b.WriteString(":\n\n")
			b.WriteString(body)
			b.WriteString("\n\n**Source:** " + componentURL)
			return mcp.NewToolResultText(b.String()), nil
		}

		// section only (member == "")
		term, isOverview := resolveSectionTerm(section)
		if isOverview {
			body := extractOverview(lines)
			result := body + "\n\n**Source:** " + componentURL
			return mcp.NewToolResultText(result), nil
		}
		s, e, _, found := h2Range(lines, term)
		if !found {
			return mcp.NewToolResultError(sectionNotFoundMessage(componentName, section, lines)), nil
		}
		body := strings.TrimRight(strings.Join(lines[s:e], "\n"), "\n")
		result := body + "\n\n**Source:** " + componentURL
		return mcp.NewToolResultText(result), nil
	}

	return tool, handler
}

// getComponentSupplement finds additional documentation for thin component docs.
func getComponentSupplement(homeDir string, componentName string) string {
	var supplement strings.Builder
	maxSupplement := 2000
	paths := GetRepoPaths(homeDir)

	// Extract the bare component name (handle paths like "Stack/VStack")
	baseName := filepath.Base(componentName)

	// Check parent component map
	if parent, ok := parentComponentMap[baseName]; ok {
		parentPath := filepath.Join(homeDir, paths.ComponentDocs, parent+".md")
		parentContent, err := os.ReadFile(parentPath)
		if err == nil {
			// Extract Properties and Events sections from parent
			extracted := extractSections(string(parentContent), []string{"Properties", "Events", "Props"})
			if extracted != "" {
				supplement.WriteString(fmt.Sprintf("*From parent component %s:*\n\n", parent))
				supplement.WriteString(extracted)
			}
		}
	}

	// Check for source .md or .tsx in xmlui/src/components/<Name>/
	if supplement.Len() < maxSupplement {
		srcDir := filepath.Join(homeDir, paths.ComponentSource, baseName)
		if entries, err := os.ReadDir(srcDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".tsx") {
					srcPath := filepath.Join(srcDir, name)
					data, err := os.ReadFile(srcPath)
					if err == nil {
						content := string(data)
						if len(content) > maxSupplement-supplement.Len() {
							content = content[:maxSupplement-supplement.Len()]
						}
						supplement.WriteString(fmt.Sprintf("\n*From source %s:*\n\n", name))
						supplement.WriteString(content)
					}
					break // only take first matching file
				}
			}
		}
	}

	result := supplement.String()
	if len(result) > maxSupplement {
		result = result[:maxSupplement]
	}
	return result
}

// extractSections extracts named markdown sections (## heading) from content.
func extractSections(content string, sectionNames []string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	capturing := false

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimPrefix(line, "## ")
			heading = strings.TrimSpace(heading)
			matched := false
			for _, name := range sectionNames {
				if strings.EqualFold(heading, name) {
					matched = true
					break
				}
			}
			if matched {
				capturing = true
				result.WriteString(line + "\n")
				continue
			} else if capturing {
				// Hit a new section, stop capturing
				break
			}
		}
		if capturing {
			result.WriteString(line + "\n")
		}
	}
	return result.String()
}

// normalizeComponentArg trims whitespace and collapses slash-qualified or
// extension-qualified component arguments (e.g. "Stack/VStack", "VStack.md")
// down to the bare name the flat component docs directory is keyed by (#29).
func normalizeComponentArg(raw string) string {
	name := strings.TrimSpace(raw)
	name = strings.ReplaceAll(name, "\\", "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSuffix(name, ".md")
	return strings.TrimSpace(name)
}

// componentNotFoundMessage builds an actionable miss message: it names the
// corpus-relative directory searched (never the absolute host path) and
// offers near-match suggestions plus the discovery tool (#29).
func componentNotFoundMessage(homeDir string, paths *RepoPaths, componentName string) string {
	msg := fmt.Sprintf("Component %q not found under %s.", componentName, paths.ComponentDocs)
	if suggestions := suggestComponentDocNames(homeDir, paths, componentName); len(suggestions) > 0 {
		msg += fmt.Sprintf(" Did you mean: %s?", strings.Join(suggestions, ", "))
	}
	msg += " Call xmlui_list_components to see all available component names."
	return msg
}

// suggestComponentDocNames returns up to 3 near matches for componentName
// drawn from the component docs directory the lookup itself reads (#29).
// Ranking is case-insensitive exact match first, then prefix match, then
// substring match; _-prefixed index files are skipped, matching the
// existing xmlui_list_components filtering convention.
func suggestComponentDocNames(homeDir string, paths *RepoPaths, componentName string) []string {
	dir := filepath.Join(homeDir, paths.ComponentDocs)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileName := entry.Name()
		if strings.HasPrefix(fileName, "_") || !strings.HasSuffix(fileName, ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(fileName, ".md"))
	}

	target := strings.ToLower(componentName)
	var exact, prefix, substr []string
	for _, name := range names {
		lower := strings.ToLower(name)
		switch {
		case lower == target:
			exact = append(exact, name)
		case strings.HasPrefix(lower, target):
			prefix = append(prefix, name)
		case strings.Contains(lower, target):
			substr = append(substr, name)
		}
	}
	sort.Strings(exact)
	sort.Strings(prefix)
	sort.Strings(substr)

	suggestions := append(append(exact, prefix...), substr...)
	if len(suggestions) > 3 {
		suggestions = suggestions[:3]
	}
	return suggestions
}

// toRepoRelative strips the homeDir prefix from an absolute filesystem path
// so error messages never leak the host's absolute cache/checkout path
// (e.g. /Users/<name>/Library/Caches/xmlui/...) into model context (#29).
// Shared by component_docs.go and read_file.go.
func toRepoRelative(homeDir, path string) string {
	rel, err := filepath.Rel(homeDir, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// errWithoutPath extracts the underlying OS error from a *fs.PathError
// without its embedded absolute path, so wrapping "%v" around a read/stat
// error can't leak the host filesystem layout (#29). Shared by
// component_docs.go and read_file.go.
func errWithoutPath(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}

// --- section/member granularity (#26) ---

// resolveSectionTerm canonicalizes a requested section name (case-insensitive,
// aliases included) down to the substring used to match it against the page's
// actual "## " heading text. "overview" has no heading of its own (it's the
// page intro before the first "## " heading), so it's reported separately via
// isOverview rather than as a search term. Unrecognized values pass through
// unchanged so they can still be substring-matched against real headings (and
// so a miss can list what headings actually exist).
func resolveSectionTerm(section string) (term string, isOverview bool) {
	key := strings.ToLower(strings.TrimSpace(section))
	switch key {
	case "overview":
		return "", true
	case "properties", "props":
		return "properties", false
	case "events":
		return "events", false
	case "methods", "apis", "exposed", "exposed-methods":
		return "methods", false
	case "styling", "theme-vars", "theming":
		return "styling", false
	default:
		return key, false
	}
}

// stripHeadingAnchor removes a trailing " [#anchor]" fragment from a heading
// line's text (e.g. "Properties [#properties]" -> "Properties").
func stripHeadingAnchor(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.LastIndex(s, "[#"); idx >= 0 && strings.HasSuffix(s, "]") {
		s = strings.TrimSpace(s[:idx])
	}
	return s
}

// stripMemberName reduces a "### " member heading's text to its bare name,
// stripping both the anchor fragment and surrounding backticks (e.g.
// "`scrollToTop` [#scrolltotop]" -> "scrollToTop").
func stripMemberName(s string) string {
	return strings.Trim(stripHeadingAnchor(s), "`")
}

// h2Heading reports whether line is a "## " heading, returning its
// anchor-stripped text.
func h2Heading(line string) (string, bool) {
	if !strings.HasPrefix(line, "## ") {
		return "", false
	}
	return stripHeadingAnchor(strings.TrimPrefix(line, "## ")), true
}

// h2Range finds the first "## " heading whose anchor-stripped text contains
// term as a case-insensitive substring, and returns the line range [start,end)
// spanning that heading through the line before the next "## " heading (or
// EOF), plus the matched heading text.
func h2Range(lines []string, term string) (start, end int, heading string, ok bool) {
	lowerTerm := strings.ToLower(term)
	for i, line := range lines {
		h, isH2 := h2Heading(line)
		if !isH2 {
			continue
		}
		if strings.Contains(strings.ToLower(h), lowerTerm) {
			end = len(lines)
			for j := i + 1; j < len(lines); j++ {
				if _, isH2b := h2Heading(lines[j]); isH2b {
					end = j
					break
				}
			}
			return i, end, h, true
		}
	}
	return 0, 0, "", false
}

// h3Range searches lines[rangeStart:rangeEnd] for a "### " member heading
// whose stripped name case-insensitively equals memberName, returning the
// absolute line range [start,end) spanning that heading through the line
// before the next "### " or "## " heading (or the end of the search range),
// plus the matched member name.
func h3Range(lines []string, rangeStart, rangeEnd int, memberName string) (start, end int, name string, ok bool) {
	for i := rangeStart; i < rangeEnd; i++ {
		if !strings.HasPrefix(lines[i], "### ") {
			continue
		}
		mn := stripMemberName(strings.TrimPrefix(lines[i], "### "))
		if strings.EqualFold(mn, memberName) {
			end = rangeEnd
			for j := i + 1; j < rangeEnd; j++ {
				if strings.HasPrefix(lines[j], "### ") || strings.HasPrefix(lines[j], "## ") {
					end = j
					break
				}
			}
			return i, end, mn, true
		}
	}
	return 0, 0, "", false
}

// enclosingH2At returns the anchor-stripped text of the nearest "## " heading
// at or before line index idx, or "" if idx precedes any "## " heading.
func enclosingH2At(lines []string, idx int) string {
	heading := ""
	for i := 0; i < idx && i < len(lines); i++ {
		if h, ok := h2Heading(lines[i]); ok {
			heading = h
		}
	}
	return heading
}

// extractOverview returns the page intro: everything from the top through
// the line before the first "## " heading.
func extractOverview(lines []string) string {
	var out []string
	for _, line := range lines {
		if _, ok := h2Heading(line); ok {
			break
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// listH2Headings returns the anchor-stripped text of every "## " heading in
// document order.
func listH2Headings(lines []string) []string {
	var out []string
	for _, line := range lines {
		if h, ok := h2Heading(line); ok {
			out = append(out, h)
		}
	}
	return out
}

// listH3Names returns the stripped names of every "### " member heading in
// lines[rangeStart:rangeEnd], in document order.
func listH3Names(lines []string, rangeStart, rangeEnd int) []string {
	var out []string
	for i := rangeStart; i < rangeEnd && i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "### ") {
			out = append(out, stripMemberName(strings.TrimPrefix(lines[i], "### ")))
		}
	}
	return out
}

// sectionNotFoundMessage builds an actionable miss message naming the
// headings that actually exist on the page, in #29's style (no host paths).
func sectionNotFoundMessage(componentName, section string, lines []string) string {
	headings := listH2Headings(lines)
	msg := fmt.Sprintf("Section %q not found in the %s docs.", section, componentName)
	if len(headings) == 0 {
		return msg + " This page has no '## ' sections."
	}
	return msg + fmt.Sprintf(" Available sections: %s.", strings.Join(headings, ", "))
}

// memberNotFoundMessage builds an actionable miss message listing the member
// names that actually exist (bounded to ~20, noting "and N more"), in #29's
// style (no host paths).
func memberNotFoundMessage(componentName, member string, lines []string, rangeStart, rangeEnd int) string {
	names := listH3Names(lines, rangeStart, rangeEnd)
	msg := fmt.Sprintf("Member %q not found in the %s docs.", member, componentName)
	if len(names) == 0 {
		return msg + " This page (or section) has no '### ' members."
	}
	const limit = 20
	shown := names
	suffix := ""
	if len(shown) > limit {
		suffix = fmt.Sprintf(" and %d more", len(shown)-limit)
		shown = shown[:limit]
	}
	return msg + fmt.Sprintf(" Available members: %s%s.", strings.Join(shown, ", "), suffix)
}
