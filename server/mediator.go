package server

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// mdLinkRe matches markdown links like [text](path)
var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

//
// --------------------------- Public API ---------------------------
//

// MediatorConfig configures a mediated search run.
type MediatorConfig struct {
	// Absolute directories to scan (order matters for biasing).
	Roots []string

	// Section keys your tool wants to report (e.g. components/howtos/examples/source).
	// These are used to initialize the JSON "sections"+"facets" maps.
	SectionKeys []string

	// Preferred sections to bias when expanding (keeps your previous "docs first" behavior).
	PreferSections []string // e.g. []string{"components","howtos"}

	// Max human-readable hits (keeps your 50-cap).
	MaxResults int // default 50

	// Max snippet length in characters (default 200)
	MaxSnippetLength int

	// Max files to return after ranking (default 15)
	MaxFileResults int

	// Max snippets per file in output (default 3)
	MaxSnippetsPerFile int

	// File extensions to scan.
	FileExtensions []string // default: .mdx, .md, .tsx, .scss

	// Optional: tokens to drop when relaxing queries.
	Stopwords map[string]struct{}

	// Optional: synonyms expansion (token → []alternatives or phrases).
	Synonyms map[string][]string

	// Optional: per-hit classifier (relpath, absPath -> section key). If nil, SimpleClassifier() is used.
	// rel is relative to homeDir, absPath is the absolute file path.
	Classifier func(rel string, absPath string) string

	// Optional: enable filename matches (per your legacy behavior). Default true.
	EnableFilenameMatches bool

	// Optional: the MCP tool name this search serves (e.g. "xmlui_search_howto").
	// Guidance excludes the producing tool from pivot suggestions (#23 polish).
	ToolName string
}

// FacetCounts represents both match counts and unique file counts for a section
type FacetCounts struct {
	Files   int `json:"files"`   // unique files with matches
	Matches int `json:"matches"` // total matching lines
}

// DocumentationURL represents a specific documentation link
type DocumentationURL struct {
	Title string `json:"title"`
	Path  string `json:"path,omitempty"`
	URL   string `json:"url"`
	Type  string `json:"type"` // "component", "howto", "example", etc.
}

// AgentGuidance provides rule reminders and suggestions for low-confidence scenarios
type AgentGuidance struct {
	RuleReminders        []string           `json:"rule_reminders"`
	SuggestedApproach    string             `json:"suggested_approach"`
	URLBase              string             `json:"url_base,omitempty"`           // Base URL for constructing documentation links
	DocumentationURLs    []DocumentationURL `json:"documentation_urls,omitempty"` // Specific URLs found in documentation
	SearchToolPreference string             `json:"search_tool_preference,omitempty"`
}

// SalienceSummary describes how the ranked candidates answer the query's
// distinctive terms. TermCoverage is the authoritative per-term truth. Terms
// is the distinctive band — all substantive query terms minus in-corpus-
// generic ones, zero-coverage terms included (#14) — and the aggregate counts
// are candidate-level unions over that band, derived from the same basis as
// TermCoverage. UnansweredTerms are substantive query terms no candidate
// covers at all: non-empty means the query's real intent is unanswerable in
// the corpus, the content-gap signature (#12).
type SalienceSummary struct {
	Terms             []string            `json:"terms"`
	TitleMatchCount   int                 `json:"title_match_count"`
	ContentMatchCount int                 `json:"content_match_count"`
	UnansweredTerms   []string            `json:"unanswered_terms"`
	TermCoverage      []TermCoverageEntry `json:"term_coverage"`
}

// TermCoverageEntry reports how the ranked candidates answer one substantive
// query term, zeros included. Unlike the salient aggregates it involves no
// term selection, so a consumer sees the full vector — a zero-count intent
// term on a prose query is the content-gap tell regardless of which terms
// rarity crowned salient (#13).
type TermCoverageEntry struct {
	Term           string `json:"term"`
	TitleMatches   int    `json:"title_matches"`
	ContentMatches int    `json:"content_matches"`
}

// MediatorJSON is the machine-readable summary we append after the human block.
type MediatorJSON struct {
	QueryPlan           []stageHit              `json:"query_plan"`
	Tokens              map[string][]string     `json:"tokens"` // kept/removed/expanded
	Sections            map[string][]resultItem `json:"sections"`
	Facets              map[string]FacetCounts  `json:"facets"`
	Confidence          string                  `json:"confidence"`
	Salience            *SalienceSummary        `json:"salience,omitempty"`
	RelatedQueries      []string                `json:"related_queries"`
	AgentGuidance       *AgentGuidance          `json:"agent_guidance,omitempty"`
	Diagnostics         map[string]any          `json:"diagnostics,omitempty"`
	SearchToolHierarchy []string                `json:"search_tool_hierarchy,omitempty"`
	TopicMatches        []string                `json:"topic_matches,omitempty"`
	Suggestions         []string                `json:"suggestions,omitempty"`
	OutOfScopePointers  []DocumentationURL      `json:"out_of_scope_pointers,omitempty"`
}

// ExecuteMediatedSearch runs the staged scan and returns:
//
//  1. human readable block,
//
//  2. JSON summary (also included at the end of the human block),
//
//  3. error if something goes wrong (I/O etc. are soft-failed inside).
// scoredFile accumulates matches for a single file during search.
type scoredFile struct {
	RelPath    string
	AbsPath    string
	Section    string
	Score      float64
	Snippets   []scoredSnippet
	Deprecated      bool   // true if file contains a [!WARNING] deprecation notice
	ReplacementText string // e.g. "global variables"
	ReplacementLink string // e.g. "/guides/markup#global-variables"
	TitleMatch      bool   // filename contains a query term (the scoring bonus fired)
	// tracking which query terms were found in this file
	TermsFound map[string]bool
}

type scoredSnippet struct {
	Line     int
	Text     string
	IsTitle  bool // filename match or heading
	TermHits int  // distinct query terms on this line, set at hit time
}

func ExecuteMediatedSearch(homeDir string, cfg MediatorConfig, originalQuery string) (string, MediatorJSON, error) {
	// defaults
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = 50
	}
	if cfg.MaxSnippetLength <= 0 {
		cfg.MaxSnippetLength = 200
	}
	if cfg.MaxFileResults <= 0 {
		cfg.MaxFileResults = 15
	}
	if cfg.MaxSnippetsPerFile <= 0 {
		cfg.MaxSnippetsPerFile = 3
	}
	if len(cfg.FileExtensions) == 0 {
		cfg.FileExtensions = []string{".mdx", ".md", ".tsx", ".scss"}
	}
	if cfg.Classifier == nil {
		cfg.Classifier = SimpleClassifier(homeDir, []string{})
	}
	if cfg.EnableFilenameMatches == false {
		// leave as-is; default is true below
	} else {
		cfg.EnableFilenameMatches = true
	}
	if len(cfg.SectionKeys) == 0 {
		cfg.SectionKeys = []string{"components", "howtos", "examples", "source"}
	}

	// Prepare accumulators
	fileScores := make(map[string]*scoredFile) // keyed by absPath

	jsonOut := MediatorJSON{
		QueryPlan: []stageHit{},
		Tokens:    map[string][]string{"kept": {}, "removed": {}, "expanded": {}},
		Sections:  make(map[string][]resultItem),
		Facets:    make(map[string]FacetCounts),
		Diagnostics: map[string]any{
			"original_query": strings.TrimSpace(originalQuery),
		},
	}

	// Initialize sections for stable ordering
	for _, k := range cfg.SectionKeys {
		jsonOut.Sections[k] = []resultItem{}
	}

	// Normalize query tokens for scoring
	kept, removed := normalizeTokens(originalQuery, cfg.Stopwords)
	jsonOut.Tokens["kept"] = kept
	jsonOut.Tokens["removed"] = removed
	queryTerms := kept
	if len(queryTerms) == 0 {
		queryTerms = strings.Fields(strings.ToLower(originalQuery))
	}

	// -------- Topic matching (Rec #4) --------
	ensureTopicIndex(homeDir)
	topicMatches := matchTopics(queryTerms)
	topicBonusFiles := make(map[string]bool) // canonical doc paths that get bonus
	if len(topicMatches) > 0 {
		for _, tm := range topicMatches {
			jsonOut.TopicMatches = append(jsonOut.TopicMatches, tm.Name)
			for _, doc := range tm.CanonicalDocs {
				topicBonusFiles[doc] = true
			}
		}
	}

	// -------- helpers --------

	addFileHit := func(rel string, absPath string, lineNum int, line string, queryTermsForMatch []string) {
		// Code-fence marker lines (```xmlui-pg copy display name="…") are
		// markup, not content: 173 of 190 howto docs carry that literal
		// playground fence, making its tokens read corpus-ubiquitous. They
		// contribute neither term coverage nor snippets (#13).
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			return
		}

		// Truncate snippet
		snippet := line
		if len(snippet) > cfg.MaxSnippetLength {
			snippet = snippet[:cfg.MaxSnippetLength] + "..."
		}

		sf, exists := fileScores[absPath]
		if !exists {
			section := cfg.Classifier(rel, absPath)
			sf = &scoredFile{
				RelPath:    rel,
				AbsPath:    absPath,
				Section:    section,
				TermsFound: make(map[string]bool),
			}
			fileScores[absPath] = sf
		}

		// Track which query terms this hit covers
		snippetLower := strings.ToLower(snippet)
		termHits := 0
		for _, term := range queryTermsForMatch {
			if strings.Contains(snippetLower, term) {
				sf.TermsFound[term] = true
				termHits++
			}
		}

		// Deduplicate by line number (same line can match across stages)
		isDuplicate := false
		for _, existing := range sf.Snippets {
			if existing.Line == lineNum {
				isDuplicate = true
				break
			}
		}

		// Add snippet. The pool is capped, but by term density rather than
		// first-seen order: a match-dense doc otherwise fills its pool with
		// early lines and the one line answering the query's distinctive term
		// never reaches snippet selection (#28: the answering L79 sat beyond
		// the cap while the lede monopolized the pool).
		if !isDuplicate {
			isTitle := lineNum == 0 || strings.HasPrefix(strings.TrimSpace(line), "#")
			snip := scoredSnippet{Line: lineNum, Text: snippet, IsTitle: isTitle, TermHits: termHits}
			if len(sf.Snippets) < 20 {
				sf.Snippets = append(sf.Snippets, snip)
			} else {
				weakest, weakestHits := -1, termHits
				for i, existing := range sf.Snippets {
					if !existing.IsTitle && existing.TermHits < weakestHits {
						weakest, weakestHits = i, existing.TermHits
					}
				}
				if weakest >= 0 {
					sf.Snippets[weakest] = snip
				}
			}
		}
	}

	runStage := func(stageName, stageQuery string, roots []string, usePartialMatch bool) int {
		hits := 0
		if stageQuery == "" {
			jsonOut.QueryPlan = append(jsonOut.QueryPlan, stageHit{Stage: stageName, Query: stageQuery, Hits: 0})
			return 0
		}
		lq := strings.ToLower(stageQuery)
		minWords := 0
		if usePartialMatch {
			minWords = calculateMinWords(len(strings.Fields(lq)))
		}

		for _, root := range roots {
			_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					if d.Name() == "node_modules" || d.Name() == ".git" {
						return filepath.SkipDir
					}
					return nil
				}
				if !hasAllowedExt(d.Name(), cfg.FileExtensions) {
					return nil
				}

				var matchFunc func(string, string) bool
				if usePartialMatch {
					matchFunc = func(text, query string) bool {
						return partialMatch(text, query, minWords)
					}
				} else {
					matchFunc = fuzzyMatch
				}

				rel, _ := filepath.Rel(homeDir, path)

				if cfg.EnableFilenameMatches && matchFunc(d.Name(), lq) {
					addFileHit(rel, path, 0, "[filename match]", queryTerms)
					hits++
				}

				f, err := os.Open(path)
				if err != nil {
					return nil
				}
				defer f.Close()

				sc := bufio.NewScanner(f)
				ln := 1
				sawWarning := false
				for sc.Scan() {
					line := sc.Text()
					if matchFunc(line, lq) {
						addFileHit(rel, path, ln, line, queryTerms)
						hits++
					}
					// Detect deprecation: [!WARNING] + "deprecated" nearby
					if strings.Contains(line, "[!WARNING]") {
						sawWarning = true
					}
					if sawWarning && strings.Contains(strings.ToLower(line), "deprecated") {
						if sf, ok := fileScores[path]; ok {
							sf.Deprecated = true
						}
					}
					// Extract replacement link from deprecation block
					if sawWarning {
						if m := mdLinkRe.FindStringSubmatch(line); m != nil {
							if sf, ok := fileScores[path]; ok && sf.ReplacementLink == "" {
								sf.ReplacementText = m[1]
								sf.ReplacementLink = m[2]
							}
						}
						// Stop scanning the warning block after a blank line
						if strings.TrimSpace(line) == "" {
							sawWarning = false
						}
					}
					ln++
				}
				return nil
			})
		}
		jsonOut.QueryPlan = append(jsonOut.QueryPlan, stageHit{Stage: stageName, Query: lq, Hits: hits})
		return hits
	}

	// -------- stages --------

	totalHits := 0

	// Stage 1: exact
	totalHits += runStage("exact", strings.ToLower(originalQuery), cfg.Roots, false)

	// Stage 2: relaxed (strip sigils/stopwords)
	if len(kept) > 0 {
		relaxed := strings.Join(kept, " ")
		totalHits += runStage("relaxed", relaxed, cfg.Roots, false)
	}

	// Stage 3: partial matching
	if len(kept) > 0 {
		relaxed := strings.Join(kept, " ")
		roots := cfg.Roots
		if looksLikeConcept(kept) && len(cfg.PreferSections) > 0 {
			roots = reorderRootsByPreference(cfg.Roots, cfg.PreferSections)
		}
		totalHits += runStage("partial", relaxed, roots, true)
		jsonOut.Tokens["expanded"] = kept
	}

	// -------- Score files --------
	sectionWeights := map[string]float64{
		"components": 1.5,
		"howtos":     1.5,
		"examples":   1.2,
		"source":     1.0,
		"blog":       0.8,
		"unknown":    0.5,
	}

	// Per-term document frequency over all candidates, computable here because
	// scoring runs after hit collection. The filename bonus fires only for
	// terms outside the in-corpus-generic band (the #14 rule): a slug hit on
	// "component"/"input"-class tokens is coincidence, not relevance, and a
	// flat +2.0 for them let eleven generic-slug docs outrank the doc whose
	// body restates the query (#27). Small candidate sets skip the gate, as
	// the salience band does.
	bonusEligible := make(map[string]bool, len(queryTerms))
	genericThreshold := -1
	if len(fileScores) >= 3 {
		genericThreshold = (2*len(fileScores) + 2) / 3
	}
	for _, term := range queryTerms {
		if len(termStem(term)) < 4 {
			continue
		}
		if genericThreshold < 0 {
			bonusEligible[term] = true
			continue
		}
		df := 0
		for _, sf := range fileScores {
			if sf.TermsFound[term] {
				df++
			}
		}
		bonusEligible[term] = df < genericThreshold
	}

	for _, sf := range fileScores {
		// (a) Term coverage: distinct query terms found / total query terms
		if len(queryTerms) > 0 {
			sf.Score += float64(len(sf.TermsFound)) / float64(len(queryTerms))
		}

		// (b) Section weight
		weight, ok := sectionWeights[sf.Section]
		if !ok {
			weight = 1.0
		}
		sf.Score *= weight

		// (c) Filename match bonus: token-boundary, stem-aware, and only for
		// terms outside the generic band (#11, #27).
		for _, term := range queryTerms {
			if bonusEligible[term] && filenameMatchesTerm(sf.RelPath, term) {
				sf.Score += 2.0
				sf.TitleMatch = true
				break
			}
		}

		// (d) Topic bonus
		for bonusPath := range topicBonusFiles {
			if strings.Contains(sf.RelPath, bonusPath) {
				sf.Score += 5.0
				break
			}
		}

		// (e) Match density bonus (more snippets = more relevant)
		sf.Score += float64(len(sf.Snippets)) * 0.1

		// (f) Deprecation penalty: demote files with [!WARNING] + "deprecated"
		if sf.Deprecated {
			sf.Score *= 0.3
		}
	}

	// Sort files by score descending
	ranked := make([]*scoredFile, 0, len(fileScores))
	for _, sf := range fileScores {
		ranked = append(ranked, sf)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		// Tie-break by path: the pre-sort order comes from map iteration, so
		// without this, equal-score files shuffle run to run and the top-N
		// cutoff (hence df, salience, and analytics records) is nondeterministic.
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].RelPath < ranked[j].RelPath
		}
		return ranked[i].Score > ranked[j].Score
	})

	// Take top N files
	if len(ranked) > cfg.MaxFileResults {
		ranked = ranked[:cfg.MaxFileResults]
	}

	// Salience is computed before sections are built so snippet selection can
	// prefer lines covering the query's distinctive terms (#28).
	salience := computeSalience(ranked, queryTerms)

	// Build sections and facets from ranked files
	uniqueFiles := make(map[string]map[string]struct{})
	for _, k := range cfg.SectionKeys {
		uniqueFiles[k] = make(map[string]struct{})
	}

	for _, sf := range ranked {
		section := sf.Section
		if _, ok := jsonOut.Sections[section]; !ok {
			jsonOut.Sections[section] = []resultItem{}
			uniqueFiles[section] = make(map[string]struct{})
		}
		uniqueFiles[section][sf.RelPath] = struct{}{}

		// Pick best snippets: prefer title/heading lines, then first N
		bestSnippets := pickBestSnippets(sf.Snippets, cfg.MaxSnippetsPerFile, queryTerms)
		for _, snip := range bestSnippets {
			jsonOut.Sections[section] = append(jsonOut.Sections[section], resultItem{
				Type:       section,
				Path:       sf.RelPath,
				AbsPath:    sf.AbsPath,
				Line:       snip.Line,
				Snippet:    snip.Text,
				Score:      sf.Score,
				TitleMatch: sf.TitleMatch,
			})
		}
	}

	// Build facets
	for k := range jsonOut.Sections {
		jsonOut.Facets[k] = FacetCounts{
			Files:   len(uniqueFiles[k]),
			Matches: len(jsonOut.Sections[k]),
		}
	}

	// Confidence from the top of the score distribution: "high" means the best
	// hit covers most query terms and stands out from the tail, not that many
	// files contained common tokens (#10).
	jsonOut.Confidence = confidenceFromRanked(ranked, queryTerms)

	// Salience summary for gap classification downstream (#12).
	jsonOut.Salience = &salience

	// Set search tool hierarchy for howto/example queries
	if isHowToQuery(originalQuery) || isExampleQuery(originalQuery) {
		jsonOut.SearchToolHierarchy = []string{
			"xmlui_examples (preferred)",
			"xmlui_search_howto (preferred)",
			"xmlui_search (fallback)",
		}
	}

	// Agent guidance
	jsonOut.AgentGuidance = generateAgentGuidance(cfg.ToolName, jsonOut.Confidence, jsonOut.Facets, jsonOut.Sections, originalQuery, kept, homeDir)

	// Inject topic URLs into guidance only for topics corroborated by the
	// ranked results, deduplicated and capped: an uncorroborated flat dump of
	// every trigger-matched topic repeats the ranked list while dominating the
	// payload (#10).
	if len(topicMatches) > 0 && jsonOut.AgentGuidance != nil {
		registry := GetURLRegistry(homeDir)
		jsonOut.AgentGuidance.DocumentationURLs = corroboratedTopicURLs(topicMatches, ranked, registry.ValidateURL, maxDocumentationURLs)
	}

	// Out-of-scope pointers (#25): a sectioned search (e.g. how-to-only) can
	// match a topic whose canonical docs live outside the searched roots —
	// the corroboration gate above rightly excludes them from citations, but
	// discarding them entirely turns "answered elsewhere" into a false gap.
	// Emit them as bounded, explicitly-labeled leads instead. Suppressed on
	// high confidence: the search answered, leads would be noise.
	if len(topicMatches) > 0 && jsonOut.Confidence != "high" {
		jsonOut.OutOfScopePointers = outOfScopePointers(homeDir, topicMatches, ranked, jsonOut.AgentGuidance)
	}

	// "Did You Mean?" suggestions (Rec #5). Deduplicated before the cap — the
	// same name can arrive by two paths — and NOT mirrored into RuleReminders:
	// that mirror printed the line twice on the zero-hit path (#21, #23).
	if len(ranked) == 0 || jsonOut.Confidence == "low" {
		suggestions := dedupeStrings(suggestAlternatives(originalQuery, homeDir, 6))
		if len(suggestions) > 3 {
			suggestions = suggestions[:3]
		}
		if len(suggestions) > 0 {
			jsonOut.Suggestions = suggestions
		}
	}

	// Related queries - removed for now
	jsonOut.RelatedQueries = []string{}

	// -------- Human block --------
	var out strings.Builder
	if len(ranked) == 0 {
		out.WriteString("No matches found.\n")
		writeSalienceLines(&out, jsonOut)

		hasGuidance := false
		if jsonOut.AgentGuidance != nil {
			if len(jsonOut.AgentGuidance.RuleReminders) > 0 ||
				jsonOut.AgentGuidance.SuggestedApproach != "" ||
				jsonOut.AgentGuidance.SearchToolPreference != "" {
				hasGuidance = true
			}
		}

		if hasGuidance {
			out.WriteString("\n")
			if jsonOut.AgentGuidance != nil {
				if len(jsonOut.AgentGuidance.RuleReminders) > 0 {
					for _, reminder := range jsonOut.AgentGuidance.RuleReminders {
						out.WriteString("• " + reminder + "\n")
					}
				}
				if jsonOut.AgentGuidance.SuggestedApproach != "" {
					out.WriteString("\n" + jsonOut.AgentGuidance.SuggestedApproach + "\n")
				}
				if jsonOut.AgentGuidance.SearchToolPreference != "" {
					out.WriteString("\nPREFERRED TOOL: " + jsonOut.AgentGuidance.SearchToolPreference + "\n")
				}
			}
		}

		if len(jsonOut.Suggestions) > 0 {
			out.WriteString("\nDid you mean: " + strings.Join(jsonOut.Suggestions, ", ") + "?\n")
		}

		writeGuidanceBlock(&out, jsonOut)
		return out.String(), jsonOut, nil
	}

	fmt.Fprintf(&out, "Query: %q  (files=%d, total_hits=%d, confidence=%s)\n",
		originalQuery, len(ranked), totalHits, jsonOut.Confidence)
	writeSalienceLines(&out, jsonOut)

	// Show topic matches
	if len(jsonOut.TopicMatches) > 0 {
		fmt.Fprintf(&out, "Topics: %s\n", strings.Join(jsonOut.TopicMatches, ", "))
	}

	fmt.Fprintf(&out, "Facets: ")
	keys := keysSortedV2(jsonOut.Facets)
	for i, k := range keys {
		if i > 0 {
			out.WriteString("  ")
		}
		facet := jsonOut.Facets[k]
		if facet.Files == 1 {
			fmt.Fprintf(&out, "%s=%d", k, facet.Matches)
		} else if facet.Files > 0 {
			fmt.Fprintf(&out, "%s=%d files (%d matches)", k, facet.Files, facet.Matches)
		}
	}
	out.WriteString("\n\n")

	// Grouped-by-file output with scores
	for _, sf := range ranked {
		fmt.Fprintf(&out, "## %s  (score=%.2f, section=%s)\n", sf.RelPath, sf.Score, sf.Section)
		if sf.Deprecated {
			if sf.ReplacementLink != "" {
				fmt.Fprintf(&out, "  **DEPRECATED**: Use [%s](%s%s) instead.\n", sf.ReplacementText, constructURLBase(), sf.ReplacementLink)
			} else {
				out.WriteString("  **DEPRECATED**\n")
			}
		}
		bestSnippets := pickBestSnippets(sf.Snippets, cfg.MaxSnippetsPerFile, queryTerms)
		for _, snip := range bestSnippets {
			if snip.Line == 0 {
				fmt.Fprintf(&out, "  %s\n", snip.Text)
			} else {
				fmt.Fprintf(&out, "  L%d: %s\n", snip.Line, snip.Text)
			}
		}
		out.WriteString("\n")
	}

	if len(jsonOut.Suggestions) > 0 {
		out.WriteString("Did you mean: " + strings.Join(jsonOut.Suggestions, ", ") + "?\n\n")
	}

	writeGuidanceBlock(&out, jsonOut)

	return out.String(), jsonOut, nil
}

// writeSalienceLines renders the in-band gap-vs-bad-query evidence (#18):
// the absent-terms line whenever any substantive term matched nothing, and
// the compact per-term coverage vector below high confidence — presentation
// of fields the salience work (#12/#13) already computes on every search.
func writeSalienceLines(out *strings.Builder, jsonOut MediatorJSON) {
	if jsonOut.Salience == nil {
		return
	}
	if len(jsonOut.Salience.UnansweredTerms) > 0 {
		fmt.Fprintf(out, "Terms absent from all results: %s\n", strings.Join(jsonOut.Salience.UnansweredTerms, ", "))
	}
	if jsonOut.Confidence == "high" || len(jsonOut.Salience.TermCoverage) == 0 {
		return
	}
	parts := make([]string, 0, len(jsonOut.Salience.TermCoverage))
	for _, entry := range jsonOut.Salience.TermCoverage {
		switch {
		case entry.ContentMatches == 0 && entry.TitleMatches == 0:
			parts = append(parts, entry.Term+"(absent)")
		case entry.TitleMatches > 0:
			parts = append(parts, fmt.Sprintf("%s(c:%d t:%d)", entry.Term, entry.ContentMatches, entry.TitleMatches))
		default:
			parts = append(parts, fmt.Sprintf("%s(c:%d)", entry.Term, entry.ContentMatches))
		}
	}
	fmt.Fprintf(out, "Term coverage: %s\n", strings.Join(parts, " "))
}

// writeGuidanceBlock appends agent guidance and documentation URLs
// to the human-readable output, replacing the verbose full JSON dump.
func writeGuidanceBlock(out *strings.Builder, jsonOut MediatorJSON) {
	out.WriteString("---\n")

	if len(jsonOut.SearchToolHierarchy) > 0 {
		out.WriteString("Preferred tools: " + strings.Join(jsonOut.SearchToolHierarchy, ", ") + "\n")
	}

	if jsonOut.AgentGuidance != nil {
		if jsonOut.AgentGuidance.SuggestedApproach != "" {
			out.WriteString("Suggested approach: " + jsonOut.AgentGuidance.SuggestedApproach + "\n")
		}
		if jsonOut.AgentGuidance.SearchToolPreference != "" {
			out.WriteString("Preferred tool: " + jsonOut.AgentGuidance.SearchToolPreference + "\n")
		}
		if len(jsonOut.AgentGuidance.DocumentationURLs) > 0 {
			out.WriteString("\nDocumentation URLs:\n")
			for _, doc := range jsonOut.AgentGuidance.DocumentationURLs {
				fmt.Fprintf(out, "  - %s: %s\n", doc.Title, doc.URL)
			}
		}
	}

	if len(jsonOut.OutOfScopePointers) > 0 {
		out.WriteString("\nPossibly relevant outside these results (leads to verify, not citations):\n")
		for _, doc := range jsonOut.OutOfScopePointers {
			fmt.Fprintf(out, "  - %s (%s): %s\n", doc.Title, doc.Path, doc.URL)
		}
	}
}

// pickBestSnippets selects the best N snippets from a file's matches. The
// first title line is kept for identity; remaining slots go to the lines
// covering the most distinctive query terms, so a doc cannot be shown only
// through lines its own body contradicts (#28: an explicitly vertical query
// surfaced just the "Row"/"HStack" lede while the answering VStack line sat
// unshown at L79). Pure cross-reference lines never stand in for content.
func pickBestSnippets(snippets []scoredSnippet, maxN int, distinctiveTerms []string) []scoredSnippet {
	if len(snippets) <= maxN {
		return snippets
	}

	lineTerms := func(s scoredSnippet) map[string]bool {
		text := strings.ToLower(s.Text)
		covered := make(map[string]bool)
		for _, term := range distinctiveTerms {
			t := strings.ToLower(term)
			if strings.Contains(text, termStem(t)) {
				covered[t] = true
			}
		}
		return covered
	}

	result := make([]scoredSnippet, 0, maxN)
	used := make(map[int]bool)
	covered := make(map[string]bool)
	// The identity slot goes to a real heading when one exists: the
	// "[filename match]" pseudo-snippet is also IsTitle but carries no
	// information, and it must not displace the document's own title.
	titleIdx := -1
	for i, s := range snippets {
		if s.IsTitle && strings.HasPrefix(strings.TrimSpace(s.Text), "#") {
			titleIdx = i
			break
		}
	}
	if titleIdx < 0 {
		for i, s := range snippets {
			if s.IsTitle {
				titleIdx = i
				break
			}
		}
	}
	if titleIdx >= 0 {
		result = append(result, snippets[titleIdx])
		used[titleIdx] = true
		for t := range lineTerms(snippets[titleIdx]) {
			covered[t] = true
		}
	}

	// Greedy marginal coverage: each slot goes to the line adding the most
	// query terms not yet shown, so the visible snippets collectively answer
	// the query instead of restating the lede's terms. Prose outranks markup
	// boilerplate (verticalAlignment="center" adds tokens but no information —
	// the consumer-verified #28 failure), and cross-reference lines are last;
	// remaining ties break on total term density, then file order.
	for len(result) < maxN {
		bestIdx, bestMarginal, bestHits, bestTier := -1, -1, -1, 3
		for i, s := range snippets {
			if used[i] {
				continue
			}
			tier := 0
			if isMarkupLine(s.Text) {
				tier = 1
			}
			if isCrossReferenceLine(s.Text) {
				tier = 2
			}
			marginal := 0
			for t := range lineTerms(s) {
				if !covered[t] {
					marginal++
				}
			}
			better := false
			switch {
			case bestIdx < 0:
				better = true
			case tier != bestTier:
				better = tier < bestTier
			case marginal != bestMarginal:
				better = marginal > bestMarginal
			case s.TermHits != bestHits:
				better = s.TermHits > bestHits
			}
			if better {
				bestIdx, bestMarginal, bestHits, bestTier = i, marginal, s.TermHits, tier
			}
		}
		if bestIdx < 0 {
			break
		}
		used[bestIdx] = true
		result = append(result, snippets[bestIdx])
		for t := range lineTerms(snippets[bestIdx]) {
			covered[t] = true
		}
	}
	// Selection order optimized coverage; presentation restores document order.
	sort.SliceStable(result, func(i, j int) bool { return result[i].Line < result[j].Line })
	return result
}

// isMarkupLine reports whether a line is code or attribute boilerplate rather
// than prose: XML/markup tags, attribute="value" fragments, braces, or fence
// residue. Such lines can cover query tokens without carrying any answer.
var attrLineRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*\s*=\s*["'{]`)

func isMarkupLine(text string) bool {
	s := strings.TrimSpace(text)
	if s == "" {
		return true
	}
	switch s[0] {
	case '<', '{', '}', '`':
		return true
	}
	return attrLineRe.MatchString(s)
}

// isCrossReferenceLine reports whether a line opens with a markdown link
// (after any list marker) — the see-also shape, not this doc's own content
// (#28: a doc that merely links to the answer must not be represented by that
// line). Prose with an inline link mid-sentence is content and stays eligible.
func isCrossReferenceLine(text string) bool {
	s := strings.TrimSpace(text)
	s = strings.TrimLeft(s, "-*•0123456789. \t")
	loc := mdLinkRe.FindStringIndex(s)
	return loc != nil && loc[0] == 0
}

//
// ------------------------ Helpers / Core -------------------------
//

// fuzzyMatch: simple AND-of-words contains matching (case-insensitive).
func fuzzyMatch(text, query string) bool {
	t := strings.ToLower(text)
	q := strings.ToLower(query)
	words := strings.Fields(q)
	if len(words) == 1 {
		return strings.Contains(t, q)
	}
	for _, w := range words {
		if !strings.Contains(t, w) {
			return false
		}
	}
	return true
}

// partialMatch: relaxed matching requiring only a subset of words to be present.
func partialMatch(text, query string, minWords int) bool {
	t := strings.ToLower(text)
	words := strings.Fields(strings.ToLower(query))
	if len(words) <= 1 {
		return strings.Contains(t, strings.ToLower(query))
	}

	found := 0
	for _, w := range words {
		if strings.Contains(t, w) {
			found++
		}
	}
	return found >= minWords
}

// calculateMinWords: smart threshold calculation for partial matching.
func calculateMinWords(totalWords int) int {
	switch {
	case totalWords <= 2:
		return totalWords // 100% for 1-2 words
	case totalWords <= 4:
		return 2 // 50% for 3-4 words
	case totalWords >= 5:
		return 2 // Just 2 words for 5+ word queries
	default:
		return 1
	}
}

func hasAllowedExt(name string, allowed []string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, a := range allowed {
		if ext == strings.ToLower(a) {
			return true
		}
	}
	return false
}

type stageHit struct {
	Stage string `json:"stage"`
	Query string `json:"query"`
	Hits  int    `json:"hits"`
}

type resultItem struct {
	Type       string  `json:"type"` // section key
	Path       string  `json:"path"`
	AbsPath    string  `json:"abs_path,omitempty"`
	Line       int     `json:"line"`
	Snippet    string  `json:"snippet"`
	Score      float64 `json:"score,omitempty"`
	TitleMatch bool    `json:"title_match,omitempty"`
}

// normalizeTokens: lowercase, strip simple punctuation/sigils, drop stopwords.
func normalizeTokens(q string, stop map[string]struct{}) (kept []string, removed []string) {
	s := strings.ToLower(q)
	replacer := strings.NewReplacer(
		`"`, " ", "`", " ", "'", " ",
		"{", " ", "}", " ", "(", " ", ")", " ",
		"[", " ", "]", " ", "<", " ", ">", " ",
		"$", " ", "@", " ", "=", " ", ":", " ",
	)
	s = replacer.Replace(s)
	for _, tok := range strings.Fields(s) {
		if _, isStop := stop[tok]; isStop {
			removed = append(removed, tok)
			continue
		}
		kept = append(kept, tok)
	}
	return
}

// looksLikeConcept: simple heuristic — any token looks "identifier-ish"
func looksLikeConcept(tokens []string) bool {
	for _, t := range tokens {
		if len(t) >= 3 && (isAlphaNum(t[0]) || t[0] == '_') {
			return true
		}
	}
	return false
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

func reorderRootsByPreference(roots []string, preferredSections []string) []string {
	// Score roots by whether they contain path segments associated with preferred sections.
	type scored struct {
		root  string
		score int
	}
	scoreRoot := func(root string) int {
		r := strings.ReplaceAll(root, "\\", "/")
		score := 0
		for i, sec := range preferredSections {
			switch sec {
			case "components":
				if containsSegment(r, "components") || containsSegment(r, "pages") {
					score = max(score, 100-i)
				}
			case "howtos":
				if containsSegment(r, "howto") {
					score = max(score, 100-i)
				}
			case "source":
				if strings.Contains(r, "/src/components") {
					score = max(score, 100-i)
				}
			case "blog":
				if containsSegment(r, "blog") {
					score = max(score, 100-i)
				}
			}
		}
		return score
	}
	arr := make([]scored, 0, len(roots))
	for _, r := range roots {
		arr = append(arr, scored{root: r, score: scoreRoot(r)})
	}
	sort.SliceStable(arr, func(i, j int) bool { return arr[i].score > arr[j].score })
	out := make([]string, 0, len(arr))
	for _, s := range arr {
		out = append(out, s.root)
	}
	return out
}

// maxDocumentationURLs bounds the topic-URL block appended to guidance.
const maxDocumentationURLs = 5

// confidenceFromRanked derives confidence from the best-ranked hit rather than
// match volume. Coverage is the fraction of query terms the top hit contains;
// volume alone can no longer produce "high" (#10).
func confidenceFromRanked(ranked []*scoredFile, queryTerms []string) string {
	if len(ranked) == 0 {
		return "low"
	}
	coverage := 1.0
	if len(queryTerms) > 0 {
		coverage = float64(len(ranked[0].TermsFound)) / float64(len(queryTerms))
	}
	switch {
	case coverage < 0.34:
		return "low"
	case coverage >= 0.75 && topStandsOut(ranked) && topCoversDistinctiveTerms(ranked, queryTerms):
		return "high"
	default:
		return "medium"
	}
}

// topCoversDistinctiveTerms requires the best hit (or a near-tied co-top hit)
// to cover every achievable query term: every term at least one ranked
// candidate answers, stem-aware. Aggregate coverage of generic tokens can
// otherwise read "high" while the query's salient concept goes unanswered in
// the top hit yet answered elsewhere in the corpus (#11). Terms no candidate
// matches don't gate; they already depress coverage.
func topCoversDistinctiveTerms(ranked []*scoredFile, queryTerms []string) bool {
	df, minDF := termDocumentFrequency(ranked, queryTerms)
	if minDF == 0 {
		return true
	}
	// Rank order inside a narrow score band is noise (live corpus: 4.20 vs
	// 4.10 for a query-params doc vs the answering deep-link doc), so any
	// near-tied co-top hit may satisfy the guard, not just ranked[0].
	for _, sf := range ranked {
		if sf.Score < 0.9*ranked[0].Score {
			break
		}
		covers := true
		for _, term := range queryTerms {
			if df[term] > 0 && !hitCoversTerm(sf, term) {
				covers = false
				break
			}
		}
		if covers {
			return true
		}
	}
	return false
}

// topStandsOut reports whether the best score is clearly separated from the
// middle of the ranked tail. One or two results are specific by construction.
func topStandsOut(ranked []*scoredFile) bool {
	if len(ranked) <= 2 {
		return true
	}
	median := ranked[len(ranked)/2].Score
	if median <= 0 {
		return true
	}
	return ranked[0].Score >= 1.5*median
}

// corroboratedTopicURLs returns validated URLs for topics whose canonical docs
// appear among the ranked results, deduplicated and capped to maxURLs.
func corroboratedTopicURLs(topicMatches []TopicEntry, ranked []*scoredFile, validate func(string) string, maxURLs int) []DocumentationURL {
	var out []DocumentationURL
	seen := make(map[string]bool)
	for _, tm := range topicMatches {
		if !topicCorroborated(tm, ranked) {
			continue
		}
		for _, u := range tm.URLs {
			if len(out) >= maxURLs {
				return out
			}
			if seen[u] || validate(u) == "" {
				continue
			}
			seen[u] = true
			out = append(out, DocumentationURL{Title: tm.Name, URL: u, Type: "topic"})
		}
	}
	return out
}

const maxOutOfScopePointers = 3

// outOfScopePointers returns leads for matched topics whose canonical docs do
// not appear among the ranked results: the document's real H1 title, its
// relative path, and its URL. Deduplicated against the corroborated citation
// list and capped (#25).
func outOfScopePointers(homeDir string, topicMatches []TopicEntry, ranked []*scoredFile, guidance *AgentGuidance) []DocumentationURL {
	seen := make(map[string]bool)
	if guidance != nil {
		for _, doc := range guidance.DocumentationURLs {
			seen[doc.URL] = true
		}
	}
	registry := GetURLRegistry(homeDir)
	var out []DocumentationURL
	for _, tm := range topicMatches {
		if topicCorroborated(tm, ranked) {
			continue
		}
		for _, rel := range tm.CanonicalDocs {
			if len(out) >= maxOutOfScopePointers {
				return out
			}
			url := constructDocURL(rel)
			if url == "" || seen[url] || registry.ValidateURL(url) == "" {
				continue
			}
			seen[url] = true
			out = append(out, DocumentationURL{
				Title: documentTitle(filepath.Join(homeDir, rel), rel),
				Path:  rel,
				URL:   url,
				Type:  "pointer",
			})
		}
	}
	return out
}

func topicCorroborated(tm TopicEntry, ranked []*scoredFile) bool {
	for _, doc := range tm.CanonicalDocs {
		for _, sf := range ranked {
			if strings.Contains(sf.RelPath, doc) {
				return true
			}
		}
	}
	return false
}

func keysSortedV2(m map[string]FacetCounts) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

//
// ----------------------- Repo-aware helpers ----------------------
//

// SimpleClassifier returns a default path-based section classifier.
// exampleRoots are optional paths outside homeDir that should be classified as "examples".
func SimpleClassifier(homeDir string, exampleRoots []string) func(rel string, absPath string) string {
	paths := GetRepoPaths(homeDir)

	// Normalize paths for prefix matching
	componentDocs := strings.ReplaceAll(paths.ComponentDocs, "\\", "/") + "/"
	componentSource := strings.ReplaceAll(paths.ComponentSource, "\\", "/") + "/"
	pagesPath := strings.ReplaceAll(paths.Pages, "\\", "/") + "/"
	howtoPath := strings.ReplaceAll(paths.Howto, "\\", "/") + "/"
	blogPath := strings.ReplaceAll(paths.Blog, "\\", "/") + "/"
	extensionDocs := strings.ReplaceAll(paths.ExtensionDocs, "\\", "/") + "/"

	// Normalize example roots for comparison
	normalizedExampleRoots := make([]string, len(exampleRoots))
	for i, root := range exampleRoots {
		normalizedExampleRoots[i] = filepath.Clean(root)
	}

	return func(rel string, absPath string) string {
		r := strings.ReplaceAll(rel, "\\", "/")
		switch {
		case strings.HasPrefix(r, howtoPath):
			return "howtos"
		case strings.HasPrefix(r, componentDocs):
			return "components"
		case strings.HasPrefix(r, extensionDocs):
			return "components"
		case strings.HasPrefix(r, pagesPath):
			return "components"
		case strings.HasPrefix(r, componentSource):
			return "source"
		case strings.HasPrefix(r, blogPath):
			return "blog"
		}

		// Check if the absolute path is within any example root
		if absPath != "" {
			absPathClean := filepath.Clean(absPath)
			for _, exampleRoot := range normalizedExampleRoots {
				relToExample, err := filepath.Rel(exampleRoot, absPathClean)
				if err == nil && !strings.HasPrefix(relToExample, "..") {
					return "examples"
				}
			}
		}

		return "unknown"
	}
}

// DefaultStopwords provides a conservative set; you can override in cfg.
func DefaultStopwords() map[string]struct{} {
	stopwords := map[string]struct{}{
		"example": {}, "examples": {}, "usage": {}, "working": {}, "actual": {}, "real": {}, "when": {},
	}
	// English function words. Left in queries they saturate term coverage,
	// document frequency, and the filename-match signal (#11): "a"/"the"/
	// "with"/"from" match nearly every document and many filenames.
	for _, w := range []string{
		"a", "an", "the", "and", "or", "but",
		"with", "without", "from", "into", "onto", "until", "while", "where",
		"how", "what", "which", "that", "this", "these", "those",
		"is", "are", "was", "were", "be", "been", "being",
		"has", "have", "had", "do", "does", "did",
		"can", "could", "should", "would", "will", "shall", "may", "might", "must",
		"to", "of", "in", "on", "at", "by", "for", "as",
		"it", "its", "i", "we", "you", "they",
		"my", "our", "your", "their", "me", "us", "them", "like",
	} {
		stopwords[w] = struct{}{}
	}
	return stopwords
}

// DefaultSynonyms provides minimal, generic expansions; override if desired.
func DefaultSynonyms() map[string][]string {
	return map[string][]string{}
}

// detectFeatureCombination identifies when query asks for combining features that aren't documented together
func detectFeatureCombination(queryTokens []string, sections map[string][]resultItem) bool {
	if len(queryTokens) < 2 {
		return false // Single feature queries can't be combinations
	}

	// Check if any single result shows multiple query tokens together
	for _, items := range sections {
		for _, item := range items {
			tokensInSameResult := 0
			snippet := strings.ToLower(item.Snippet)
			for _, token := range queryTokens {
				if strings.Contains(snippet, strings.ToLower(token)) {
					tokensInSameResult++
				}
			}
			if tokensInSameResult >= 2 {
				return false // Found tokens together in same result = safe
			}
		}
	}
	return true // Never found tokens together = risky combination
}

// detectSyntaxInventionRisk identifies scenarios with high risk of syntax invention
func detectSyntaxInventionRisk(queryTokens []string, facets map[string]FacetCounts) bool {
	// Risk factors (generic patterns, not domain-specific)
	riskFactors := 0

	// Factor 1: Multiple technical terms (likely asking about combining features)
	if len(queryTokens) >= 2 {
		riskFactors++
	}

	// Factor 2: Low documentation coverage
	totalDocs := 0
	for _, facet := range facets {
		totalDocs += facet.Files
	}
	if totalDocs < 3 {
		riskFactors++
	}

	// Factor 3: No examples/howtos (implementation guidance missing)
	if facets["examples"].Files == 0 && facets["howtos"].Files == 0 {
		riskFactors++
	}

	return riskFactors >= 2
}

// generateAgentGuidance provides focused guidance prioritizing tool redirection
// Provides concise, actionable guidance without excessive repetition
func generateAgentGuidance(toolName string, confidence string, facets map[string]FacetCounts, sections map[string][]resultItem, originalQuery string, queryTokens []string, homeDir string) *AgentGuidance {
	// Concise base guidance - always included
	baseGuidance := []string{
		"Cite sources with file paths and URLs",
		"Provide URLs from documentation_urls when available",
	}

	// Calculate total hits
	totalHits := 0
	for _, fc := range facets {
		totalHits += fc.Files + fc.Matches
	}

	// PRIORITY 1: No results at all - concise failure guidance only
	// Don't include base guidance (cite sources/URLs) when there are no results to cite
	if totalHits == 0 {
		return generateFailureGuidance(toolName, originalQuery, nil, queryTokens)
	}

	// PRIORITY 2: Feature Combination Risk
	if detectFeatureCombination(queryTokens, sections) {
		guidance := &AgentGuidance{
			RuleReminders:     append(baseGuidance, "Verify features work together in a single example"),
			URLBase:           constructURLBase(),
			DocumentationURLs: extractDocumentationURLs(sections, confidence),
			SuggestedApproach: "Search for examples showing the complete pattern",
		}
		return guidance
	}

	// Initialize guidance for successful searches
	guidance := &AgentGuidance{
		RuleReminders:     baseGuidance,
		URLBase:           constructURLBase(),
		DocumentationURLs: extractDocumentationURLs(sections, confidence),
	}

	// PRIORITY 3: Query type mismatch - example query without examples
	if isExampleQuery(originalQuery) && facets["examples"].Files == 0 {
		guidance.RuleReminders = append(guidance.RuleReminders, "Try xmlui_examples tool")
		guidance.SearchToolPreference = "xmlui_examples"
		return guidance
	}

	// PRIORITY 4: Query type mismatch - how-to query without tutorials
	if isHowToQuery(originalQuery) && facets["howtos"].Files == 0 {
		guidance.RuleReminders = append(guidance.RuleReminders, "Try xmlui_search_howto tool")
		guidance.SearchToolPreference = "xmlui_search_howto"
		return guidance
	}

	// PRIORITY 5: Low confidence scenarios
	if confidence == "low" {
		guidance.SuggestedApproach = "Verify feature exists in documentation"
		return guidance
	}

	// Default: successful search with good results
	return guidance
}

// isHowToQuery detects queries asking for how-to instructions
func isHowToQuery(query string) bool {
	lq := strings.ToLower(query)
	howToPatterns := []string{
		"how to", "how do", "how can", "how should", "how would",
		"tutorial", "guide", "step by step", "instructions",
		"walkthrough", "demonstration",
	}
	for _, pattern := range howToPatterns {
		if strings.Contains(lq, pattern) {
			return true
		}
	}
	return false
}

// isExampleQuery detects queries asking for examples
func isExampleQuery(query string) bool {
	lq := strings.ToLower(query)
	examplePatterns := []string{
		"example", "examples", "demo", "sample", "show me",
		"working example", "code example", "usage example",
	}
	for _, pattern := range examplePatterns {
		if strings.Contains(lq, pattern) {
			return true
		}
	}
	return false
}

func constructURLBase() string {
	return baseURL
}

// constructDocumentationURL converts a file path to a clickable documentation URL.
// Returns empty string if no URL can be constructed.
func constructDocumentationURL(filePath string, lineNum int, baseURL string) string {
	return constructDocURL(filePath)
}

// citationScoreFloor drops tail results from the citation footer: only
// results scoring at least this fraction of the top score are cited (#20).
const citationScoreFloor = 0.4

// extractDocumentationURLs extracts citation URLs from found documentation
// sections, gated on the evidence the ranking already produced (#20): nothing
// is cited at low confidence (the tool just said it found nothing worth
// citing), tail results below the relevance floor are dropped, the list is
// capped, and order is deterministic (score then path — the sections map's
// iteration order is not).
func extractDocumentationURLs(sections map[string][]resultItem, confidence string) []DocumentationURL {
	if confidence == "low" {
		return []DocumentationURL{}
	}

	type citation struct {
		item    resultItem
		section string
	}
	best := make(map[string]citation)
	for sectionName, items := range sections {
		for _, item := range items {
			url := constructDocumentationURL(item.Path, item.Line, constructURLBase())
			if url == "" {
				continue
			}
			if current, exists := best[url]; !exists || item.Score > current.item.Score {
				best[url] = citation{item: item, section: sectionName}
			}
		}
	}

	ranked := make([]citation, 0, len(best))
	topScore := 0.0
	for _, c := range best {
		ranked = append(ranked, c)
		if c.item.Score > topScore {
			topScore = c.item.Score
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].item.Score == ranked[j].item.Score {
			return ranked[i].item.Path < ranked[j].item.Path
		}
		return ranked[i].item.Score > ranked[j].item.Score
	})

	urls := []DocumentationURL{}
	for _, c := range ranked {
		if len(urls) >= maxDocumentationURLs {
			break
		}
		if c.item.Score < topScore*citationScoreFloor {
			break
		}
		urls = append(urls, DocumentationURL{
			Title: documentTitle(c.item.AbsPath, c.item.Path),
			URL:   constructDocumentationURL(c.item.Path, c.item.Line, constructURLBase()),
			Type:  c.section,
		})
	}

	return urls
}

// documentTitleCache memoizes first-heading lookups per absolute path.
var (
	documentTitleMu    sync.Mutex
	documentTitleCache = map[string]string{}
)

// documentTitle returns the document's real H1 heading, falling back to the
// slug-derived transform only when the file has no heading or cannot be read.
// Title-casing the slug fabricates titles that appear nowhere in the corpus
// and mangles acronyms ("Qr", "Contentseparator") — citation labels must be
// the document's own words (#23).
func documentTitle(absPath, relPath string) string {
	if absPath != "" {
		documentTitleMu.Lock()
		cached, seen := documentTitleCache[absPath]
		documentTitleMu.Unlock()
		if !seen {
			cached = readFirstHeading(absPath)
			documentTitleMu.Lock()
			documentTitleCache[absPath] = cached
			documentTitleMu.Unlock()
		}
		if cached != "" {
			return cached
		}
	}
	return extractTitleFromPath(relPath)
}

// readFirstHeading returns the first "# " heading near the top of the file.
func readFirstHeading(absPath string) string {
	f, err := os.Open(absPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for lines := 0; scanner.Scan() && lines < 50; lines++ {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimSpace(line[2:])
			// Drop trailing anchor markup like "[#layout-summary]" — it is
			// site plumbing, not part of the document's title.
			if idx := strings.LastIndex(title, "[#"); idx > 0 && strings.HasSuffix(title, "]") {
				title = strings.TrimSpace(title[:idx])
			}
			return title
		}
	}
	return ""
}

// extractTitleFromPath extracts a human-readable title from a file path
func extractTitleFromPath(filePath string) string {
	base := filepath.Base(filePath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	// Convert kebab-case to title case
	words := strings.Split(name, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// generateFailureGuidance provides specific guidance when no results are found
func generateFailureGuidance(toolName string, originalQuery string, queryPlan []stageHit, kept []string) *AgentGuidance {
	guidance := &AgentGuidance{
		RuleReminders: []string{"No documentation found for this query"},
	}

	// Special cases that should redirect to specific tools — but never to the
	// tool that produced this response (#23 polish).
	if isHowToQuery(originalQuery) && toolName != "xmlui_search_howto" {
		guidance.SuggestedApproach = "Try xmlui_list_howto or xmlui_search_howto"
		guidance.SearchToolPreference = "xmlui_list_howto"
		return guidance
	}

	if isExampleQuery(originalQuery) && toolName != "xmlui_examples" {
		guidance.SuggestedApproach = "Try xmlui_examples with simpler terms"
		guidance.SearchToolPreference = "xmlui_examples"
		return guidance
	}

	// General guidance for other failed searches
	guidance.SuggestedApproach = "Try simpler search terms or use " + strings.Join(otherSearchTools(toolName), "/")
	return guidance
}

// dedupeStrings removes duplicates preserving first-seen order.
func dedupeStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// otherSearchTools lists the sibling search tools an agent can pivot to,
// excluding the tool that produced this response — a tool advising the agent
// to use itself reads as a dead end (#23 polish).
func otherSearchTools(toolName string) []string {
	all := []string{"xmlui_examples", "xmlui_search_howto", "xmlui_search"}
	out := make([]string, 0, len(all))
	for _, t := range all {
		if t != toolName {
			out = append(out, t)
		}
	}
	return out
}

// termStem trims common English suffixes so morphological variants corroborate
// a term ("linking" matches a doc that says "deep link"). Conservative: only
// applies when the stem keeps at least 4 characters.
func termStem(term string) string {
	for _, suffix := range []string{"ing", "ed", "es", "s"} {
		if strings.HasSuffix(term, suffix) && len(term)-len(suffix) >= 4 {
			return term[:len(term)-len(suffix)]
		}
	}
	return term
}

// hitCoversTerm reports whether a ranked hit answers a query term, either via
// the exact-term tracking from scoring or via the term's stem appearing in the
// hit's filename or snippets.
func hitCoversTerm(sf *scoredFile, term string) bool {
	if sf.TermsFound[term] {
		return true
	}
	stem := termStem(term)
	if filenameMatchesTerm(sf.RelPath, term) {
		return true
	}
	if stem == term {
		return false
	}
	for _, snip := range sf.Snippets {
		if strings.Contains(strings.ToLower(snip.Text), stem) {
			return true
		}
	}
	return false
}

// filenameMatchesTerm reports whether a query term matches the filename at a
// token boundary, stem-aware ("linking" matches a "link" slug token).
// Substring matching let "form" match "transform" and bought rank for slug
// coincidences (#27); every filename-match signal routes through here so the
// bonus, the coverage fallback, and the title counts agree.
func filenameMatchesTerm(filename, term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	stem := termStem(term)
	if len(stem) < 4 {
		return false
	}
	base := strings.ToLower(filepath.Base(filename))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	tokens := strings.FieldsFunc(base, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for _, token := range tokens {
		if token == term || token == stem || termStem(token) == stem {
			return true
		}
	}
	return false
}

// termDocumentFrequency counts, per query term, how many ranked candidates
// answer it (hitCoversTerm basis). minDF is the lowest nonzero count — the
// document frequency of the query's most distinctive achievable term(s).
func termDocumentFrequency(ranked []*scoredFile, queryTerms []string) (map[string]int, int) {
	df := make(map[string]int, len(queryTerms))
	minDF := 0
	for _, term := range queryTerms {
		count := 0
		for _, sf := range ranked {
			if hitCoversTerm(sf, term) {
				count++
			}
		}
		df[term] = count
		if count > 0 && (minDF == 0 || count < minDF) {
			minDF = count
		}
	}
	return df, minDF
}

// computeSalience summarizes how the ranked candidates answer the query's
// distinctive terms. TermCoverage is the authoritative per-term truth; Terms
// is the distinctive band (all substantive terms minus in-corpus-generic
// ones), and the aggregate counts are candidate-level unions over that band —
// derived from the same per-term basis as TermCoverage, so they cannot
// contradict it (#14). Title vs content coverage of the band separates a
// discoverability gap (a doc answers, but no title says so) from a content
// gap (nothing answers) in the analytics record (#12).
func computeSalience(ranked []*scoredFile, queryTerms []string) SalienceSummary {
	summary := SalienceSummary{Terms: []string{}, UnansweredTerms: []string{}, TermCoverage: []TermCoverageEntry{}}
	df, _ := termDocumentFrequency(ranked, queryTerms)
	seenUnanswered := make(map[string]bool)
	for _, term := range queryTerms {
		if df[term] == 0 && len(termStem(term)) >= 4 && !seenUnanswered[term] {
			seenUnanswered[term] = true
			summary.UnansweredTerms = append(summary.UnansweredTerms, term)
		}
	}
	seenCoverage := make(map[string]bool)
	for _, term := range queryTerms {
		stem := termStem(term)
		if len(stem) < 4 || seenCoverage[term] {
			continue
		}
		seenCoverage[term] = true
		entry := TermCoverageEntry{Term: term, ContentMatches: df[term]}
		for _, sf := range ranked {
			if filenameMatchesTerm(sf.RelPath, term) {
				entry.TitleMatches++
			}
		}
		summary.TermCoverage = append(summary.TermCoverage, entry)
	}
	// Distinctive band: all substantive terms except in-corpus-generic ones
	// (content coverage >= ~2/3 of the candidates, applied only when there
	// are >=3 candidates so tiny result sets don't over-exclude). Zero-
	// coverage terms are included — their absence is the content-gap tell,
	// and the high guard does not read this list, so absent terms are safe
	// here. Rarity is no longer the selection key (#14).
	genericThreshold := -1
	if len(ranked) >= 3 {
		genericThreshold = (2*len(ranked) + 2) / 3
	}
	seenBand := make(map[string]bool)
	for _, term := range queryTerms {
		if len(termStem(term)) < 4 || seenBand[term] {
			continue
		}
		seenBand[term] = true
		if genericThreshold > 0 && df[term] >= genericThreshold {
			continue
		}
		summary.Terms = append(summary.Terms, term)
	}

	// Aggregates are candidate-level unions over the band.
	for _, sf := range ranked {
		content := false
		title := false
		for _, term := range summary.Terms {
			if hitCoversTerm(sf, term) {
				content = true
			}
			if filenameMatchesTerm(sf.RelPath, term) {
				title = true
			}
		}
		if content {
			summary.ContentMatchCount++
		}
		if title {
			summary.TitleMatchCount++
		}
	}
	return summary
}
