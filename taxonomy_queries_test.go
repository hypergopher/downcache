package downcache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hypergopher/downcache"
)

func TestGetTaxonomyTerms(t *testing.T) {
	markPath, dataPath := setupTestEnvironment(t)
	defer cleanupTestEnvironment(t, markPath, dataPath)

	dg := createDownCache(t, markPath, dataPath)

	defer func(dg *downcache.DownCache) {
		_ = dg.Close()
	}(dg)

	assert.NotNil(t, dg)

	cases := []struct {
		name          string
		taxonomy      string
		expectedCount int
		expectedTerms []string
	}{
		{
			name:          "Get all tags",
			taxonomy:      "tags",
			expectedCount: 3,
			expectedTerms: []string{
				"tag1",
				"tag2",
				"tag3",
			},
		},
		{
			name:          "Get all categories",
			taxonomy:      "categories",
			expectedCount: 4,
			expectedTerms: []string{
				"cat1",
				"cat2",
				"cat3",
				"cat4", // cat4 added in articles/nested/nested-article.md
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			terms, err := dg.GetTaxonomyTerms(tc.taxonomy)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedCount, len(terms))
			assert.ElementsMatch(t, tc.expectedTerms, terms)
		})
	}
}
