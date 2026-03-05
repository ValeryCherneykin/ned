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

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		dirOnly := strings.HasSuffix(line, "/")
		pattern := strings.TrimSuffix(line, "/")

		m.rules = append(m.rules, rule{pattern: pattern, dirOnly: dirOnly})
	}

	return m
}

// Match reports whether relPath (relative to the watched directory) should be ignored.
// isDir must be true when relPath refers to a directory.
func (m *Matcher) Match(relPath string, isDir bool) bool {
	if m == nil {
		return false
	}

	// Normalise to forward slashes for consistent matching.
	relPath = filepath.ToSlash(relPath)
	base := filepath.Base(relPath)

	for _, r := range m.rules {
		// dir-only rules only match directories.
		if r.dirOnly && !isDir {
			continue
		}

		// Match against base name first (most common case: *.log, node_modules).
		if matchGlob(r.pattern, base) {
			return true
		}

		// Match against full relative path (e.g. dist/bundle.js).
		if matchGlob(r.pattern, relPath) {
			return true
		}

		// Match path prefix — if rule is "vendor" ignore "vendor/anything".
		prefix := r.pattern + "/"
		if strings.HasPrefix(relPath, prefix) {
			return true
		}
	}

	return false
}

// matchGlob wraps filepath.Match with a fallback for invalid patterns.
func matchGlob(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		// Invalid pattern — treat as no match.
		return false
	}

	return matched
}
