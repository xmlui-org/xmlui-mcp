package server

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// PatternEntry is a curated code pattern with trigger terms.
type PatternEntry struct {
	Name     string   // Pattern name
	Triggers []string // Query terms that match this pattern
	Code     string   // Example code snippet
	URL      string   // Documentation URL
}

var patternIndex = []PatternEntry{
	{
		Name:     "Conditional Rendering",
		Triggers: []string{"conditional", "rendering", "when", "visible", "show", "hide"},
		Code: `<!-- Show/hide with when attribute -->
<Text when="{showGreeting}" value="Hello!" />

<!-- Toggle visibility -->
<Button label="Toggle" onClick="showGreeting = !showGreeting" />`,
		URL: "https://docs.xmlui.org/conditional-rendering",
	},
	{
		Name:     "List Rendering",
		Triggers: []string{"list", "loop", "foreach", "repeat", "items", "array", "map"},
		Code: `<!-- Render a list of items -->
<List data="{items}">
  <ListItem>
    <Text value="{$item.name}" />
  </ListItem>
</List>`,
		URL: "https://docs.xmlui.org/components/List",
	},
	{
		Name:     "Form with Validation",
		Triggers: []string{"form", "validation", "validate", "submit", "input"},
		Code: `<Form onSubmit="handleSubmit">
  <FormItem label="Name">
    <TextBox id="name" required="true" />
  </FormItem>
  <FormItem label="Email">
    <TextBox id="email" required="true" />
  </FormItem>
  <Button type="submit" label="Submit" />
</Form>`,
		URL: "https://docs.xmlui.org/components/Form",
	},
	{
		Name:     "Navigation / Routing",
		Triggers: []string{"navigation", "navigate", "route", "routing", "page", "pages", "link"},
		Code: `<!-- Page navigation -->
<NavPanel>
  <NavLink to="/home" label="Home" />
  <NavLink to="/about" label="About" />
</NavPanel>

<!-- Pages define routes -->
<Pages>
  <Page id="home" url="/home">
    <Text value="Home page" />
  </Page>
</Pages>`,
		URL: "https://docs.xmlui.org/components/Pages",
	},
	{
		Name:     "Data Binding / State",
		Triggers: []string{"binding", "bind", "state", "variable", "appstate", "reactive"},
		Code: `<!-- Declare state variable -->
<var name="count" value="0" />

<!-- Bind to component -->
<Text value="Count: {count}" />
<Button label="Increment" onClick="count = count + 1" />`,
		URL: "https://docs.xmlui.org/data-binding",
	},
	{
		Name:     "Dialog / Modal",
		Triggers: []string{"dialog", "modal", "popup", "overlay", "confirm", "alert"},
		Code: `<Button label="Open Dialog" onClick="showDialog = true" />

<Dialog isOpen="{showDialog}" title="Confirm" onClose="showDialog = false">
  <Text value="Are you sure?" />
  <Button label="Yes" onClick="handleConfirm" />
  <Button label="Cancel" onClick="showDialog = false" />
</Dialog>`,
		URL: "https://docs.xmlui.org/components/Dialog",
	},
	{
		Name:     "API / Data Fetching",
		Triggers: []string{"api", "fetch", "data", "rest", "endpoint", "http", "request"},
		Code: `<!-- Define an API call -->
<DataSource id="users" url="/api/users" method="GET" />

<!-- Use the data -->
<List data="{users.data}">
  <ListItem>
    <Text value="{$item.name}" />
  </ListItem>
</List>`,
		URL: "https://docs.xmlui.org/data-binding",
	},
	{
		Name:     "Theming / Dark Mode",
		Triggers: []string{"theme", "theming", "dark", "light", "mode", "color", "custom"},
		Code: `<!-- Apply a theme -->
<App theme="dark">
  <Text value="Dark mode app" />
</App>

<!-- Custom theme variables in theme.json -->
{
  "colors": {
    "primary": "#3B82F6",
    "background": "#1F2937"
  }
}`,
		URL: "https://docs.xmlui.org/theming",
	},
	{
		Name:     "Table / DataGrid",
		Triggers: []string{"table", "datagrid", "grid", "column", "row", "sort", "pagination"},
		Code: `<Table data="{users}">
  <Column field="name" header="Name" />
  <Column field="email" header="Email" />
  <Column field="role" header="Role" />
</Table>`,
		URL: "https://docs.xmlui.org/components/Table",
	},
	{
		Name:     "Tabs / Tab Navigation",
		Triggers: []string{"tab", "tabs", "tabbed", "tabpanel", "tabstrip"},
		Code: `<TabStrip>
  <Tab label="Overview">
    <Text value="Overview content" />
  </Tab>
  <Tab label="Details">
    <Text value="Details content" />
  </Tab>
</TabStrip>`,
		URL: "https://docs.xmlui.org/components/TabStrip",
	},
	{
		Name:     "Event Handling",
		Triggers: []string{"event", "handler", "callback", "onclick", "onchange"},
		Code: `<!-- Inline handler -->
<Button label="Click me" onClick="handleClick" />

<!-- In code-behind (index.js): -->
window.handleClick = function() {
  console.log("Button clicked");
};`,
		URL: "https://docs.xmlui.org/event-handling",
	},
	{
		Name:     "Code-Behind Pattern",
		Triggers: []string{"code-behind", "codebehind", "script", "javascript", "function"},
		Code: `<!-- In your .xmlui file: -->
<Button label="Calculate" onClick="doCalc" />
<Text value="{result}" />

<!-- In index.js (code-behind): -->
window.doCalc = function() {
  window.result = 42;
};`,
		URL: "https://docs.xmlui.org/code-behind",
	},
	{
		Name:     "Stack Layout",
		Triggers: []string{"stack", "vstack", "hstack", "layout", "vertical", "horizontal"},
		Code: `<!-- Vertical stack (default) -->
<VStack>
  <Text value="Top" />
  <Text value="Bottom" />
</VStack>

<!-- Horizontal stack -->
<HStack>
  <Text value="Left" />
  <Text value="Right" />
</HStack>`,
		URL: "https://docs.xmlui.org/components/Stack",
	},
}

// NewPatternTool creates the xmlui_pattern tool.
func NewPatternTool(homeDir string) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("xmlui_pattern",
		mcp.WithDescription("Returns curated XMLUI code patterns and cookbook snippets for common tasks. Use this to quickly find how to implement common UI patterns."),
		mcp.WithString("query", mcp.Required(), mcp.Description("What pattern to find, e.g. 'conditional rendering', 'form validation', 'dark mode'")),
	)

	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	}

	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, ok := req.Params.Arguments["query"].(string)
		if !ok || strings.TrimSpace(raw) == "" {
			return mcp.NewToolResultError("Missing or invalid 'query' parameter"), nil
		}
		query := strings.TrimSpace(raw)

		// Tokenize query
		tokens := strings.Fields(strings.ToLower(query))
		tokenSet := make(map[string]bool)
		for _, t := range tokens {
			tokenSet[t] = true
		}

		// Find matching patterns
		type matchedPattern struct {
			pattern    PatternEntry
			matchCount int
		}
		var matches []matchedPattern

		for _, pattern := range patternIndex {
			count := 0
			for _, trigger := range pattern.Triggers {
				if tokenSet[strings.ToLower(trigger)] {
					count++
				}
				// Also check substring containment
				for _, token := range tokens {
					if strings.Contains(strings.ToLower(trigger), token) || strings.Contains(token, strings.ToLower(trigger)) {
						count++
					}
				}
			}
			if count > 0 {
				matches = append(matches, matchedPattern{pattern: pattern, matchCount: count})
			}
		}

		// Sort by match count descending
		for i := 0; i < len(matches); i++ {
			for j := i + 1; j < len(matches); j++ {
				if matches[j].matchCount > matches[i].matchCount {
					matches[i], matches[j] = matches[j], matches[i]
				}
			}
		}

		if len(matches) > 0 {
			var out strings.Builder
			// Show top 3 matches
			limit := 3
			if limit > len(matches) {
				limit = len(matches)
			}
			for i := 0; i < limit; i++ {
				p := matches[i].pattern
				if i > 0 {
					out.WriteString("\n---\n\n")
				}
				fmt.Fprintf(&out, "## %s\n\n", p.Name)
				fmt.Fprintf(&out, "```xml\n%s\n```\n\n", p.Code)
				fmt.Fprintf(&out, "**Documentation:** %s\n", p.URL)
			}
			return mcp.NewToolResultText(out.String()), nil
		}

		// Fallback: search howto directory
		pagesDir := DetectPagesDir(homeDir)
		roots := []string{
			filepath.Join(homeDir, pagesDir, "howto"),
		}

		cfg := MediatorConfig{
			Roots:                 roots,
			SectionKeys:           []string{"howtos"},
			PreferSections:        []string{"howtos"},
			MaxResults:            20,
			MaxFileResults:        5,
			MaxSnippetsPerFile:    3,
			FileExtensions:        []string{".md", ".mdx"},
			Stopwords:             DefaultStopwords(),
			Synonyms:              DefaultSynonyms(),
			Classifier:            HowtoClassifier(homeDir),
			EnableFilenameMatches: true,
		}

		human, _, err := ExecuteMediatedSearch(homeDir, cfg, query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("No curated pattern found. Searching howto articles:\n\n" + human), nil
	}

	return tool, handler
}
