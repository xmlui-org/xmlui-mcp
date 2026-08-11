package server

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func resetTopicIndexForTest(t *testing.T) {
	t.Helper()
	previousIndex := topicIndex
	previousHome := topicHomeDir
	topicIndex = nil
	topicHomeDir = ""
	topicIndexOnce = sync.Once{}
	t.Cleanup(func() {
		topicIndex = previousIndex
		topicHomeDir = previousHome
		topicIndexOnce = sync.Once{}
	})
}

func howtoMediatorConfig(howtoDir string) MediatorConfig {
	return MediatorConfig{
		Roots:          []string{howtoDir},
		SectionKeys:    []string{"howtos"},
		FileExtensions: []string{".md"},
		Stopwords:      DefaultStopwords(),
		Synonyms:       DefaultSynonyms(),
		Classifier: func(rel string, absPath string) string {
			return "howtos"
		},
		EnableFilenameMatches: true,
	}
}

func writeHowtoFixture(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The #10 reproducer shape: many files each containing a minority of the
// query's terms. Volume is present, relevance is not — confidence must not
// read "high".
func TestConfidenceNotHighOnTokenScatter(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeHowtoFixture(t, howtoDir, "use-modal-dialog-onclose.md",
		"# Use a modal dialog onClose\nHandle the close button of a modal dialog.\nThe close event fires when dismissed.\n")
	writeHowtoFixture(t, howtoDir, "submit-form-custom-button.md",
		"# Submit a form from a custom button\nWire a button to submit.\nThe button triggers submission.\n")
	writeHowtoFixture(t, howtoDir, "announce-status-changes.md",
		"# Announce status changes\nShow a notification when status changes.\nA notification informs the user.\n")

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"dismissible notification banner with a close button")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Confidence == "high" {
		t.Fatalf("expected confidence below high for token scatter, got %q", summary.Confidence)
	}
}

func TestConfidenceHighOnStrongSpecificMatch(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeHowtoFixture(t, howtoDir, "sync-tilegrid-selection-across-grids.md",
		"# Sync tilegrid selection across grids\nKeep tilegrid selection in sync across grids.\nSync the selection whenever either tilegrid changes.\n")
	writeHowtoFixture(t, howtoDir, "unrelated-topic.md",
		"# Unrelated topic\nNothing relevant here.\n")

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"sync tilegrid selection across grids")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Confidence != "high" {
		t.Fatalf("expected high confidence for strong specific match, got %q", summary.Confidence)
	}
}

func TestConfidenceLowOnNoMatches(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHowtoFixture(t, howtoDir, "something.md", "# Something\nEntirely different content.\n")

	human, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"zebra quantum flux capacitor")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Confidence != "low" {
		t.Fatalf("expected low confidence for no matches, got %q", summary.Confidence)
	}
	if !strings.Contains(human, "No matches found.") {
		t.Fatalf("expected no-matches output, got: %s", human)
	}
}

func TestMatchTopicsRequiresMeaningfulOverlap(t *testing.T) {
	resetTopicIndexForTest(t)
	topicIndex = []TopicEntry{
		{Name: "Modal Dialogs", TriggerTerms: []string{"modal", "dialogs"}},
		{Name: "Forms", TriggerTerms: []string{"forms"}},
	}

	// Multi-token query overlapping only one trigger of a two-trigger topic:
	// no match.
	if got := matchTopics([]string{"modal", "banner", "notification"}); len(got) != 0 {
		t.Fatalf("expected no topics for single-trigger overlap on multi-token query, got %d", len(got))
	}

	// Multi-token query overlapping both triggers: match.
	got := matchTopics([]string{"modal", "dialogs", "close"})
	if len(got) != 1 || got[0].Name != "Modal Dialogs" {
		t.Fatalf("expected Modal Dialogs match, got %+v", got)
	}

	// Single-token query still matches on one overlap.
	got = matchTopics([]string{"forms"})
	if len(got) != 1 || got[0].Name != "Forms" {
		t.Fatalf("expected Forms match for single-token query, got %+v", got)
	}

	// A single-trigger topic is reachable from a multi-token query: the
	// required overlap is capped at the topic's own trigger count.
	got = matchTopics([]string{"forms", "validation"})
	if len(got) != 1 || got[0].Name != "Forms" {
		t.Fatalf("expected Forms match for multi-token query, got %+v", got)
	}
}

func TestCorroboratedTopicURLs(t *testing.T) {
	validateAll := func(u string) string { return u }
	ranked := []*scoredFile{
		{RelPath: "docs/pages/forms.md", Score: 3.0},
		{RelPath: "docs/pages/other.md", Score: 1.0},
	}
	topics := []TopicEntry{
		{
			Name:          "Forms",
			CanonicalDocs: []string{"pages/forms.md"},
			URLs:          []string{"/docs/forms", "/docs/forms", "/docs/forms-advanced"},
		},
		{
			Name:          "Uncorroborated",
			CanonicalDocs: []string{"pages/absent.md"},
			URLs:          []string{"/docs/absent"},
		},
	}

	got := corroboratedTopicURLs(topics, ranked, validateAll, 5)
	if len(got) != 2 {
		t.Fatalf("expected 2 deduplicated corroborated URLs, got %+v", got)
	}
	for _, doc := range got {
		if doc.Title != "Forms" {
			t.Fatalf("unexpected topic in output: %+v", doc)
		}
	}

	// Cap applies.
	if got := corroboratedTopicURLs(topics, ranked, validateAll, 1); len(got) != 1 {
		t.Fatalf("expected cap of 1, got %d", len(got))
	}

	// Validation failures are excluded.
	rejectAll := func(u string) string { return "" }
	if got := corroboratedTopicURLs(topics, ranked, rejectAll, 5); len(got) != 0 {
		t.Fatalf("expected no URLs when validation rejects, got %+v", got)
	}
}

// The default human output must not carry a flat URL dump when no topic is
// corroborated by the ranked results.
func TestNoFlatURLDumpWithoutCorroboration(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHowtoFixture(t, howtoDir, "use-buttons.md", "# Use buttons\nA button howto.\n")

	// Seed a topic index whose topics trigger on the query but whose canonical
	// docs are not among the ranked results.
	topicIndex = []TopicEntry{
		{
			Name:          "Button Basics",
			TriggerTerms:  []string{"button", "basics"},
			CanonicalDocs: []string{"pages/components/OtherComponent.md"},
			URLs:          []string{"/docs/button-basics"},
		},
	}

	human, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"button basics")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(human, "Documentation URLs:") {
		t.Fatalf("expected no Documentation URLs block, got: %s", human)
	}
	if summary.AgentGuidance != nil && len(summary.AgentGuidance.DocumentationURLs) != 0 {
		t.Fatalf("expected no documentation_urls in summary, got %+v", summary.AgentGuidance.DocumentationURLs)
	}
}
