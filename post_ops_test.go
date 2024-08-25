package downcache_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hypergopher/downcache"
)

// Cases for creating files
var opsCases = []struct {
	name                string
	postType            string
	path                string
	content             string
	format              downcache.FrontmatterFormat
	meta                *downcache.PostMeta
	expectValidationErr error
}{
	{
		name:     "Create a new page with yaml",
		postType: downcache.PostTypeKeyPage.String(),
		path:     "new-page",
		content:  "This is a new page",
		format:   downcache.FrontmatterYAML,
		meta: &downcache.PostMeta{
			Name:       "New Page",
			Visibility: "public",
			Status:     "published",
			Published:  time.Now(),
		},
	},
	{
		name:     "Create a new article with toml",
		postType: downcache.PostTypeKeyArticle.String(),
		path:     "foobar/new-article",
		content:  "This is a new article",
		format:   downcache.FrontmatterTOML,
		meta: &downcache.PostMeta{
			Name:       "New Article",
			Visibility: "public",
			Status:     "published",
			Published:  time.Now(),
			Taxonomies: map[string][]string{
				"tags":       {"tag1", "tag2"},
				"categories": {"cat1", "cat2"},
			},
			Authors: []string{"author1", "author2"},
		},
	},
	{
		name:     "Create a new draft article",
		postType: downcache.PostTypeKeyArticle.String(),
		path:     "foobar/draft-article",
		content:  "This is a new draft article",
		format:   downcache.FrontmatterTOML,
		meta: &downcache.PostMeta{
			Name:       "Draft Article",
			Visibility: "public",
			Status:     "draft",
			Published:  time.Now(),
			Taxonomies: map[string][]string{
				"tags":       {"draft"},
				"categories": {"draft"},
			},
			Authors:  []string{"author1"},
			Featured: true,
			Photo:    "/images/draft.jpg",
		},
	},
	{
		name:     "Create a new note without metadata",
		postType: downcache.PostTypeKeyNote.String(),
		path:     "note",
		content:  "This is a new note",
		format:   downcache.FrontmatterYAML,
		meta:     nil,
	},
	{
		name:                "Validation error: invalid doc type",
		postType:            "invalid",
		path:                "invalid-doc-type",
		content:             "This is a new note",
		format:              downcache.FrontmatterYAML,
		meta:                nil,
		expectValidationErr: downcache.ErrInvalidPostType,
	},
	{
		name:                "Validation error: invalid path",
		postType:            downcache.PostTypeKeyNote.String(),
		path:                "",
		content:             "This is a new note",
		format:              downcache.FrontmatterYAML,
		meta:                nil,
		expectValidationErr: downcache.ErrInvalidPostSlug,
	},
	{
		name:     "Validation error: invalid status",
		postType: downcache.PostTypeKeyNote.String(),
		path:     "invalid-status",
		content:  "This is a new note",
		format:   downcache.FrontmatterYAML,
		meta: &downcache.PostMeta{
			Name:   "Invalid Status",
			Status: "invalid",
		},
		expectValidationErr: downcache.ErrInvalidPostMeta,
	},
	{
		name:     "Validation error: invalid visibility",
		postType: downcache.PostTypeKeyNote.String(),
		path:     "invalid-visibility",
		content:  "This is a new note",
		format:   downcache.FrontmatterYAML,
		meta: &downcache.PostMeta{
			Name:       "Invalid Visibility",
			Visibility: "invalid",
		},
		expectValidationErr: downcache.ErrInvalidPostMeta,
	},
}

func TestDownCache_DeletePost(t *testing.T) {
	markPath, dataPath := setupTestEnvironment(t)
	defer cleanupTestEnvironment(t, markPath, dataPath)

	dg := createDownCache(t, markPath, dataPath)

	defer func(dg *downcache.DownCache) {
		_ = dg.Close()
	}(dg)

	assert.NotNil(t, dg)

	for _, tc := range opsCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectValidationErr == nil {
				// Save the post
				savePath, err := dg.CreatePost(tc.postType, tc.path, tc.content, tc.meta)
				assert.NoError(t, err)

				// Find the file
				_, err = os.Stat(savePath)
				assert.NoError(t, err)

				// Delete the file
				err = dg.DeletePost(tc.postType, tc.path)
				assert.NoError(t, err)

				// Check the index, the post should not exist
				_, err = dg.GetPost(downcache.PageID(tc.postType, tc.path))
				assert.Error(t, err)
			}
		})
	}
}

func TestDownCache_CreatePost(t *testing.T) {
	markPath, dataPath := setupTestEnvironment(t)
	defer cleanupTestEnvironment(t, markPath, dataPath)

	dg := createDownCache(t, markPath, dataPath)

	defer func(dg *downcache.DownCache) {
		_ = dg.Close()
	}(dg)

	assert.NotNil(t, dg)

	// Create posts that don't exist
	for _, tc := range opsCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectValidationErr != nil {
				_, err := dg.CreatePost(tc.postType, tc.path, tc.content, tc.meta)
				assert.ErrorIs(t, err, tc.expectValidationErr)

			} else {
				savePath, err := dg.CreatePost(tc.postType, tc.path, tc.content, tc.meta)
				assert.NoError(t, err)

				// Find the file
				filePath := filepath.Join(markPath, tc.postType, tc.path+".md")
				assert.Equal(t, filePath, savePath)
				_, err = os.Stat(filePath)
				assert.NoError(t, err)

				// Read the file
				fileContent, err := os.ReadFile(filePath)
				assert.NoError(t, err)

				// Check the content
				assert.Contains(t, string(fileContent), tc.content)
				if tc.meta != nil {
					assert.Contains(t, string(fileContent), tc.meta.Name)
					assert.Contains(t, string(fileContent), tc.meta.Visibility)
					assert.Contains(t, string(fileContent), tc.meta.Status)
				}

				// Check the index
				doc, err := dg.GetPost(downcache.PageID(tc.postType, tc.path))
				assert.NoError(t, err)

				assert.Equal(t, tc.postType, doc.PostType)
				assert.Equal(t, tc.path, doc.Slug)

				if tc.meta != nil {
					assert.Equal(t, tc.meta.Name, doc.Name)
					assert.Equal(t, tc.meta.Visibility, doc.Visibility)
					assert.Equal(t, tc.meta.Status, doc.Status)
					assert.Equal(t, tc.meta.Authors, doc.Authors)
					assert.Equal(t, tc.meta.Taxonomies, doc.Taxonomies)
					assert.Equal(t, tc.meta.Featured, doc.Featured)
					assert.Equal(t, tc.meta.Photo, doc.Photo)
				}
			}
		})
	}

	// Attempt to create a post that already exists
	case1 := opsCases[0]
	t.Run("Create a post that already exists", func(t *testing.T) {
		// Attempt to create the doc again
		_, err := dg.CreatePost(case1.postType, case1.path, case1.content, case1.meta)
		assert.Error(t, err)
	})
}

func TestDownCache_UpdatePost(t *testing.T) {
	markPath, dataPath := setupTestEnvironment(t)
	defer cleanupTestEnvironment(t, markPath, dataPath)

	dg := createDownCache(t, markPath, dataPath)

	defer func(dg *downcache.DownCache) {
		_ = dg.Close()
	}(dg)

	assert.NotNil(t, dg)

	for _, tc := range opsCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectValidationErr != nil {
				_, err := dg.UpdatePost(tc.postType, tc.path, tc.content, tc.meta)
				assert.ErrorIs(t, err, tc.expectValidationErr)
			} else {
				// Create the initial post
				_, err := dg.CreatePost(tc.postType, tc.path, tc.content, tc.meta)
				assert.NoError(t, err)

				if tc.meta == nil {
					tc.meta = &downcache.PostMeta{}
				}

				// Update the post
				newMeta := &downcache.PostMeta{
					Name:       "Updated " + tc.meta.Name,
					Visibility: "private",
					Status:     "draft",
					Published:  time.Now(),
					Taxonomies: map[string][]string{
						"tags":       {"updated"},
						"categories": {"updated"},
					},
					Authors:  []string{"author3"},
					Featured: false,
					Photo:    "/images/updated.jpg",
				}

				_, err = dg.UpdatePost(tc.postType, tc.path, tc.content, newMeta)
				assert.NoError(t, err)

				// Check the index
				doc, err := dg.GetPost(downcache.PageID(tc.postType, tc.path))
				assert.NoError(t, err)

				assert.Equal(t, tc.postType, doc.PostType)
				assert.Equal(t, tc.path, doc.Slug)
				assert.Equal(t, newMeta.Name, doc.Name)
				assert.Equal(t, newMeta.Visibility, doc.Visibility)
				assert.Equal(t, newMeta.Status, doc.Status)
				assert.Equal(t, newMeta.Authors, doc.Authors)
				assert.Equal(t, newMeta.Taxonomies, doc.Taxonomies)
				assert.Equal(t, newMeta.Featured, doc.Featured)
				assert.Equal(t, newMeta.Photo, doc.Photo)
			}
		})
	}

	// Attempt to update a post that doesn't exist
	case1 := opsCases[0]
	t.Run("Update a post that doesn't exist", func(t *testing.T) {
		_, err := dg.UpdatePost(case1.postType, case1.path+"/does-not-exist", case1.content, case1.meta)
		assert.ErrorIs(t, err, downcache.ErrPostNotFound)
	})
}
