package sqlitestore_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hypergopher/downcache"
	"github.com/hypergopher/downcache/sqlitestore"
)

func setupTestEnvironment(t *testing.T) *sqlitestore.SQLiteStore {
	// Ensure the testdata directory exists
	if _, err := os.Stat("testdata/data"); os.IsNotExist(err) {
		if err := os.Mkdir("testdata/data", 0755); err != nil {
			t.Fatalf("Failed to create testdata directory: %v", err)
		}
	}

	//Remove any existing test database
	tempDir, err := os.MkdirTemp("", "test_sqlitestore")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	//Create a new SQLiteStore db
	dbPath := filepath.Join(tempDir, "test.db")

	//dbPath := "/Users/patrick/code/hypergopher/downcache/sqlitestore/testdata/data/test.db"

	// Create a new SQLiteStore db using modernc.org/sqlite
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLite db: %v", err)
	}

	// Call init and create a new SQLiteStore db
	store := sqlitestore.NewSQLiteStore(db, dbPath, "posts")

	err = store.Init()
	if err != nil {
		t.Fatalf("Failed to init store: %v", err)
	}

	return store
}

func teardownTestEnvironment(t *testing.T, store *sqlitestore.SQLiteStore) {
	// Close the store
	if err := store.Close(); err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}

	// Remove the test database
	if err := os.RemoveAll(filepath.Dir(store.DBPath())); err != nil {
		t.Fatalf("Failed to remove test database: %v", err)
	}
}

func createTestPost(t *testing.T, store *sqlitestore.SQLiteStore, testPost *downcache.Post) *downcache.Post {
	post := testPost
	if post == nil {
		// Create a new post
		post = &downcache.Post{
			Name:       "Test Post",
			Slug:       "test-post",
			PostType:   "article",
			Content:    "This is a test post",
			Published:  sql.NullString{String: time.Now().Format(time.RFC3339), Valid: true},
			Status:     "draft",
			Visibility: "public",
			Properties: map[string]string{
				"test1": "test 1",
				"test2": "test 2",
			},
			Taxonomies: map[string][]string{
				"tags":       {"tag1", "tag2"},
				"categories": {"cat1", "cat2"},
			},
		}
	}

	// Create the post
	post, err := store.Create(context.Background(), post)
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}

	return post
}

func TestSQLiteStore_Init(t *testing.T) {
	store := setupTestEnvironment(t)

	// Check the store
	if store == nil {
		t.Fatalf("Store is nil")
	}

	defer teardownTestEnvironment(t, store)
}

func TestSQLiteStore_Create(t *testing.T) {
	store := setupTestEnvironment(t)
	defer teardownTestEnvironment(t, store)

	// Create the post
	post := createTestPost(t, store, nil)

	// Check the post
	if post == nil {
		t.Fatalf("Post is nil")
	}

	// Assert with testify that the post ID is not empty
	assert.NotEmpty(t, post.ID)
	assert.Greater(t, post.ID, int64(0))
	assert.Equal(t, post.PostID, downcache.PostPathID(post.PostType, post.Slug))
}

func TestSQLiteStore_Get(t *testing.T) {
	store := setupTestEnvironment(t)
	defer teardownTestEnvironment(t, store)

	// Create the post
	post := createTestPost(t, store, nil)

	// Check the post
	if post == nil {
		t.Fatalf("Post is nil")
	}

	// Get the post
	post2, err := store.Get(context.Background(), post.PostType, post.Slug)
	if err != nil {
		t.Fatalf("Failed to get post: %v", err)
	}

	// Check the post
	if post2 == nil {
		t.Fatalf("Post is nil")
	}

	// Assert with testify that the post ID is not empty
	assert.NotEmpty(t, post2.ID)
	assert.Greater(t, post2.ID, int64(0))
	assert.Equal(t, post2.PostID, downcache.PostPathID(post2.PostType, post2.Slug))
	assert.Equal(t, post2.Name, post.Name)
	assert.Equal(t, post2.Slug, post.Slug)
	assert.Equal(t, post2.PostType, post.PostType)
	assert.Equal(t, post2.Content, post.Content)
	assert.Equal(t, post2.Published, post.Published)
	assert.Equal(t, post2.Status, post.Status)
	assert.Equal(t, post2.Visibility, post.Visibility)
	assert.EqualValues(t, post2.Properties, post.Properties)
	assert.EqualValues(t, post2.Taxonomies, post.Taxonomies)
}

func TestSQLiteStore_Update(t *testing.T) {
	store := setupTestEnvironment(t)
	defer teardownTestEnvironment(t, store)

	post := createTestPost(t, store, nil)

	// Check the post
	if post == nil {
		t.Fatalf("Post is nil")
	}

	oldType := post.PostType
	oldSlug := post.Slug

	// Update the post
	post.Name = "Updated Test Post"
	post.Taxonomies = map[string][]string{
		"tags":       {"tag3", "tag4"},
		"categories": {"cat3", "cat4"},
	}
	err := store.Update(context.Background(), oldType, oldSlug, post)
	if err != nil {
		t.Fatalf("Failed to update post: %v", err)
	}

	// Assert with testify that the post ID is not empty
	assert.NotEmpty(t, post.ID)
	assert.Greater(t, post.ID, int64(0))
	assert.Equal(t, post.PostID, downcache.PostPathID(post.PostType, post.Slug))
	assert.Equal(t, post.Name, "Updated Test Post")
	assert.EqualValues(t, post.Taxonomies, map[string][]string{
		"tags":       {"tag3", "tag4"},
		"categories": {"cat3", "cat4"},
	})
}

func TestSQLiteStore_Delete(t *testing.T) {
	store := setupTestEnvironment(t)
	defer teardownTestEnvironment(t, store)

	// Create the post
	post := createTestPost(t, store, nil)

	// Check the post
	if post == nil {
		t.Fatalf("Post is nil")
	}

	// Delete the post
	err := store.Delete(context.Background(), post.PostType, post.Slug)
	if err != nil {
		t.Fatalf("Failed to delete post: %v", err)
	}

	// Get the post
	_, err = store.Get(context.Background(), post.PostType, post.Slug)
	assert.Error(t, err)
}

func TestSQLiteStore_Search(t *testing.T) {
	store := setupTestEnvironment(t)
	defer teardownTestEnvironment(t, store)

	post1 := createTestPost(t, store, nil)
	post2 := createTestPost(t, store, &downcache.Post{
		Name:       "Test Post 2",
		Slug:       "test-post-2",
		PostType:   "article",
		Content:    "This is a test post 2",
		Published:  sql.NullString{String: time.Now().Format(time.RFC3339), Valid: true},
		Status:     "published",
		Visibility: "public",
		Properties: map[string]string{
			"test1": "test 1",
			"test2": "test 2",
		},
		Taxonomies: map[string][]string{
			"tags":       {"tag1", "tag2"},
			"categories": {"cat1", "cat2"},
		},
	})
	post3 := createTestPost(t, store, &downcache.Post{
		Name:       "Test Post 3",
		Slug:       "test-post-3",
		PostType:   "article",
		Content:    "This is a test post 3",
		Published:  sql.NullString{String: time.Now().Format(time.RFC3339), Valid: true},
		Status:     "published",
		Visibility: "private",
		Properties: map[string]string{
			"test3": "test3",
		},
		Taxonomies: map[string][]string{
			"tags":       {"tag3"},
			"categories": {"cat3"},
		},
	})

	cases := []struct {
		name          string
		filter        downcache.FilterOptions
		expectedPosts []*downcache.Post
	}{
		{
			name: "All posts",
			filter: downcache.FilterOptions{
				PageNum:  1,
				PageSize: 10,
			},
			expectedPosts: []*downcache.Post{post1, post2, post3},
		},
		{
			name: "Filter by name",
			filter: downcache.FilterOptions{
				PageNum:      1,
				PageSize:     10,
				FilterSearch: "Test Post 2",
			},
			expectedPosts: []*downcache.Post{post2},
		},
		{
			name: "Filter by status",
			filter: downcache.FilterOptions{
				PageNum:      1,
				PageSize:     10,
				FilterStatus: "draft",
			},
			expectedPosts: []*downcache.Post{post1},
		},
		{
			name: "Filter by tag",
			filter: downcache.FilterOptions{
				PageNum:  1,
				PageSize: 10,
				FilterTaxonomies: []downcache.KeyValueFilter{
					{
						Key:   "tags",
						Value: "tag3",
					},
				},
			},
			expectedPosts: []*downcache.Post{post3},
		},
		{
			name: "Filter by visibility and category",
			filter: downcache.FilterOptions{
				PageNum:          1,
				PageSize:         10,
				FilterVisibility: "private",
				FilterTaxonomies: []downcache.KeyValueFilter{
					{
						Key:   "categories",
						Value: "cat3",
					},
				},
			},
			expectedPosts: []*downcache.Post{post3},
		},
		{
			name: "Filter by property",
			filter: downcache.FilterOptions{
				PageNum:  1,
				PageSize: 10,
				FilterProperties: []downcache.KeyValueFilter{
					{
						Key:   "test1",
						Value: "test 1",
					},
				},
			},
			expectedPosts: []*downcache.Post{post2, post1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Search for posts
			posts, err := store.Search(context.Background(), tc.filter)
			if err != nil {
				t.Fatalf("Failed to search posts: %v", err)
			}

			// Check the posts
			if posts == nil {
				t.Fatalf("Posts is nil")
			}

			// Assert with testify that the posts are not empty
			assert.NotEmpty(t, posts)
			assert.Len(t, posts, len(tc.expectedPosts))

			// Each of the post names should be in the expectedPosts names
			for _, post := range posts {
				found := false
				for _, expectedPost := range tc.expectedPosts {
					if post.Name == expectedPost.Name {
						found = true
						break
					}
				}
				assert.True(t, found, "Post name not found in expected posts")
			}
		})
	}
}
