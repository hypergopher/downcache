package downcache_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hypergopher/downcache"
)

const (
	testDataDir = "testdata"
	tempDir     = "downcache_test_temp"
	markDir     = "markdown"
	dataDir     = "index"
)

func setupTestEnvironment(t *testing.T) (string, string) {
	t.Helper()

	// Create test data directories
	tempDir, err := os.MkdirTemp("", tempDir)
	require.NoError(t, err)

	markPath := filepath.Join(tempDir, markDir)
	dataPath := filepath.Join(tempDir, dataDir)

	require.NoError(t, os.MkdirAll(markPath, 0755))
	require.NoError(t, os.MkdirAll(dataPath, 0755))

	// Copy testdata files to temp directory
	require.NoError(t, copyDir(t, testDataDir, markPath))

	return markPath, dataPath
}

// copyDir copies the contents of a directory to another directory
func copyDir(t *testing.T, join string, path string) error {
	t.Helper()

	return filepath.Walk(join, func(src string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(join, src)
		if err != nil {
			return err
		}

		dest := filepath.Join(path, relPath)

		if info.IsDir() {
			return os.MkdirAll(dest, 0755)
		}

		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}

		return os.WriteFile(dest, data, 0644)
	})
}

func cleanupTestEnvironment(t *testing.T, tempDir ...string) {
	t.Helper()

	for _, dir := range tempDir {
		require.NoError(t, os.RemoveAll(dir))
	}
}

func createDownCache(t *testing.T, markPath, dataPath string) *downcache.DownCache {
	t.Helper()

	dg, err := downcache.NewDownCache(downcache.Options{
		MarkdownDir: markPath,
		DataDir:     dataPath,
		Authors: map[string]downcache.Author{
			"author1": {
				Name:      "Author 1",
				AvatarURL: "/images/author1.jpg",
				Links: []downcache.AuthorLink{
					{
						Name: "Mastodon",
						Icon: "mastodon",
						URL:  "https://example.social/@author1",
					},
				},
			},
		},
		Taxonomies: map[string]string{
			"tags":       "tag",
			"categories": "category",
		},
		ClearIndexes: true,
		Reindex:      true,
		Logger:       nil,
	})
	require.NoError(t, err)
	require.NotNil(t, dg)

	return dg
}

func assertEqualDocuments(t *testing.T, expected, actual *downcache.Post) {
	t.Helper()

	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Subtitle, actual.Subtitle)
	assert.Equal(t, expected.Summary, actual.Summary)
	assert.Equal(t, expected.Slug, actual.Slug)
	assert.Equal(t, expected.Content, actual.Content)
	assert.Equal(t, expected.Status, actual.Status)
	assert.Equal(t, expected.Featured, actual.Featured)
	assert.Equal(t, expected.Photo, actual.Photo)
	assert.Equal(t, expected.Taxonomies, actual.Taxonomies)
	assert.Equal(t, expected.Properties, actual.Properties)
	assert.Equal(t, expected.Authors, actual.Authors)
	assert.WithinDuration(t, expected.Published, actual.Published, time.Hour*24)
}

func TestNewDownCache(t *testing.T) {
	markPath, dataPath := setupTestEnvironment(t)
	defer cleanupTestEnvironment(t, markPath, dataPath)

	dg := createDownCache(t, markPath, dataPath)

	defer func(dg *downcache.DownCache) {
		_ = dg.Close()
	}(dg)

	assert.NotNil(t, dg)
}

func TestDownCache_Reindex(t *testing.T) {
	markPath, dataPath := setupTestEnvironment(t)
	defer cleanupTestEnvironment(t, markPath, dataPath)

	dg := createDownCache(t, markPath, dataPath)

	defer func(dg *downcache.DownCache) {
		_ = dg.Close()
	}(dg)

	assert.NotNil(t, dg)

	// Reindex the files
	counts, err := dg.Reindex()
	require.NoError(t, err)

	assert.Equal(t, 2, counts["pages"])
	assert.Equal(t, 6, counts["articles"])
}

func TestDownCache_GetDocument(t *testing.T) {
	markPath, dataPath := setupTestEnvironment(t)
	defer cleanupTestEnvironment(t, markPath, dataPath)

	dg := createDownCache(t, markPath, dataPath)

	defer func(dg *downcache.DownCache) {
		_ = dg.Close()
	}(dg)

	assert.NotNil(t, dg)

	// Reindex the files
	_, err := dg.Reindex()
	require.NoError(t, err)

	// Create table-driven tests to get posts and compare the results
	cases := []struct {
		name     string
		path     string
		expected *downcache.Post
	}{
		{
			name: "Get published page",
			path: "pages/published-page",
			expected: &downcache.Post{
				EstimatedReadTime: "< 1 min",
				Name:              "Page 1",
				Subtitle:          "Page 1 subtitle",
				Summary:           "Page 1 summary",
				Slug:              "published-page",
				Content:           "<p>Page 1 content.</p>\n",
				Status:            "published",
				PostType:          "page",
			},
		},
		{
			name: "Get Unpublished page",
			path: "pages/unpublished-page",
			expected: &downcache.Post{
				EstimatedReadTime: "< 1 min",
				Name:              "Page 2",
				Subtitle:          "",
				Summary:           "Page 2 summary",
				Slug:              "unpublished-page",
				Content:           "<p>Page 2 content.</p>\n",
				Status:            "draft",
				PostType:          "page",
			},
		},
		{
			name: "Get published article (YAML)",
			path: "articles/published-article",
			expected: &downcache.Post{
				EstimatedReadTime: "< 1 min",
				Name:              "Published Article",
				Subtitle:          "Published article subtitle",
				Summary:           "Published article summary",
				Slug:              "published-article",
				Content:           "<p>Published article content.</p>\n",
				Status:            "published",
				Published:         time.Date(2024, 8, 21, 0, 0, 0, 0, time.UTC),
				PostType:          "article",
				Featured:          true,
				Photo:             "/images/featured.jpg",
				Authors:           []string{"author1"},
				Taxonomies: map[string][]string{
					"tags":       {"tag1", "tag2", "tag3"},
					"categories": {"cat1", "cat2", "cat3"},
				},
				Properties: map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
		{
			name: "Get published article (TOML)",
			path: "articles/published-toml-article",
			expected: &downcache.Post{
				EstimatedReadTime: "< 1 min",
				Name:              "Published TOML Article",
				Subtitle:          "Published article subtitle",
				Summary:           "Published article summary",
				Slug:              "published-toml-article",
				Content:           "<p>Published article content.</p>\n",
				Status:            "published",
				Published:         time.Date(2024, 8, 21, 0, 0, 0, 0, time.UTC),
				PostType:          "article",
				Featured:          true,
				Photo:             "/images/featured.jpg",
				Authors:           []string{"author1"},
				Taxonomies: map[string][]string{
					"tags":       {"tag1", "tag2"},
					"categories": {"cat1", "cat2"},
				},
				Properties: map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
		{
			name: "Get published NESTED article",
			path: "articles/nested/nested-article",
			expected: &downcache.Post{
				EstimatedReadTime: "< 1 min",
				Name:              "Nested FOOBAR Article",
				Subtitle:          "Nested article subtitle",
				Summary:           "Nested article summary",
				Slug:              "nested/nested-article",
				Content:           "<p>Nested article content.</p>\n",
				Status:            "published",
				Published:         time.Date(2024, 8, 21, 0, 0, 0, 0, time.UTC),
				PostType:          "article",
				Featured:          true,
				Photo:             "/images/featured.jpg",
				Taxonomies: map[string][]string{
					"tags":       {"tag1", "tag2"},
					"categories": {"cat1", "cat2", "cat4"},
				},
				Properties: map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
		{
			name: "Get date formatted article",
			path: "articles/2024-08-21-article-with-date",
			expected: &downcache.Post{
				EstimatedReadTime: "< 1 min",
				Name:              "Dated Article",
				Subtitle:          "Dated article subtitle",
				Summary:           "Dated article summary",
				Slug:              "2024-08-21-article-with-date",
				Content:           "<p>Dated article content.</p>\n",
				Status:            "published",
				Published:         time.Date(2024, 8, 21, 0, 0, 0, 0, time.UTC),
				PostType:          "article",
				FileTimePath:      "2024-08-21",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := dg.GetPost(tc.path)
			require.NoError(t, err)

			// Ignore the ETag value for now
			doc.ETag = ""

			assertEqualDocuments(t, tc.expected, doc)
		})
	}
}
