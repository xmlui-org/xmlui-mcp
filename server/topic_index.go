package server

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

// TopicEntry represents a topic extracted from doc headings that maps
// query terms to documentation resources.
type TopicEntry struct {
	Name          string   // The heading text (e.g. "Global Variables")
	TriggerTerms []string // Lowercase words from the heading
	CanonicalDocs []string // Relative doc paths that should get a score bonus
	URLs          []string // Direct documentation URLs
	Description   string   // The heading text (same as Name for auto-generated)
}

var (
	topicIndex     []TopicEntry
	topicIndexOnce sync.Once
	topicHomeDir   string
)

// initTopicIndex builds the topic index by scanning doc headings.
// Called lazily on first use via sync.Once.
func initTopicIndex(homeDir string) {
	topicHomeDir = homeDir
	topicIndexOnce.Do(func() {
		topicIndex = buildTopicIndex(homeDir)
	})
}

// ensureTopicIndex guarantees the topic index is initialized.
func ensureTopicIndex(homeDir string) {
	if topicIndex == nil {
		initTopicIndex(homeDir)
	}
}

// buildTopicIndex scans markdown files in the docs tree for headings
// and generates topic entries from them.
func buildTopicIndex(homeDir string) []TopicEntry {
	var entries []TopicEntry
	seen := make(map[string]bool) // deduplicate by lowercase heading

	pagesDir := filepath.Join(homeDir, DetectPagesDir(homeDir))
	componentsDir := filepath.Join(homeDir, "docs", "content", "components")

	dirs := []string{pagesDir, componentsDir}

	registry := GetURLRegistry(homeDir)

	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".md") && !strings.HasSuffix(d.Name(), ".mdx") {
				return nil
			}

			rel, _ := filepath.Rel(homeDir, path)
			docURL := constructValidatedDocURL(rel, registry)

			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
					continue
				}

				// Strip the markdown heading prefix
				heading := strings.TrimLeft(line, "# ")

				// Strip anchor syntax like [#some-anchor]
				if idx := strings.Index(heading, "[#"); idx >= 0 {
					heading = strings.TrimSpace(heading[:idx])
				}

				heading = strings.TrimSpace(heading)
				if heading == "" {
					continue
				}

				key := strings.ToLower(heading)
				if seen[key] {
					continue
				}
				seen[key] = true

				// Tokenize heading into trigger terms
				terms := tokenizeHeading(heading)
				if len(terms) == 0 {
					continue
				}

				// Skip very generic single-word headings
				if len(terms) == 1 && isGenericHeading(terms[0]) {
					continue
				}

				// Build URL with fragment
				var urls []string
				if docURL != "" {
					slug := strings.ReplaceAll(key, " ", "-")
					// For top-level headings (# ), no fragment needed
					if strings.HasPrefix(line, "## ") {
						urls = append(urls, docURL+"#"+slug)
					} else {
						urls = append(urls, docURL)
					}
				}

				entries = append(entries, TopicEntry{
					Name:          heading,
					TriggerTerms: terms,
					CanonicalDocs: []string{rel},
					URLs:          urls,
					Description:   heading,
				})
			}
			return nil
		})
	}

	return entries
}

// tokenizeHeading splits a heading into lowercase keyword tokens,
// filtering out very short words and common noise.
func tokenizeHeading(heading string) []string {
	words := strings.FieldsFunc(heading, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	var tokens []string
	for _, w := range words {
		lower := strings.ToLower(w)
		if len(lower) < 2 {
			continue
		}
		tokens = append(tokens, lower)
	}
	return tokens
}

// isGenericHeading returns true for headings that are too common to be
// useful as topic triggers (e.g. "Properties", "Events", "Examples").
func isGenericHeading(term string) bool {
	generic := map[string]bool{
		"properties": true, "events": true, "examples": true,
		"styling": true, "behaviors": true, "overview": true,
		"usage": true, "description": true, "notes": true,
		"methods": true, "parameters": true, "returns": true,
		"see": true, "also": true, "summary": true,
	}
	return generic[term]
}

// matchTopics returns all topics whose trigger terms overlap with the query tokens.
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
