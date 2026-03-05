package ignore_test

import (
	"testing"

	"github.com/ValeryCherneykin/ned/internal/ignore"
)

func TestMatcher_NodeModules(t *testing.T) {
	t.Parallel()

	m := ignore.ParseString("node_modules/\n.git/\nvendor/\n")

	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"node_modules", true, true},
		{"node_modules/lodash/index.js", false, true},
		{".git", true, true},
		{".git/config", false, true},
		{"vendor", true, true},
		{"vendor/github.com/pkg/errors/errors.go", false, true},
		{"main.go", false, false},
		{"internal/app/app.go", false, false},
	}

	for _, tc := range cases {
		got := m.Match(tc.path, tc.isDir)
		if got != tc.want {
			t.Errorf("Match(%q, isDir=%v) = %v, want %v", tc.path, tc.isDir, got, tc.want)
		}
	}
}

func TestMatcher_GlobPatterns(t *testing.T) {
	t.Parallel()

	m := ignore.ParseString("*.log\n*.tmp\ndist/\n")

	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"app.log", false, true},
		{"error.log", false, true},
		{"session.tmp", false, true},
		{"dist", true, true},
		{"dist/bundle.js", false, true},
		{"main.go", false, false},
		{"README.md", false, false},
	}

	for _, tc := range cases {
		got := m.Match(tc.path, tc.isDir)
		if got != tc.want {
			t.Errorf("Match(%q, isDir=%v) = %v, want %v", tc.path, tc.isDir, got, tc.want)
		}
	}
}

func TestMatcher_Comments(t *testing.T) {
	t.Parallel()

	m := ignore.ParseString("# this is a comment\n\n*.log\n# another comment\n")

	if !m.Match("app.log", false) {
		t.Error("Match(app.log) = false, want true")
	}

	if m.Match("main.go", false) {
		t.Error("Match(main.go) = true, want false")
	}
}

func TestMatcher_NilSafe(t *testing.T) {
	t.Parallel()

	var m *ignore.Matcher

	if m.Match("anything", false) {
		t.Error("nil Matcher should match nothing")
	}
}

func TestMatcher_ExactFile(t *testing.T) {
	t.Parallel()

	m := ignore.ParseString(".env\n.DS_Store\n")

	if !m.Match(".env", false) {
		t.Error("Match(.env) = false, want true")
	}

	if !m.Match(".DS_Store", false) {
		t.Error("Match(.DS_Store) = false, want true")
	}

	if m.Match(".envrc", false) {
		t.Error("Match(.envrc) = true, want false")
	}
}
