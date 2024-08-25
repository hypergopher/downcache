package downcache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hypergopher/downcache"
)

func TestDownCache_GetPosts(t *testing.T) {
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

	cases := []struct {
		name          string
		filter        downcache.FilterOptions
		expectedCount int
	}{
		{
			name: "Get all published articles",
			filter: downcache.FilterOptions{
				FilterPostType: downcache.PostTypeKeyArticle,
			},
			expectedCount: 4,
		},
		{
			name: "Get all articles (published and unpublished)",
			filter: downcache.FilterOptions{
				FilterPostType: downcache.PostTypeKeyArticle,
				FilterStatus:   downcache.FilterTypeAny.String(),
			},
			expectedCount: 5,
		},
		{
			name: "Get draft articles only",
			filter: downcache.FilterOptions{
				FilterPostType: downcache.PostTypeKeyArticle,
				FilterStatus:   "draft",
			},
			expectedCount: 1,
		},
		{
			name: "Get private articles only",
			filter: downcache.FilterOptions{
				FilterPostType:   downcache.PostTypeKeyArticle,
				FilterVisibility: "private",
			},
			expectedCount: 1,
		},
		{
			name: "Search for a specific post",
			filter: downcache.FilterOptions{
				FilterPostType: downcache.PostTypeKeyArticle,
				FilterSearch:   "Nested FOOBAR",
			},
			expectedCount: 1,
		},
		{
			name: "Filter by category",
			filter: downcache.FilterOptions{
				FilterType: downcache.FilterTypeTaxonomy,
				FilterKey:  "categories",
				FilterTerm: "cat3",
			},
			expectedCount: 1,
		},
		{
			name: "Filter by tag",
			filter: downcache.FilterOptions{
				FilterType: downcache.FilterTypeTaxonomy,
				FilterKey:  "tags",
				FilterTerm: "tag3",
			},
			expectedCount: 1,
		},
		{
			name: "Filter by author",
			filter: downcache.FilterOptions{
				FilterType: downcache.FilterTypeAuthor,
				FilterTerm: "author1",
			},
			expectedCount: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paginator, err := dg.GetPosts(tc.filter)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedCount, paginator.TotalPosts)
		})
	}
}
