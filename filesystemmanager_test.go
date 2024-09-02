package downcache_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hypergopher/downcache"
)

// Assume we have a real MarkdownProcessor implementation
var realProcessor downcache.MarkdownProcessor = &downcache.DefaultMarkdownProcessor{}

func TestLocalFileSystemManager_Walk(t *testing.T) {
	testDataDir := filepath.Join("testdata")
	fsm := downcache.NewLocalFileSystemManager(testDataDir, realProcessor, downcache.FrontmatterYAML)

	posts, errs := fsm.Walk(context.Background())

	var receivedPosts []*downcache.Post
	for post := range posts {
		receivedPosts = append(receivedPosts, post)
	}

	for err := range errs {
		require.NoError(t, err)
	}

	assert.Len(t, receivedPosts, 3) // Assuming we have 3 markdown files in testdata

	expectedPosts := map[string]*downcache.Post{
		"articles:post1": {
			PostType:  "articles",
			Slug:      "post1",
			Name:      "First Blog Post",
			Author:    "John Doe",
			Status:    "published",
			Pinned:    true,
			Published: sql.NullString{String: "2024-01-01", Valid: true},
		},
		"pages:about": {
			PostType: "pages",
			Slug:     "about",
			Name:     "About Us",
			Author:   "Jane Smith",
			Status:   "published",
		},
		"notes:quick-note": {
			PostType:  "notes",
			Slug:      "quick-note",
			Name:      "Quick Note",
			Author:    "Bob Johnson",
			Status:    "draft",
			Published: sql.NullString{String: "2024-01-02", Valid: true},
		},
	}

	for _, post := range receivedPosts {
		expected, ok := expectedPosts[post.PostType+":"+post.Slug]
		assert.True(t, ok, "Unexpected post: %s:%s", post.PostType, post.Slug)
		assert.Equal(t, expected.Name, post.Name)
		assert.Equal(t, expected.Author, post.Author)
		assert.Equal(t, expected.Status, post.Status)
		assert.Equal(t, expected.Pinned, post.Pinned)
		assert.Equal(t, expected.Published, post.Published, "Post: %s:%s", post.PostType, post.Slug)
		assert.NotEmpty(t, post.Content)
		assert.NotEmpty(t, post.HTML)
	}
}

func TestLocalFileSystemManager_ReadWriteDelete(t *testing.T) {
	testDataDir := filepath.Join("testdata")
	fsm := downcache.NewLocalFileSystemManager(testDataDir, realProcessor, downcache.FrontmatterYAML)

	testCases := []struct {
		name     string
		postType string
		slug     string
		filename string
	}{
		{
			name:     "Article",
			postType: "articles",
			slug:     "post1",
			filename: "post1.md",
		},
		{
			name:     "Page",
			postType: "pages",
			slug:     "about",
			filename: "about.md",
		},
		{
			name:     "Note",
			postType: "notes",
			slug:     "quick-note",
			filename: "quick-note.md",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test Read
			readPost, err := fsm.Read(context.Background(), tc.postType, tc.slug)
			require.NoError(t, err)
			assert.Equal(t, tc.postType, readPost.PostType)
			assert.Equal(t, tc.slug, readPost.Slug)
			assert.NotEmpty(t, readPost.Content)
			assert.NotEmpty(t, readPost.HTML)

			// Test Write (to a temporary file)
			tempSlug := fmt.Sprintf("temp-%s", tc.slug)
			err = fsm.Write(context.Background(), &downcache.Post{
				Name:     readPost.Name,
				PostType: tc.postType,
				Slug:     tempSlug,
				Content:  "Foo bar is baz",
			})
			require.NoError(t, err)

			// Verify the written file
			tempPost, err := fsm.Read(context.Background(), tc.postType, tempSlug)
			require.NoError(t, err)
			assert.Equal(t, tempPost.HTML, "<p>Foo bar is baz</p>\n")

			// Test Delete (the temporary file)
			err = fsm.Delete(context.Background(), tc.postType, tempSlug)
			require.NoError(t, err)

			// Verify deletion
			_, err = fsm.Read(context.Background(), tc.postType, tempSlug)
			assert.Error(t, err)
		})
	}
}

func TestLocalFileSystemManager_Move(t *testing.T) {
	testDataDir := filepath.Join("testdata")
	fsm := downcache.NewLocalFileSystemManager(testDataDir, realProcessor, downcache.FrontmatterYAML)

	// Create a temporary post for moving
	tempType := "articles"
	tempSlug := "temp-move-post"
	tempContent := `---
author: Test Author
name: Temporary Move Post
status: draft
---

# Temporary Move Post
This is a temporary post for testing the move operation.`

	tempContentOnly := `# Temporary Move Post
This is a temporary post for testing the move operation.`

	err := fsm.Write(context.Background(), &downcache.Post{
		Name:     "Temporary Move Post",
		Author:   "Test Author",
		Status:   "draft",
		PostType: tempType,
		Slug:     tempSlug,
		Content:  tempContentOnly,
	})
	require.NoError(t, err)

	// Move the post
	newType := "pages"
	newSlug := "moved-post"
	err = fsm.Move(context.Background(), tempType, tempSlug, newType, newSlug)
	require.NoError(t, err)

	// Verify the post has been moved
	movedPost, err := fsm.Read(context.Background(), newType, newSlug)
	require.NoError(t, err)
	assert.Equal(t, tempContent, movedPost.Content)
	assert.Equal(t, newType, movedPost.PostType)
	assert.Equal(t, newSlug, movedPost.Slug)

	// Verify the original post no longer exists
	_, err = fsm.Read(context.Background(), tempType, tempSlug)
	assert.Error(t, err)

	// Clean up
	err = fsm.Delete(context.Background(), newType, newSlug)
	require.NoError(t, err)
}

func TestLocalFileSystemManager_Concurrency(t *testing.T) {
	testDataDir := filepath.Join("testdata")
	fsm := downcache.NewLocalFileSystemManager(testDataDir, realProcessor, downcache.FrontmatterYAML)

	concurrentOps := 100
	errChan := make(chan error, concurrentOps)

	for i := 0; i < concurrentOps; i++ {
		go func(i int) {
			slug := fmt.Sprintf("concurrent-post-%d", i)
			content := fmt.Sprintf(`---
name: Concurrent Post %d
author: Test Author
status: draft
---
# Concurrent Post %d
This is concurrent post %d.`, i, i, i)

			post := &downcache.Post{
				PostType: "articles",
				Slug:     slug,
				Content:  content,
			}

			err := fsm.Write(context.Background(), post)
			if err != nil {
				errChan <- err
				return
			}

			_, err = fsm.Read(context.Background(), post.PostType, post.Slug)
			if err != nil {
				errChan <- err
				return
			}

			err = fsm.Delete(context.Background(), post.PostType, post.Slug)
			if err != nil {
				errChan <- err
				return
			}

			errChan <- nil
		}(i)
	}

	for i := 0; i < concurrentOps; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}
}
