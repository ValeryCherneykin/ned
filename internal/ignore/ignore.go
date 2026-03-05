// Package ignore implements .nedignore file parsing and path matching.
// Syntax is a subset of .gitignore: glob patterns, dir/ suffix, # comments.
package ignore

import (
	"bufio"
	"path/filepath"
	"strings"
)

// Matcher holds compiled ignore rules from a .nedignore file.
// The zero value matches nothing — safe to use without initialisation.
type Matcher struct {
	rules []rule
}

type rule struct {
	pattern string
	dirOnly bool // pattern ends with /
}

// ParseString parses .nedignore content from a string.
func ParseString(content string) *Matcher {
	m := &Matcher{}

	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		dirOnly := strings.HasSuffix(line, "/")
		pattern := strings.TrimSuffix(line, "/")

		m.rules = append(m.rules, rule{pattern: pattern, dirOnly: dirOnly})
	}

	return m
}

// Match reports whether relPath should be ignored.
// isDir must be true when relPath refers to a directory.
func (m *Matcher) Match(relPath string, isDir bool) bool {
	if m == nil || len(m.rules) == 0 {
		return false
	}

	relPath = filepath.ToSlash(relPath)

	for _, r := range m.rules {
		if matchRule(r, relPath, isDir) {
			return true
		}
	}

	return false
}

func matchRule(r rule, relPath string, isDir bool) bool {
	// Split path into components: "a/b/c" → ["a", "b", "c"]
	parts := strings.Split(relPath, "/")

	// Check every path component and prefix against the rule.
	// This handles: node_modules/lodash/index.js → matches "node_modules" rule.
	for i, part := range parts {
		isLastPart := i == len(parts)-1
		partIsDir := !isLastPart || isDir

		// dir-only rules only match directory components.
		if r.dirOnly && !partIsDir {
			continue
		}

		// Match component name against pattern.
		if matchGlob(r.pattern, part) {
			return true
		}

		// Also match the path up to this component (handles explicit paths like "dist/bundle.js").
		prefix := strings.Join(parts[:i+1], "/")
		if matchGlob(r.pattern, prefix) {
			return true
		}
	}

	return false
}

// matchGlob wraps filepath.Match with a fallback for invalid patterns.
func matchGlob(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		return false
	}

	return matched
}
