package server

import "strings"

// TopicEntry represents a curated topic that maps common query concepts
// to the best documentation resources.
type TopicEntry struct {
	Name          string   // Human-readable topic name
	TriggerTerms []string // Terms that trigger this topic match
	CanonicalDocs []string // Relative doc paths that should get a score bonus
	URLs          []string // Direct documentation URLs
	Description   string   // Brief description of the topic
}

// topicIndex is the static list of curated topics.
var topicIndex = []TopicEntry{
	{
		Name:         "Event Callbacks",
		TriggerTerms: []string{"event", "callback", "handler", "onclick", "onchange", "onselect", "onsubmit", "onclose"},
		CanonicalDocs: []string{
			"docs/content/pages/event-handling",
			"docs/content/components/Button",
		},
		URLs: []string{},
		Description: "XMLUI event handling: onClick, onChange, and other event callbacks on components",
	},
	{
		Name:         "Script and Code-Behind",
		TriggerTerms: []string{"script", "code-behind", "codebehind", "javascript", "function", "window"},
		CanonicalDocs: []string{
			"docs/content/pages/code-behind",
		},
		URLs: []string{
			"https://docs.xmlui.org/guides/scripting",
		},
		Description: "Using JavaScript code-behind files and script functions in XMLUI applications",
	},
	{
		Name:         "Conditional Rendering",
		TriggerTerms: []string{"conditional", "condition", "if", "when", "visible", "show", "hide", "toggle"},
		CanonicalDocs: []string{
			"docs/content/pages/conditional-rendering",
		},
		URLs: []string{},
		Description: "Showing or hiding components based on conditions using when/visible attributes",
	},
	{
		Name:         "Stack Layout Family",
		TriggerTerms: []string{"stack", "vstack", "hstack", "layout", "vertical", "horizontal", "cvstack", "chstack"},
		CanonicalDocs: []string{
			"docs/content/components/Stack",
		},
		URLs: []string{
			"https://docs.xmlui.org/components/Stack",
		},
		Description: "Stack-based layout components: VStack, HStack, CVStack, CHStack for arranging child elements",
	},
	{
		Name:         "Theming and Styling",
		TriggerTerms: []string{"theme", "theming", "style", "color", "dark", "light", "custom", "css", "variable"},
		CanonicalDocs: []string{
			"docs/content/pages/theming",
		},
		URLs: []string{},
		Description: "Customizing the look and feel of XMLUI apps through theme variables",
	},
	{
		Name:         "Data Binding",
		TriggerTerms: []string{"binding", "bind", "data", "state", "variable", "appstate", "var", "value"},
		CanonicalDocs: []string{
			"docs/content/pages/data-binding",
		},
		URLs: []string{},
		Description: "Binding data to components using variables and AppState",
	},
}

// matchTopics returns all topics whose trigger terms overlap with the query tokens.
// A topic matches if at least one trigger term is found in the query tokens.
func matchTopics(queryTokens []string) []TopicEntry {
	if len(queryTokens) == 0 {
		return nil
	}

	// Build a set of query tokens for fast lookup
	tokenSet := make(map[string]bool, len(queryTokens))
	for _, t := range queryTokens {
		tokenSet[strings.ToLower(t)] = true
	}

	var matches []TopicEntry
	for _, topic := range topicIndex {
		matchCount := 0
		for _, trigger := range topic.TriggerTerms {
			if tokenSet[strings.ToLower(trigger)] {
				matchCount++
			}
		}
		// Require at least one trigger match
		if matchCount > 0 {
			matches = append(matches, topic)
		}
	}
	return matches
}
