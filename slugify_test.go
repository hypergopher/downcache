package downcache_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hypergopher/downcache"
)

func TestConvertPathToSlug(t *testing.T) {
	typeRules := downcache.DefaultPostTypes()
	articleTypeRule := typeRules[downcache.PostTypeKeyArticle]
	pageTypeRule := typeRules[downcache.PostTypeKeyPage]

	fileTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UTC()
	tests := []struct {
		name                 string
		fullPath             string
		ruleType             downcache.PostType
		expectedSlug         string
		expectedFileTimePath string
		expectedFileTime     *time.Time
	}{
		{
			name:                 "Path with date in directory should not parse the date",
			fullPath:             "/path/to/files/articles/2024-01-01/my-post.md",
			ruleType:             articleTypeRule,
			expectedSlug:         "2024-01-01/my-post",
			expectedFileTimePath: "",
			expectedFileTime:     nil,
		},
		{
			name:                 "Path with date in file name should parse the date",
			fullPath:             "/path/to/files/articles/2024-01-01-my-post.md",
			ruleType:             articleTypeRule,
			expectedSlug:         "2024-01-01-my-post",
			expectedFileTimePath: "2024-01-01",
			expectedFileTime:     &fileTime,
		},
		{
			name:                 "Path with nested directory and date in file name should parse the date",
			fullPath:             "/path/to/files/articles/foobar/2024-01-01-my-post.md",
			ruleType:             articleTypeRule,
			expectedSlug:         "foobar/2024-01-01-my-post",
			expectedFileTimePath: "2024-01-01",
			expectedFileTime:     &fileTime,
		},
		{
			name:                 "Path with index.md file",
			fullPath:             "/path/to/files/articles/foobar/my-post/index.md",
			ruleType:             articleTypeRule,
			expectedSlug:         "foobar/my-post",
			expectedFileTimePath: "",
			expectedFileTime:     nil,
		},
		{
			name:                 "Path without date",
			fullPath:             "/path/to/files/articles/my-post.md",
			ruleType:             articleTypeRule,
			expectedSlug:         "my-post",
			expectedFileTimePath: "",
			expectedFileTime:     nil,
		},
		{
			name:                 "Edge case with empty path",
			fullPath:             "",
			ruleType:             articleTypeRule,
			expectedSlug:         "",
			expectedFileTimePath: "",
			expectedFileTime:     nil,
		},
		{
			name:                 "Edge case with non-date prefix",
			fullPath:             "/path/to/files/articles/abcd-ef-gh-my-post.md",
			ruleType:             articleTypeRule,
			expectedSlug:         "abcd-ef-gh-my-post",
			expectedFileTimePath: "",
			expectedFileTime:     nil,
		},
		{
			name:                 "Test non-slugified path",
			fullPath:             "/path/to/files/articles/foobar/My Post With Spaces.md",
			ruleType:             articleTypeRule,
			expectedSlug:         "foobar/my-post-with-spaces",
			expectedFileTimePath: "",
			expectedFileTime:     nil,
		},
		{
			name:                 "Test non-slugified path with date prefix",
			fullPath:             "/path/to/files/articles/2024-01-01-My Post With Spaces.md",
			ruleType:             articleTypeRule,
			expectedSlug:         "2024-01-01-my-post-with-spaces",
			expectedFileTimePath: "2024-01-01",
			expectedFileTime:     &fileTime,
		},
		{
			name:                 "Test non-slugified path with odd characters and index.md",
			fullPath:             "/path/to/files/articles/foobar/My Post With Spaces & Odd Characters/index.md",
			ruleType:             articleTypeRule,
			expectedSlug:         "foobar/my-post-with-spaces-and-odd-characters",
			expectedFileTimePath: "",
			expectedFileTime:     nil,
		},
		{
			name:                 "Test deeply nested path folders with odd characters and index.md",
			fullPath:             "/path/to/files/articles/foo/bar/baz/My Post With Spaces & Odd Characters/index.md",
			ruleType:             articleTypeRule,
			expectedSlug:         "foo/bar/baz/my-post-with-spaces-and-odd-characters",
			expectedFileTimePath: "",
			expectedFileTime:     nil,
		},
		{
			name:                 "Page type path without date",
			fullPath:             "/path/to/files/pages/my-page.md",
			ruleType:             pageTypeRule,
			expectedSlug:         "my-page",
			expectedFileTimePath: "",
			expectedFileTime:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slugPath := downcache.SlugifyPath("/path/to/files", tt.fullPath, tt.ruleType)
			assert.Equal(t, tt.expectedSlug, slugPath.Slug)
			assert.Equal(t, tt.expectedFileTime, slugPath.FileTime)
			assert.Equal(t, tt.expectedFileTimePath, slugPath.FileTimePath)
		})
	}
}
