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

// The #11 reproducer shape: the top hit covers the query's generic tokens
// while a lower-ranked candidate holds the distinctive term. Aggregate
// coverage alone must not read "high".
func TestConfidenceNotHighWhenDistinctiveTermUnanswered(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeHowtoFixture(t, howtoDir, "disable-save-button-validation.md",
		"# Disable the save button during validation\nDisable the save button while the form validates.\nThe form field state drives the save button.\n")
	writeHowtoFixture(t, howtoDir, "other-notes.md",
		"# Other notes\nTrack changed form field values.\n")

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"disable save button form field changed")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Confidence == "high" {
		t.Fatalf("expected confidence below high when distinctive term is unanswered, got %q", summary.Confidence)
	}
}

func TestConfidenceHighWhenDistinctiveTermCovered(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeHowtoFixture(t, howtoDir, "disable-save-button-validation.md",
		"# Disable the save button during validation\nDisable the save button while the form validates.\n")
	writeHowtoFixture(t, howtoDir, "other-notes.md",
		"# Other notes\nTrack changed form field values.\n")

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"disable save button validation")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Confidence != "high" {
		t.Fatalf("expected high confidence when the distinctive term is covered, got %q", summary.Confidence)
	}
}

func TestTitleMatchStampedOnResultItems(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeHowtoFixture(t, howtoDir, "sync-selection.md",
		"# Sync selection\nKeep the selection in sync.\n")
	writeHowtoFixture(t, howtoDir, "unrelated-name.md",
		"# Unrelated name\nAlso about selection sync details.\n")

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"sync selection")
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]bool{}
	for _, items := range summary.Sections {
		for _, item := range items {
			if item.TitleMatch {
				byPath[item.Path] = true
			}
		}
	}
	if len(byPath) != 1 {
		t.Fatalf("expected exactly one title-matched path, got %#v", byPath)
	}
	for path := range byPath {
		if !strings.Contains(path, "sync-selection") {
			t.Fatalf("title match stamped on wrong path: %s", path)
		}
	}
}

// The three #12 gap classes, from the salience summary's title vs content
// coverage of the query's distinctive terms.
func TestSalienceSummaryClassifiesGapKinds(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// "clipboard" is the distinctive term. One doc answers it in content but
	// not in its title; several docs title-match only scaffolding tokens.
	writeHowtoFixture(t, howtoDir, "submit-form-custom-button.md",
		"# Submit a form from a custom button\nWire a button to submit.\nCopy the form data text on submit.\n")
	writeHowtoFixture(t, howtoDir, "style-text-variants.md",
		"# Style text variants\nStyle button text with variants.\n")
	writeHowtoFixture(t, howtoDir, "copy-values-between-fields.md",
		"# Copy values between fields\nCopy button text to the clipboard on click.\n")

	// Discoverability shape: content answers "clipboard", no title does.
	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"copy text clipboard button")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Salience == nil {
		t.Fatal("expected salience summary")
	}
	if strings.Join(summary.Salience.Terms, ",") != "clipboard" {
		t.Fatalf("salient terms = %v, want [clipboard]", summary.Salience.Terms)
	}
	if summary.Salience.TitleMatchCount != 0 {
		t.Fatalf("salient title matches = %d, want 0", summary.Salience.TitleMatchCount)
	}
	if summary.Salience.ContentMatchCount != 1 {
		t.Fatalf("salient content matches = %d, want 1", summary.Salience.ContentMatchCount)
	}

	// On-topic-titled shape: a doc titled on the distinctive term.
	writeHowtoFixture(t, howtoDir, "copy-to-clipboard.md",
		"# Copy to clipboard\nUse the clipboard API from a button.\n")
	_, summary, err = ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"copy text clipboard button")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Salience == nil || summary.Salience.TitleMatchCount < 1 {
		t.Fatalf("expected salient title match after adding titled doc, got %+v", summary.Salience)
	}

	// Content-gap shape: distinctive concept absent everywhere. The query
	// still matches scaffolding tokens, so results exist, but no achievable
	// term is distinctive of the intent and title/content counts stay honest.
	_, summary, err = ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"microphone capture button")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Salience == nil {
		t.Fatal("expected salience summary")
	}
	// Absent intent terms belong in the band (#14): their absence is the
	// content-gap tell. (Generic-term exclusion needs >=3 candidates and is
	// covered by TestSalientAggregatesConsistentWithTermCoverage.)
	band := strings.Join(summary.Salience.Terms, ",")
	if !strings.Contains(band, "microphone") || !strings.Contains(band, "capture") {
		t.Fatalf("salient_terms = %v, want microphone and capture in band", summary.Salience.Terms)
	}
	unanswered := strings.Join(summary.Salience.UnansweredTerms, ",")
	if !strings.Contains(unanswered, "microphone") || !strings.Contains(unanswered, "capture") {
		t.Fatalf("unanswered_terms = %v, want microphone and capture", summary.Salience.UnansweredTerms)
	}
}

// Playground fence lines are markup: their tokens must not count as content
// coverage (#13's copy-inflation case).
func TestFenceBoilerplateExcludedFromCoverage(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Fence carries "copy"; body does not. The file is a candidate via
	// "clipboard" in its body.
	writeHowtoFixture(t, howtoDir, "paste-images.md",
		"# Paste images\n```xmlui-pg copy display name=\"Demo\"\n<App/>\n```\nPaste from the clipboard into an image.\n")
	// Body genuinely says "copy".
	writeHowtoFixture(t, howtoDir, "duplicate-rows.md",
		"# Duplicate rows\nCopy the clipboard contents into a new row.\n")

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"copy clipboard")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Salience == nil {
		t.Fatal("expected salience summary")
	}
	var copyEntry *TermCoverageEntry
	for i := range summary.Salience.TermCoverage {
		if summary.Salience.TermCoverage[i].Term == "copy" {
			copyEntry = &summary.Salience.TermCoverage[i]
		}
	}
	if copyEntry == nil {
		t.Fatalf("no term_coverage entry for copy: %+v", summary.Salience.TermCoverage)
	}
	if copyEntry.ContentMatches != 1 {
		t.Fatalf("copy content_matches = %d, want 1 (fence line must not count)", copyEntry.ContentMatches)
	}
	for _, items := range summary.Sections {
		for _, item := range items {
			if strings.Contains(item.Snippet, "```") {
				t.Fatalf("fence line leaked into snippets: %q", item.Snippet)
			}
		}
	}
}

// The record's per-term vector must include zero-count intent terms — the
// selection-proof content-gap tell.
func TestTermCoverageIncludesZeroCountIntentTerm(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHowtoFixture(t, howtoDir, "use-buttons.md",
		"# Use buttons\nWire a button click handler.\n")

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"microphone button")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Salience == nil {
		t.Fatal("expected salience summary")
	}
	found := false
	for _, entry := range summary.Salience.TermCoverage {
		if entry.Term == "microphone" {
			found = true
			if entry.TitleMatches != 0 || entry.ContentMatches != 0 {
				t.Fatalf("microphone coverage = %+v, want zeros", entry)
			}
		}
	}
	if !found {
		t.Fatalf("microphone missing from term_coverage: %+v", summary.Salience.TermCoverage)
	}
}

// The #14 contradiction shape: a rare incidental term must not drive the
// aggregates to zero when intent terms are strongly titled. Aggregates are
// unions over the band, so they must agree with term_coverage.
func TestSalientAggregatesConsistentWithTermCoverage(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeHowtoFixture(t, howtoDir, "follow-a-list-to-the-bottom.md",
		"# Follow a list to the bottom\nKeep the list scrolled with the bottom pinned as items arrive.\n")
	writeHowtoFixture(t, howtoDir, "implement-infinite-scroll.md",
		"# Implement infinite scroll\nScroll pagination for long lists.\n")
	writeHowtoFixture(t, howtoDir, "virtualize-long-lists.md",
		"# Virtualize long lists\nVirtualized scrolling for large data.\n")

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"scroll pinned bottom arrives")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Salience == nil {
		t.Fatal("expected salience summary")
	}
	band := strings.Join(summary.Salience.Terms, ",")
	if !strings.Contains(band, "pinned") || !strings.Contains(band, "bottom") {
		t.Fatalf("intent terms missing from band: %v", summary.Salience.Terms)
	}
	if summary.Salience.TitleMatchCount < 1 {
		t.Fatalf("salient_title_match_count = %d, want >=1 (bottom-titled doc exists): %+v",
			summary.Salience.TitleMatchCount, summary.Salience)
	}
	// Consistency invariant: a zero aggregate may not coexist with a nonzero
	// per-term count for any band term.
	inBand := map[string]bool{}
	for _, term := range summary.Salience.Terms {
		inBand[term] = true
	}
	for _, entry := range summary.Salience.TermCoverage {
		if !inBand[entry.Term] {
			continue
		}
		if entry.TitleMatches > 0 && summary.Salience.TitleMatchCount == 0 {
			t.Fatalf("aggregate title=0 contradicts term_coverage %+v", entry)
		}
		if entry.ContentMatches > 0 && summary.Salience.ContentMatchCount == 0 {
			t.Fatalf("aggregate content=0 contradicts term_coverage %+v", entry)
		}
	}
}

// Citation labels must be the document's own H1, not a title-cased slug (#23).
func TestDocumentTitleUsesRealHeading(t *testing.T) {
	dir := t.TempDir()

	divergent := filepath.Join(dir, "drive-a-slider-with-a-runtime-domain.md")
	if err := os.WriteFile(divergent, []byte("# Drive a Slider whose min/max domain changes at runtime\nBody.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	acronym := filepath.Join(dir, "generate-a-qr-code-from-user-input.md")
	if err := os.WriteFile(acronym, []byte("# Generate a QR code from user input\nBody.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	headingless := filepath.Join(dir, "no-heading.md")
	if err := os.WriteFile(headingless, []byte("Just prose, no heading.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := documentTitle(divergent, "howto/drive-a-slider-with-a-runtime-domain.md"); got != "Drive a Slider whose min/max domain changes at runtime" {
		t.Fatalf("divergent title = %q", got)
	}
	if got := documentTitle(acronym, "howto/generate-a-qr-code-from-user-input.md"); got != "Generate a QR code from user input" {
		t.Fatalf("acronym title = %q", got)
	}
	// No heading: fall back to the slug transform.
	if got := documentTitle(headingless, "howto/no-heading.md"); got != "No Heading" {
		t.Fatalf("headingless fallback = %q", got)
	}
	// Unreadable path: fall back.
	if got := documentTitle(filepath.Join(dir, "absent.md"), "howto/absent-doc.md"); got != "Absent Doc" {
		t.Fatalf("unreadable fallback = %q", got)
	}
}

func TestCitationFooterUsesRealTitles(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHowtoFixture(t, howtoDir, "generate-a-qr-code-from-user-input.md",
		"# Generate a QR code from user input\nRender a barcode qrcode from input.\n")

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"qrcode barcode input")
	if err != nil {
		t.Fatal(err)
	}
	if summary.AgentGuidance == nil || len(summary.AgentGuidance.DocumentationURLs) == 0 {
		t.Fatalf("expected documentation URLs, got %+v", summary.AgentGuidance)
	}
	for _, doc := range summary.AgentGuidance.DocumentationURLs {
		if doc.Title == "Generate A Qr Code From User Input" {
			t.Fatalf("fabricated slug-cased title leaked into citations: %+v", doc)
		}
	}
	found := false
	for _, doc := range summary.AgentGuidance.DocumentationURLs {
		if doc.Title == "Generate a QR code from user input" {
			found = true
		}
	}
	if !found {
		t.Fatalf("real H1 title missing from citations: %+v", summary.AgentGuidance.DocumentationURLs)
	}
}

// #25: a sectioned (howto-only) search can match a topic whose canonical doc
// lives outside the searched roots. The corroboration gate rightly excludes
// it from citations, but the computed location must still surface as a
// bounded, labeled lead rather than being discarded outright.
func TestOutOfScopePointersSurfaceUnreachableAnswers(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pagesDir := filepath.Join(root, "pages")
	if err := os.MkdirAll(pagesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeHowtoFixture(t, howtoDir, "build-a-fullscreen-modal-dialog.md",
		"# Build a fullscreen modal dialog\nModal dialog padding whitespace.\n")
	writeHowtoFixture(t, pagesDir, "working-with-text.md",
		"# Working with text\nBody.\n")

	topicIndex = []TopicEntry{
		{
			Name:          "whiteSpace",
			TriggerTerms:  []string{"whitespace", "preserve"},
			CanonicalDocs: []string{"pages/working-with-text.md"},
		},
	}

	human, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"preserve whitespace monospace logs")
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.OutOfScopePointers) != 1 {
		t.Fatalf("expected exactly one out-of-scope pointer, got %+v", summary.OutOfScopePointers)
	}
	doc := summary.OutOfScopePointers[0]
	if doc.Title != "Working with text" {
		t.Fatalf("pointer title = %q, want %q", doc.Title, "Working with text")
	}
	if doc.Path != "pages/working-with-text.md" {
		t.Fatalf("pointer path = %q, want %q", doc.Path, "pages/working-with-text.md")
	}
	if !strings.Contains(doc.URL, "working-with-text") {
		t.Fatalf("pointer URL = %q, want it to contain %q", doc.URL, "working-with-text")
	}
	if !strings.Contains(human, "Possibly relevant outside these results") {
		t.Fatalf("expected out-of-scope pointer block in human output, got: %s", human)
	}
	if !strings.Contains(human, doc.URL) {
		t.Fatalf("expected pointer URL in human output, got: %s", human)
	}
}

// High confidence means the search already answered the query; leads to
// elsewhere would only add noise, so they must be suppressed.
func TestOutOfScopePointersSuppressedOnHighConfidence(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pagesDir := filepath.Join(root, "pages")
	if err := os.MkdirAll(pagesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeHowtoFixture(t, howtoDir, "sync-tilegrid-selection-across-grids.md",
		"# Sync tilegrid selection across grids\nKeep tilegrid selection in sync across grids.\nSync the selection whenever either tilegrid changes.\n")
	writeHowtoFixture(t, howtoDir, "unrelated-topic.md",
		"# Unrelated topic\nNothing relevant here.\n")
	writeHowtoFixture(t, pagesDir, "elsewhere.md",
		"# Elsewhere\nBody.\n")

	// Trigger terms overlap two of the query's tokens, but the canonical doc
	// is not among the (howto-only) ranked results.
	topicIndex = []TopicEntry{
		{
			Name:          "Elsewhere Topic",
			TriggerTerms:  []string{"sync", "tilegrid"},
			CanonicalDocs: []string{"pages/elsewhere.md"},
		},
	}

	human, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"sync tilegrid selection across grids")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Confidence != "high" {
		t.Fatalf("expected high confidence, got %q", summary.Confidence)
	}
	if len(summary.OutOfScopePointers) != 0 {
		t.Fatalf("expected no out-of-scope pointers on high confidence, got %+v", summary.OutOfScopePointers)
	}
	if strings.Contains(human, "Possibly relevant outside") {
		t.Fatalf("expected no out-of-scope pointer block on high confidence, got: %s", human)
	}
}

// Pointers are capped and deduplicated against each other and against the
// corroborated citation list.
func TestOutOfScopePointersCapAndDedupe(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pagesDir := filepath.Join(root, "pages")
	if err := os.MkdirAll(pagesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeHowtoFixture(t, howtoDir, "unrelated-content.md",
		"# Unrelated content\nEntirely different subject matter.\n")

	names := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	var topics []TopicEntry
	for _, n := range names {
		title := strings.ToUpper(n[:1]) + n[1:]
		writeHowtoFixture(t, pagesDir, n+".md",
			"# "+title+" page\nBody.\n")
		topics = append(topics, TopicEntry{
			Name:          n,
			TriggerTerms:  []string{n},
			CanonicalDocs: []string{"pages/" + n + ".md"},
		})
	}
	topicIndex = topics

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"alpha bravo charlie delta echo")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Confidence == "high" {
		t.Fatalf("expected non-high confidence for weak howto corpus, got %q", summary.Confidence)
	}
	if len(summary.OutOfScopePointers) > maxOutOfScopePointers {
		t.Fatalf("expected at most %d pointers, got %d: %+v", maxOutOfScopePointers, len(summary.OutOfScopePointers), summary.OutOfScopePointers)
	}
	seenURLs := map[string]bool{}
	for _, doc := range summary.OutOfScopePointers {
		if seenURLs[doc.URL] {
			t.Fatalf("duplicate pointer URL: %s", doc.URL)
		}
		seenURLs[doc.URL] = true
	}
	if summary.AgentGuidance != nil {
		for _, cited := range summary.AgentGuidance.DocumentationURLs {
			if seenURLs[cited.URL] {
				t.Fatalf("pointer URL %s duplicates a corroborated citation", cited.URL)
			}
		}
	}
}

func TestReadFirstHeadingStripsAnchorMarkup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout-props.md")
	if err := os.WriteFile(path, []byte("# Layout Properties [#layout-summary]\nBody.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readFirstHeading(path); got != "Layout Properties" {
		t.Fatalf("anchor markup not stripped: %q", got)
	}
}

// The gap-vs-bad-query evidence must reach the agent in-band (#18).
func TestSalienceLinesRenderedInHumanOutput(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHowtoFixture(t, howtoDir, "use-buttons.md",
		"# Use buttons\nWire a button click handler.\n")

	// Medium/low path with an absent intent term: both lines render.
	// (Three tokens so the partial stage can match the button fixture.)
	human, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"microphone button click")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Confidence == "high" {
		t.Fatalf("fixture unexpectedly high confidence")
	}
	if !strings.Contains(human, "Terms absent from all results: microphone") {
		t.Fatalf("absent-terms line missing:\n%s", human)
	}
	if !strings.Contains(human, "Term coverage: ") || !strings.Contains(human, "microphone(absent)") {
		t.Fatalf("coverage vector missing:\n%s", human)
	}
	if !strings.Contains(human, "button(c:") {
		t.Fatalf("covered term missing from vector:\n%s", human)
	}

	// No-results path renders the absence evidence too.
	human, _, err = ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"zebra quantum flux")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(human, "No matches found.") || !strings.Contains(human, "Terms absent from all results:") {
		t.Fatalf("no-results path missing absence line:\n%s", human)
	}
}

func TestSalienceLinesSuppressedOnHigh(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHowtoFixture(t, howtoDir, "sync-tilegrid-selection-across-grids.md",
		"# Sync tilegrid selection across grids\nKeep tilegrid selection in sync across grids.\n")

	human, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"sync tilegrid selection across grids")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Confidence != "high" {
		t.Fatalf("fixture must be high confidence, got %q", summary.Confidence)
	}
	if strings.Contains(human, "Term coverage: ") {
		t.Fatalf("coverage vector must be suppressed at high:\n%s", human)
	}
}

// Citations are gated on the ranking's own evidence (#20): nothing cited at
// low confidence, tail results dropped by the relevance floor, capped, and
// deterministically ordered.
func TestCitationFooterGating(t *testing.T) {
	// Low confidence: no citations, even with results.
	low := extractDocumentationURLs(map[string][]resultItem{
		"howtos": {{Path: "howto/a.md", Score: 2.6}, {Path: "howto/b.md", Score: 0.5}},
	}, "low")
	if len(low) != 0 {
		t.Fatalf("low confidence must cite nothing, got %+v", low)
	}

	// Floor: sub-40%-of-top tail dropped; order deterministic by score.
	medium := extractDocumentationURLs(map[string][]resultItem{
		"howtos": {
			{Path: "howto/tail.md", Score: 0.53},
			{Path: "howto/top.md", Score: 2.63},
			{Path: "howto/near.md", Score: 2.0},
		},
	}, "medium")
	if len(medium) != 2 {
		t.Fatalf("expected floor to keep 2 of 3, got %+v", medium)
	}
	if !strings.Contains(medium[0].URL, "top") || !strings.Contains(medium[1].URL, "near") {
		t.Fatalf("citations not score-ordered: %+v", medium)
	}

	// Cap at maxDocumentationURLs even when all clear the floor.
	items := []resultItem{}
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		items = append(items, resultItem{Path: "howto/" + name + ".md", Score: 2.0})
	}
	capped := extractDocumentationURLs(map[string][]resultItem{"howtos": items}, "medium")
	if len(capped) != maxDocumentationURLs {
		t.Fatalf("cap = %d entries, got %d", maxDocumentationURLs, len(capped))
	}

	// Determinism: equal scores tie-break by path, stable across calls.
	tie := map[string][]resultItem{
		"howtos": {{Path: "howto/z.md", Score: 1.0}, {Path: "howto/a.md", Score: 1.0}},
	}
	first := extractDocumentationURLs(tie, "medium")
	second := extractDocumentationURLs(tie, "medium")
	if first[0].URL != second[0].URL || !strings.Contains(first[0].URL, "/a") {
		t.Fatalf("tie order not deterministic by path: %+v vs %+v", first, second)
	}
}

// The #27 rank-13 shape: generic-slug docs must not outrank the doc whose
// body answers the query. The filename bonus is token-boundary and gated on
// term rarity over the candidate set.
func TestFilenameBonusGatedOnRarity(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Four generic-slug docs: "component" in every filename and body.
	for _, name := range []string{"create-a-reusable-component.md", "handle-errors-on-a-component.md", "theme-a-component.md", "confine-input-to-a-component.md"} {
		writeHowtoFixture(t, howtoDir, name,
			"# "+name+"\nWork with a component in your app.\nEvery component does this.\n")
	}
	// The answering doc: body restates the query, slug shares no query token.
	writeHowtoFixture(t, howtoDir, "run-a-one-time-action-on-page-load.md",
		"# Run a one-time action on page load\nUse onMount to run initialization once when that component mounts.\nThe onMount initialization hook runs once per component.\n")

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"run initialization once when component mounts")
	if err != nil {
		t.Fatal(err)
	}
	topPath, topScore := "", 0.0
	for _, items := range summary.Sections {
		for _, item := range items {
			if item.Score > topScore {
				topScore, topPath = item.Score, item.Path
			}
		}
	}
	if !strings.Contains(topPath, "run-a-one-time-action") {
		t.Fatalf("answering doc must rank first, got %s (%.2f)", topPath, topScore)
	}
}

func TestFilenameMatchesTermTokenBoundary(t *testing.T) {
	cases := []struct {
		file string
		term string
		want bool
	}{
		{"transform-nested-api-responses.md", "form", false},       // substring, not a token
		{"enable-multi-row-selection-in-a-table.md", "tab", false}, // short stem never matches
		{"deep-link-to-a-tab-or-section.md", "linking", true},      // stem matches token
		{"pin-sticky-content-below-the-appheader.md", "appheader", true},
		{"wrap-long-text-in-a-link.md", "text", true},
		// "coding" keeps its suffix (trimming would leave a <4-char stem),
		// and "coding" != "code", so no match: stems must agree exactly.
		{"generate-a-qr-code-from-user-input.md", "coding", false},
	}

	for _, tc := range cases {
		if got := filenameMatchesTerm(tc.file, tc.term); got != tc.want {
			t.Fatalf("filenameMatchesTerm(%q, %q) = %v, want %v", tc.file, tc.term, tc.want, got)
		}
	}
}

// #27's positive control: an exact-slug match keeps its decisive win.
func TestFilenameBonusKeepsExactSlugWin(t *testing.T) {
	resetTopicIndexForTest(t)
	root := t.TempDir()
	howtoDir := filepath.Join(root, "howto")
	if err := os.MkdirAll(howtoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHowtoFixture(t, howtoDir, "pin-sticky-content-below-the-appheader.md",
		"# Pin sticky content below the AppHeader\nPin sticky content below the appheader with position settings.\n")
	writeHowtoFixture(t, howtoDir, "unrelated.md", "# Unrelated\nNothing here about appheader pinning.\n")

	_, summary, err := ExecuteMediatedSearch(root, howtoMediatorConfig(howtoDir),
		"pin sticky content below the AppHeader")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Confidence != "high" {
		t.Fatalf("exact slug match must stay high, got %q", summary.Confidence)
	}
}
