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

	// File extensions to scan.
	FileExtensions []string // default: .mdx, .md, .tsx, .scss

	// Optional: tokens to drop when relaxing queries.
	Stopwords map[string]struct{}

	// Optional: synonyms expansion (token → []alternatives or phrases).
	Synonyms map[string][]string

	// Optional: related query suggestions.
	RelatedFunc func(original string, kept []string) []string

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

// AgentGuidance provides rule reminders and suggestions for low-confidence scenarios
type AgentGuidance struct {
	RuleReminders     []string `json:"rule_reminders"`
	SuggestedApproach string   `json:"suggested_approach"`
}

// MediatorJSON is the machine-readable summary we append after the human block.
type MediatorJSON struct {
	QueryPlan      []stageHit                     `json:"query_plan"`
	Tokens         map[string][]string            `json:"tokens"` // kept/removed/expanded
	Sections       map[string][]resultItem        `json:"sections"`
	Facets         map[string]FacetCounts         `json:"facets"`
	Confidence     string                         `json:"confidence"`
	RelatedQueries []string                       `json:"related_queries"`
	AgentGuidance  *AgentGuidance                 `json:"agent_guidance,omitempty"`
	Diagnostics    map[string]any                 `json:"diagnostics,omitempty"`
}

// ExecuteMediatedSearch runs the staged scan and returns:
//  1) human readable block,
//
//	2) JSON summary (also included at the end of the human block),
//	3) error if something goes wrong (I/O etc. are soft-failed inside).
func ExecuteMediatedSearch(homeDir string, cfg MediatorConfig, originalQuery string) (string, MediatorJSON, error) {
	// defaults
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = 50
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
	results := []string{}         // human-visible lines
	seen := map[string]struct{}{} // dedupe key: path:line:text
	uniqueFiles := make(map[string]map[string]struct{}) // section -> set of file paths

	jsonOut := MediatorJSON{
		QueryPlan:  []stageHit{},
		Tokens:     map[string][]string{"kept": {}, "removed": {}, "expanded": {}},
		Sections:   make(map[string][]resultItem),
		Facets:     make(map[string]FacetCounts),
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
		results = append(results, fmt.Sprintf("%s:%d: %s", rel, lineNum, line))

		section := cfg.Classifier(rel)
		if _, ok := jsonOut.Sections[section]; !ok {
			// unknown section -> create on the fly so we don't lose hits
			jsonOut.Sections[section] = []resultItem{}
			uniqueFiles[section] = make(map[string]struct{})
		}
		jsonOut.Sections[section] = append(jsonOut.Sections[section], resultItem{
			Type: section, Path: rel, Line: lineNum, Snippet: line,
		})
		// Track unique file for this section
		uniqueFiles[section][rel] = struct{}{}
	}

	runStage := func(stageName, stageQuery string, roots []string) int {
		hits := 0
		if stageQuery == "" {
			jsonOut.QueryPlan = append(jsonOut.QueryPlan, stageHit{Stage: stageName, Query: stageQuery, Hits: 0})
			return 0
		}
		lq := strings.ToLower(stageQuery)

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
				if cfg.EnableFilenameMatches && fuzzyMatch(d.Name(), lq) {
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
					if fuzzyMatch(line, lq) {
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
	totalHits += runStage("exact", strings.ToLower(originalQuery), cfg.Roots)

	// Stage 2: relaxed (strip sigils/stopwords)
	kept, removed := normalizeTokens(originalQuery, cfg.Stopwords)
	jsonOut.Tokens["kept"] = kept
	jsonOut.Tokens["removed"] = removed
	if len(kept) > 0 && len(results) < cfg.MaxResults {
		relaxed := strings.Join(kept, " ")
		totalHits += runStage("relaxed", relaxed, cfg.Roots)
	}

	// Stage 3: synonyms; bias preferred sections if the tokens look like a concept
	if len(kept) > 0 && len(results) < cfg.MaxResults {
		expanded := expandSynonyms(kept, cfg.Synonyms)
		jsonOut.Tokens["expanded"] = expanded

		if len(expanded) > 0 {
			expandedQ := strings.Join(expanded, " ")

			roots := cfg.Roots
			if looksLikeConcept(kept) && len(cfg.PreferSections) > 0 {
				// re-order roots so preferred sections' paths come first
				roots = reorderRootsByPreference(cfg.Roots, cfg.PreferSections)
			}
			totalHits += runStage("synonyms", expandedQ, roots)
		}
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

	// Agent guidance for low-confidence scenarios
	jsonOut.AgentGuidance = generateAgentGuidance(jsonOut.Confidence, jsonOut.Facets, originalQuery)

	// Related queries
	if cfg.RelatedFunc != nil {
		jsonOut.RelatedQueries = cfg.RelatedFunc(originalQuery, kept)
	} else {
		jsonOut.RelatedQueries = defaultRelated(originalQuery, kept)
	}

	// Human block
	var out strings.Builder
	if len(results) == 0 {
		blob, _ := json.MarshalIndent(jsonOut, "", "  ")
		out.WriteString("No matches found.\n\n---\nJSON:\n")
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

	for i, line := range results {
		if i >= cfg.MaxResults {
			break
		}
		out.WriteString(line + "\n")
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

// expandSynonyms: token-wise, include simple alternates/phrases.
func expandSynonyms(tokens []string, syn map[string][]string) (expanded []string) {
	seen := map[string]struct{}{}
	for _, t := range tokens {
		expanded = append(expanded, t)
		if alts, ok := syn[t]; ok {
			for _, a := range alts {
				for _, at := range strings.Fields(strings.ToLower(a)) {
					if _, dup := seen[at]; !dup {
						expanded = append(expanded, at)
						seen[at] = struct{}{}
					}
				}
			}
		}
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
		case strings.HasPrefix(r, "docs/public/pages/"):
			if strings.Contains(r, "howto") {
				return "howtos"
			}
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

// defaultRelated gives generic follow-ups when a custom RelatedFunc isn't provided.
func defaultRelated(original string, kept []string) []string {
	out := []string{}
	if len(kept) > 0 {
		out = append(out, strings.Join(kept, " "))
	}
	return out
}

// generateAgentGuidance provides rule reminders and guidance for low-confidence scenarios
func generateAgentGuidance(confidence string, facets map[string]FacetCounts, originalQuery string) *AgentGuidance {
	// Only provide guidance for low-confidence or problematic scenarios
	if confidence != "low" && !isProblematicQuery(facets, originalQuery) {
		return nil
	}

	guidance := &AgentGuidance{
		RuleReminders: []string{},
	}

	// Analyze the scenario and provide appropriate reminders
	totalFiles := 0
	totalMatches := 0
	for _, fc := range facets {
		totalFiles += fc.Files
		totalMatches += fc.Matches
	}

	// No matches at all - strongest rule reminder
	if totalFiles == 0 && totalMatches == 0 {
		guidance.RuleReminders = []string{
			"Do not invent XMLUI syntax - only use documented constructs",
			"Always do the simplest thing possible",
			"Preview and discuss limitations before providing code",
		}
		guidance.SuggestedApproach = "Low confidence suggests this feature may not exist. Acknowledge the limitation rather than inventing syntax."
		return guidance
	}

	// Low confidence with minimal coverage
	if confidence == "low" {
		guidance.RuleReminders = []string{
			"Do not invent XMLUI syntax - only use documented constructs",
			"Preview and discuss limitations before providing code",
		}
		if totalFiles <= 1 {
			guidance.SuggestedApproach = "Limited documentation found. Verify feature exists before providing implementation details."
		} else {
			guidance.SuggestedApproach = "Mixed signals in documentation. Cross-reference sources and acknowledge uncertainties."
		}
		return guidance
	}

	// Syntax-heavy queries without good coverage
	if isSyntaxQuery(originalQuery) && (facets["examples"].Files == 0 && facets["howtos"].Files == 0) {
		guidance.RuleReminders = []string{
			"Do not invent XMLUI syntax - only use documented constructs",
			"Always cite your sources when providing code examples",
		}
		guidance.SuggestedApproach = "No examples or tutorials found. Only provide syntax that you can cite from component documentation."
		return guidance
	}

	return guidance
}

// isProblematicQuery detects scenarios that warrant agent guidance even with medium confidence
func isProblematicQuery(facets map[string]FacetCounts, query string) bool {
	// Syntax queries without examples/howtos
	if isSyntaxQuery(query) {
		return facets["examples"].Files == 0 && facets["howtos"].Files == 0
	}
	// "How to" queries without howtos
	if strings.Contains(strings.ToLower(query), "how to") || strings.Contains(strings.ToLower(query), "how do") {
		return facets["howtos"].Files == 0
	}
	return false
}

// isSyntaxQuery detects queries that are asking for specific syntax
func isSyntaxQuery(query string) bool {
	lq := strings.ToLower(query)
	// Look for syntax-indicating patterns
	syntaxPatterns := []string{
		"<", ">", "=", "align", "style", "property", "attribute",
		"bindto", "onclick", "textsize", "color", "width", "height",
	}
	for _, pattern := range syntaxPatterns {
		if strings.Contains(lq, pattern) {
			return true
		}
	}
	return false
}
