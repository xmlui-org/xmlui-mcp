package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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

	// File extensions to scan.
	FileExtensions []string // default: .mdx, .md, .tsx, .scss

	// Optional: tokens to drop when relaxing queries.
	Stopwords map[string]struct{}

	// Optional: synonyms expansion (token → []alternatives or phrases).
	Synonyms map[string][]string

	// Optional: per-hit classifier (relpath -> section key). If nil, SimpleClassifier() is used.
	Classifier func(rel string) string

	// Optional: enable filename matches (per your legacy behavior). Default true.
	EnableFilenameMatches bool
}

// FacetCounts represents both match counts and unique file counts for a section
type FacetCounts struct {
	Files   int `json:"files"`   // unique files with matches
	Matches int `json:"matches"` // total matching lines
}

// DocumentationURL represents a specific documentation link
type DocumentationURL struct {
	Title string `json:"title"`
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

// MediatorJSON is the machine-readable summary we append after the human block.
type MediatorJSON struct {
	QueryPlan           []stageHit              `json:"query_plan"`
	Tokens              map[string][]string     `json:"tokens"` // kept/removed/expanded
	Sections            map[string][]resultItem `json:"sections"`
	Facets              map[string]FacetCounts  `json:"facets"`
	Confidence          string                  `json:"confidence"`
	RelatedQueries      []string                `json:"related_queries"`
	AgentGuidance       *AgentGuidance          `json:"agent_guidance,omitempty"`
	Diagnostics         map[string]any          `json:"diagnostics,omitempty"`
	SearchToolHierarchy []string                `json:"search_tool_hierarchy,omitempty"`
}

// ExecuteMediatedSearch runs the staged scan and returns:
//
//  1. human readable block,
//
//  2. JSON summary (also included at the end of the human block),
//
//  3. error if something goes wrong (I/O etc. are soft-failed inside).
func ExecuteMediatedSearch(homeDir string, cfg MediatorConfig, originalQuery string) (string, MediatorJSON, error) {
	// defaults
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = 50
	}
	if cfg.MaxSnippetLength <= 0 {
		cfg.MaxSnippetLength = 200
	}
	if len(cfg.FileExtensions) == 0 {
		cfg.FileExtensions = []string{".mdx", ".md", ".tsx", ".scss"}
	}
	if cfg.Classifier == nil {
		cfg.Classifier = SimpleClassifier(homeDir)
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
	results := []string{}                               // human-visible lines
	seen := map[string]struct{}{}                       // dedupe key: path:line:text
	uniqueFiles := make(map[string]map[string]struct{}) // section -> set of file paths

	jsonOut := MediatorJSON{
		QueryPlan: []stageHit{},
		Tokens:    map[string][]string{"kept": {}, "removed": {}, "expanded": {}},
		Sections:  make(map[string][]resultItem),
		Facets:    make(map[string]FacetCounts),
		Diagnostics: map[string]any{
			"original_query": strings.TrimSpace(originalQuery),
		},
	}

	// Initialize sections and file tracking for stable ordering
	for _, k := range cfg.SectionKeys {
		jsonOut.Sections[k] = []resultItem{}
		uniqueFiles[k] = make(map[string]struct{})
	}

	// -------- helpers --------

	addHit := func(rel string, lineNum int, line string) {
		if len(results) >= cfg.MaxResults {
			return
		}
		key := fmt.Sprintf("%s:%d:%s", rel, lineNum, line)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}

		// Truncate snippet if too long
		snippet := line
		if len(snippet) > cfg.MaxSnippetLength {
			snippet = snippet[:cfg.MaxSnippetLength] + "..."
		}

		results = append(results, fmt.Sprintf("%s:%d: %s", rel, lineNum, snippet))

		section := cfg.Classifier(rel)
		if _, ok := jsonOut.Sections[section]; !ok {
			// unknown section -> create on the fly so we don't lose hits
			jsonOut.Sections[section] = []resultItem{}
			uniqueFiles[section] = make(map[string]struct{})
		}
		jsonOut.Sections[section] = append(jsonOut.Sections[section], resultItem{
			Type: section, Path: rel, Line: lineNum, Snippet: snippet,
		})
		// Track unique file for this section
		uniqueFiles[section][rel] = struct{}{}
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

				// filename match (optional)
				var matchFunc func(string, string) bool
				if usePartialMatch {
					matchFunc = func(text, query string) bool {
						return partialMatch(text, query, minWords)
					}
				} else {
					matchFunc = fuzzyMatch
				}

				if cfg.EnableFilenameMatches && matchFunc(d.Name(), lq) {
					rel, _ := filepath.Rel(homeDir, path)
					addHit(rel, 0, "[filename match]")
					hits++
					if len(results) >= cfg.MaxResults {
						return nil
					}
				}

				f, err := os.Open(path)
				if err != nil {
					return nil
				}
				defer f.Close()

				sc := bufio.NewScanner(f)
				ln := 1
				for sc.Scan() {
					line := sc.Text()
					if matchFunc(line, lq) {
						rel, _ := filepath.Rel(homeDir, path)
						addHit(rel, ln, line)
						hits++
						if len(results) >= cfg.MaxResults {
							return nil
						}
					}
					ln++
				}
				return nil
			})
			if len(results) >= cfg.MaxResults {
				break
			}
		}
		jsonOut.QueryPlan = append(jsonOut.QueryPlan, stageHit{Stage: stageName, Query: lq, Hits: hits})
		return hits
	}

	// -------- stages --------

	totalHits := 0

	// Stage 1: exact (legacy behavior)
	totalHits += runStage("exact", strings.ToLower(originalQuery), cfg.Roots, false)

	// Stage 2: relaxed (strip sigils/stopwords)
	kept, removed := normalizeTokens(originalQuery, cfg.Stopwords)
	jsonOut.Tokens["kept"] = kept
	jsonOut.Tokens["removed"] = removed
	if len(kept) > 0 && len(results) < cfg.MaxResults {
		relaxed := strings.Join(kept, " ")
		totalHits += runStage("relaxed", relaxed, cfg.Roots, false)
	}

	// Stage 3: partial matching (relaxed word requirements)
	if len(kept) > 0 && len(results) < cfg.MaxResults {
		relaxed := strings.Join(kept, " ")
		roots := cfg.Roots
		if looksLikeConcept(kept) && len(cfg.PreferSections) > 0 {
			// re-order roots so preferred sections' paths come first
			roots = reorderRootsByPreference(cfg.Roots, cfg.PreferSections)
		}
		totalHits += runStage("partial", relaxed, roots, true)

		// Update tokens to show we used partial matching
		jsonOut.Tokens["expanded"] = kept // Show what we searched with partial matching
	}

	// Build facets with both file counts and match counts
	for k := range jsonOut.Sections {
		jsonOut.Facets[k] = FacetCounts{
			Files:   len(uniqueFiles[k]),
			Matches: len(jsonOut.Sections[k]),
		}
	}

	// Confidence heuristic (updated to use new facet structure)
	jsonOut.Confidence = confidenceHeuristicV2(jsonOut.Facets, totalHits)

	// Set search tool hierarchy for howto/example queries
	if isHowToQuery(originalQuery) || isExampleQuery(originalQuery) {
		jsonOut.SearchToolHierarchy = []string{
			"xmlui_examples (preferred)",
			"xmlui_search_howto (preferred)",
			"xmlui_search (fallback)",
		}
	}

	// Agent guidance for low-confidence scenarios
	jsonOut.AgentGuidance = generateAgentGuidance(jsonOut.Confidence, jsonOut.Facets, jsonOut.Sections, originalQuery, kept, homeDir)

	// Related queries - removed for now
	jsonOut.RelatedQueries = []string{}

	// Human block
	var out strings.Builder
	if len(results) == 0 {
		out.WriteString("No matches found.\n")

		// Only show guidance section if there's actionable guidance
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

		blob, _ := json.MarshalIndent(jsonOut, "", "  ")
		out.WriteString("\n---\nJSON:\n")
		out.Write(blob)
		return out.String(), jsonOut, nil
	}

	fmt.Fprintf(&out, "Query: %q  (stages=%d, hits=%d, confidence=%s)\n",
		originalQuery, len(jsonOut.QueryPlan), len(results), jsonOut.Confidence)
	fmt.Fprintf(&out, "Facets: ")
	keys := keysSortedV2(jsonOut.Facets)
	for i, k := range keys {
		if i > 0 {
			out.WriteString("  ")
		}
		facet := jsonOut.Facets[k]
		if facet.Files == 1 {
			fmt.Fprintf(&out, "%s=%d", k, facet.Matches)
		} else {
			fmt.Fprintf(&out, "%s=%d files (%d matches)", k, facet.Files, facet.Matches)
		}
	}
	out.WriteString("\n\n")

	// Limit displayed results to a reasonable number (e.g., 20 for readability)
	displayLimit := 20
	if displayLimit > len(results) {
		displayLimit = len(results)
	}

	for i := 0; i < displayLimit; i++ {
		out.WriteString(results[i] + "\n")
	}

	if len(results) > displayLimit {
		fmt.Fprintf(&out, "\n... %d more results omitted ...\n", len(results)-displayLimit)
	}

	blob, _ := json.MarshalIndent(jsonOut, "", "  ")
	out.WriteString("\n---\nJSON:\n")
	out.Write(blob)

	return out.String(), jsonOut, nil
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
		return totalWords     // 100% for 1-2 words
	case totalWords <= 4:
		return 2              // 50% for 3-4 words
	case totalWords >= 5:
		return 2              // Just 2 words for 5+ word queries
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
	Type    string `json:"type"` // section key
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
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
	// We don't know the section of a root directly. We use the typical XMLUI path layout:
	//   components → docs/content/components, docs/public/pages
	//   examples   → docs/src/components
	//   source     → xmlui/src/components
	// We'll just move roots that *contain* a preferred marker to the front (stable).
	type scored struct {
		root  string
		score int
	}
	scoreRoot := func(root string) int {
		r := strings.ReplaceAll(root, "\\", "/")
		score := 0
		for i, sec := range preferredSections {
			// lightweight mapping heuristic
			switch sec {
			case "components":
				if strings.Contains(r, "/docs/content/components") || strings.Contains(r, "/docs/public/pages") {
					score = max(score, 100-i)
				}
			case "howtos":
				if strings.Contains(r, "/docs/public/pages") && strings.Contains(r, "howto") {
					score = max(score, 100-i)
				}
			case "examples":
				if strings.Contains(r, "/docs/src/components") {
					score = max(score, 100-i)
				}
			case "source":
				if strings.Contains(r, "/xmlui/src/components") {
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

// Updated confidence heuristic for new facet structure
func confidenceHeuristicV2(facets map[string]FacetCounts, total int) string {
	if total == 0 {
		return "low"
	}
	// weight docs (components/howtos) higher based on both files and matches
	compFiles := facets["components"].Files
	compMatches := facets["components"].Matches
	howtoFiles := facets["howtos"].Files
	howtoMatches := facets["howtos"].Matches

	// High confidence if we have multiple files OR many matches in docs
	if (compFiles+howtoFiles) >= 2 || (compMatches+howtoMatches) > 5 {
		return "high"
	}
	return "medium"
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
func SimpleClassifier(homeDir string) func(rel string) string {
	home := filepath.Clean(homeDir)
	return func(rel string) string {
		// Expect rel paths already relative to homeDir.
		r := strings.ReplaceAll(rel, "\\", "/")
		switch {
		case strings.HasPrefix(r, "docs/content/components/"):
			return "components"
		case strings.HasPrefix(r, "docs/public/pages/howto/"):
			return "howtos"
		case strings.HasPrefix(r, "docs/public/pages/"):
			return "components"
		case strings.HasPrefix(r, "docs/src/components/"):
			return "examples"
		case strings.HasPrefix(r, "xmlui/src/components/"):
			return "source"
		default:
			_ = home // unused safeguard
			return "source"
		}
	}
}

// DefaultStopwords provides a conservative set; you can override in cfg.
func DefaultStopwords() map[string]struct{} {
	return map[string]struct{}{
		"example": {}, "examples": {}, "usage": {}, "working": {}, "actual": {}, "real": {}, "when": {},
	}
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
func generateAgentGuidance(confidence string, facets map[string]FacetCounts, sections map[string][]resultItem, originalQuery string, queryTokens []string, homeDir string) *AgentGuidance {
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
		return generateFailureGuidance(originalQuery, nil, queryTokens)
	}

	// PRIORITY 2: Feature Combination Risk
	if detectFeatureCombination(queryTokens, sections) {
		guidance := &AgentGuidance{
			RuleReminders:     append(baseGuidance, "Verify features work together in a single example"),
			URLBase:           constructURLBase(),
			DocumentationURLs: extractDocumentationURLs(sections, constructURLBase()),
			SuggestedApproach: "Search for examples showing the complete pattern",
		}
		return guidance
	}

	// Initialize guidance for successful searches
	guidance := &AgentGuidance{
		RuleReminders:     baseGuidance,
		URLBase:           constructURLBase(),
		DocumentationURLs: extractDocumentationURLs(sections, constructURLBase()),
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
	return "https://docs.xmlui.org"
}

// constructDocumentationURL converts a file path to a clickable documentation URL
func constructDocumentationURL(filePath string, lineNum int, baseURL string) string {
	// Convert file path to URL path
	urlPath := strings.ReplaceAll(filePath, "\\", "/")

	// Map file paths to URL patterns
	switch {
	case strings.HasPrefix(urlPath, "docs/content/components/"):
		// docs/content/components/Table.md -> /components/Table
		componentName := strings.TrimSuffix(filepath.Base(urlPath), ".md")
		return fmt.Sprintf("%s/components/%s", baseURL, componentName)
	case strings.HasPrefix(urlPath, "docs/public/pages/howto/"):
		// docs/public/pages/howto/paginate-a-list.md -> /howto/paginate-a-list
		howtoName := strings.TrimSuffix(filepath.Base(urlPath), ".md")
		return fmt.Sprintf("%s/howto/%s", baseURL, howtoName)
	case strings.HasPrefix(urlPath, "docs/public/pages/"):
		// docs/public/pages/components-intro.md -> /components-intro
		pageName := strings.TrimSuffix(filepath.Base(urlPath), ".md")
		return fmt.Sprintf("%s/%s", baseURL, pageName)
	default:
		// For other paths, provide a generic link with line reference
		return fmt.Sprintf("%s#%s:%d", baseURL, urlPath, lineNum)
	}
}

// extractDocumentationURLs extracts URLs from found documentation sections
func extractDocumentationURLs(sections map[string][]resultItem, baseURL string) []DocumentationURL {
	urls := []DocumentationURL{}
	seen := make(map[string]bool)

	for sectionName, items := range sections {
		for _, item := range items {
			url := constructDocumentationURL(item.Path, item.Line, baseURL)
			if !seen[url] {
				seen[url] = true
				urls = append(urls, DocumentationURL{
					Title: extractTitleFromPath(item.Path),
					URL:   url,
					Type:  sectionName,
				})
			}
		}
	}

	return urls
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
func generateFailureGuidance(originalQuery string, queryPlan []stageHit, kept []string) *AgentGuidance {
	guidance := &AgentGuidance{
		RuleReminders: []string{"No documentation found for this query"},
	}

	// Special cases that should redirect to specific tools
	if isHowToQuery(originalQuery) {
		guidance.SuggestedApproach = "Try xmlui_list_howto or xmlui_search_howto"
		guidance.SearchToolPreference = "xmlui_list_howto"
		return guidance
	}

	if isExampleQuery(originalQuery) {
		guidance.SuggestedApproach = "Try xmlui_examples with simpler terms"
		guidance.SearchToolPreference = "xmlui_examples"
		return guidance
	}

	// General guidance for other failed searches
	guidance.SuggestedApproach = "Try simpler search terms or use xmlui_examples/xmlui_search_howto"
	return guidance
}
