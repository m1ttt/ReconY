package phase5

import (
	"sort"
	"testing"

	"reconx/internal/models"
)

func TestExtractSeedDirectories(t *testing.T) {
	ok := 200
	notFound := 404

	discovered := []models.DiscoveredURL{
		{URL: "https://example.com/admin/login", StatusCode: &ok},
		{URL: "https://example.com/admin/api/v2/users", StatusCode: &ok},
		{URL: "https://example.com/static/js/app.js", StatusCode: &ok},
		{URL: "https://example.com/nothing", StatusCode: &notFound}, // should be skipped
	}

	baseTargets := []string{"https://example.com"}

	dirs := extractSeedDirectories(baseTargets, discovered)

	// Expected directories (deduplicated, excluding base target):
	// /admin/, /admin/api/, /admin/api/v2/, /static/, /static/js/
	expected := []string{
		"https://example.com/admin/",
		"https://example.com/admin/api/",
		"https://example.com/admin/api/v2/",
		"https://example.com/static/",
		"https://example.com/static/js/",
	}

	sort.Strings(dirs)
	sort.Strings(expected)

	if len(dirs) != len(expected) {
		t.Fatalf("expected %d dirs, got %d: %v", len(expected), len(dirs), dirs)
	}

	for i, d := range dirs {
		if d != expected[i] {
			t.Errorf("dirs[%d] = %q, want %q", i, d, expected[i])
		}
	}
}

func TestExtractSeedDirectories_SkipsBaseTargets(t *testing.T) {
	ok := 200
	discovered := []models.DiscoveredURL{
		{URL: "https://example.com/index.html", StatusCode: &ok},
	}
	baseTargets := []string{"https://example.com"}

	dirs := extractSeedDirectories(baseTargets, discovered)

	// "/" is the base target, should be skipped — no dirs returned
	if len(dirs) != 0 {
		t.Errorf("expected 0 dirs, got %d: %v", len(dirs), dirs)
	}
}

func TestExtractSeedDirectories_Empty(t *testing.T) {
	dirs := extractSeedDirectories([]string{"https://example.com"}, nil)
	if len(dirs) != 0 {
		t.Errorf("expected 0 dirs, got %d", len(dirs))
	}
}

func TestExtractSeedDirectories_Cap(t *testing.T) {
	ok := 200
	// Generate 200+ unique deep paths
	var discovered []models.DiscoveredURL
	for i := 0; i < 150; i++ {
		discovered = append(discovered, models.DiscoveredURL{
			URL:        "https://example.com/a/b/c/d/e/f/g/h/" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + "/file.txt",
			StatusCode: &ok,
		})
	}

	dirs := extractSeedDirectories([]string{"https://example.com"}, discovered)

	if len(dirs) > 100 {
		t.Errorf("expected cap at 100, got %d", len(dirs))
	}
}
