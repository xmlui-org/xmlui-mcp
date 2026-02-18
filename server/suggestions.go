package server

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// suggestAlternatives returns up to maxN suggestions for a query that produced
// zero or low-confidence results. It uses Levenshtein distance and substring
// containment against known components and topic names.
func suggestAlternatives(query string, homeDir string, maxN int) []string {
	candidates := buildCandidateList(homeDir)
	if len(candidates) == 0 {
		return nil
	}

	queryLower := strings.ToLower(strings.TrimSpace(query))
	queryTokens := strings.Fields(queryLower)

	type scored struct {
		name  string
		score int // lower is better
	}

	var matches []scored
	for _, candidate := range candidates {
		candidateLower := strings.ToLower(candidate)
		bestScore := 1000

		// Check each query token against the candidate
		for _, token := range queryTokens {
			// Substring containment (very good match)
			if strings.Contains(candidateLower, token) {
				dist := len(candidateLower) - len(token) // prefer exact-length matches
				if dist < bestScore {
					bestScore = dist
				}
			}

			// Levenshtein distance
			lev := levenshtein(token, candidateLower)
			if lev <= 3 && lev < bestScore {
				bestScore = lev
			}
		}

		// Also check full query against candidate
		if strings.Contains(candidateLower, queryLower) {
			bestScore = 0
		}
		lev := levenshtein(queryLower, candidateLower)
		if lev <= 3 && lev < bestScore {
			bestScore = lev
		}

		if bestScore < 1000 {
			matches = append(matches, scored{name: candidate, score: bestScore})
		}
	}

	// Sort by score ascending (best first)
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].score < matches[j].score
	})

	// Don't suggest exact matches
	var result []string
	for _, m := range matches {
		if strings.EqualFold(m.name, query) {
			continue
		}
		result = append(result, m.name)
		if len(result) >= maxN {
			break
		}
	}
	return result
}

// buildCandidateList constructs the list of known component names and topic names.
func buildCandidateList(homeDir string) []string {
	var candidates []string

	// Scan component docs
	componentRoot := filepath.Join(homeDir, "docs", "content", "components")
	if entries, err := os.ReadDir(componentRoot); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				// Check for subcomponents
				subDir := filepath.Join(componentRoot, name)
				if subEntries, err := os.ReadDir(subDir); err == nil {
					for _, sub := range subEntries {
						if !sub.IsDir() && strings.HasSuffix(sub.Name(), ".md") {
							candidates = append(candidates, strings.TrimSuffix(sub.Name(), ".md"))
						}
					}
				}
				continue
			}
			if strings.HasSuffix(name, ".md") {
				candidates = append(candidates, strings.TrimSuffix(name, ".md"))
			}
		}
	}

	// Add topic names
	for _, topic := range topicIndex {
		candidates = append(candidates, topic.Name)
	}

	return candidates
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use single-row optimization
	prev := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr := make([]int, lb+1)
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = minOf3(
				curr[j-1]+1,   // insertion
				prev[j]+1,     // deletion
				prev[j-1]+cost, // substitution
			)
		}
		prev = curr
	}
	return prev[lb]
}

func minOf3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
