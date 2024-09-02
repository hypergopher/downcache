package downcache_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hypergopher/downcache"
)

// InMemoryFileSystem is a simple in-memory implementation of FileSystemManager
type InMemoryFileSystem struct {
	files map[string]*downcache.Post
}

func NewInMemoryFileSystem() *InMemoryFileSystem {
	return &InMemoryFileSystem{
		files: make(map[string]*downcache.Post),
	}
}

func (fs *InMemoryFileSystem) Walk(_ context.Context) (<-chan *downcache.Post, <-chan error) {
	posts := make(chan *downcache.Post)
	errs := make(chan error)
	go func() {
		defer close(posts)
		defer close(errs)
		for _, post := range fs.files {
			posts <- post
		}
	}()
	return posts, errs
}

func (fs *InMemoryFileSystem) Read(_ context.Context, postType, slug string) (*downcache.Post, error) {
	key := fmt.Sprintf("%s:%s", postType, slug)
	post, ok := fs.files[key]
	if !ok {
		return nil, fmt.Errorf("post not found")
	}
	return post, nil
}

func (fs *InMemoryFileSystem) Write(_ context.Context, post *downcache.Post) error {
	key := fmt.Sprintf("%s:%s", post.PostType, post.Slug)
	fs.files[key] = post
	return nil
}

func (fs *InMemoryFileSystem) Delete(_ context.Context, postType, slug string) error {
	key := fmt.Sprintf("%s:%s", postType, slug)
	delete(fs.files, key)
	return nil
}

func (fs *InMemoryFileSystem) Move(_ context.Context, oldType, oldSlug, newType, newSlug string) error {
	oldKey := fmt.Sprintf("%s:%s", oldType, oldSlug)
	newKey := fmt.Sprintf("%s:%s", newType, newSlug)
	post, ok := fs.files[oldKey]
	if !ok {
		return fmt.Errorf("post not found")
	}
	post.PostType = newType
	post.Slug = newSlug
	fs.files[newKey] = post
	delete(fs.files, oldKey)
	return nil
}

func TestCacheManager_SyncAll(t *testing.T) {
	fs := NewInMemoryFileSystem()
	store := downcache.NewMemoryPostStore()
	cm := downcache.NewDownCache(fs, store)

	// Add some posts to the file system
	_ = fs.Write(context.Background(), &downcache.Post{PostType: "article", Slug: "post1", Name: "Post 1"})
	_ = fs.Write(context.Background(), &downcache.Post{PostType: "page", Slug: "about", Name: "About Us"})

	err := cm.SyncAll(context.Background())
	require.NoError(t, err)

	// Verify that posts were added to the store
	post, err := store.Get(context.Background(), "article", "post1")
	require.NoError(t, err)
	assert.Equal(t, "Post 1", post.Name)

	post, err = store.Get(context.Background(), "page", "about")
	require.NoError(t, err)
	assert.Equal(t, "About Us", post.Name)
}

func TestCacheManager_CreateUpdateDelete(t *testing.T) {
	fs := NewInMemoryFileSystem()
	store := downcache.NewMemoryPostStore()
	cm := downcache.NewDownCache(fs, store)

	ctx := context.Background()
	post := &downcache.Post{PostType: "article", Slug: "new-post", Name: "New Post"}

	// Test Create
	_, err := cm.Create(ctx, post)
	require.NoError(t, err)

	// Verify post exists in both fs and store
	_, err = fs.Read(ctx, "article", "new-post")
	require.NoError(t, err)
	_, err = store.Get(ctx, "article", "new-post")
	require.NoError(t, err)

	// Test Update
	updatedPost := &downcache.Post{PostType: "article", Slug: "updated-post", Name: "Updated Post"}
	err = cm.Update(ctx, "article", "new-post", updatedPost)
	require.NoError(t, err)

	// Verify post was updated in both fs and store
	fsPost, err := fs.Read(ctx, "article", "updated-post")
	require.NoError(t, err)
	assert.Equal(t, "Updated Post", fsPost.Name)
	storePost, err := store.Get(ctx, "article", "updated-post")
	require.NoError(t, err)
	assert.Equal(t, "Updated Post", storePost.Name)

	// Test Delete
	err = cm.Delete(ctx, "article", "updated-post")
	require.NoError(t, err)

	// Verify post was deleted from both fs and store
	_, err = fs.Read(ctx, "article", "updated-post")
	assert.Error(t, err)
	_, err = store.Get(ctx, "article", "updated-post")
	assert.Error(t, err)
}

func TestCacheManager_Get(t *testing.T) {
	fs := NewInMemoryFileSystem()
	store := downcache.NewMemoryPostStore()
	cm := downcache.NewDownCache(fs, store)

	ctx := context.Background()
	post := &downcache.Post{PostType: "article", Slug: "test-post", Name: "Test Post"}

	// Add post to file system only
	err := fs.Write(ctx, post)
	require.NoError(t, err)

	// Test Get
	retrievedPost, err := cm.Get(ctx, "article", "test-post")
	require.NoError(t, err)
	assert.Equal(t, post.Name, retrievedPost.Name)

	// Verify post was added to store
	storePost, err := store.Get(ctx, "article", "test-post")
	require.NoError(t, err)
	assert.Equal(t, post.Name, storePost.Name)
}

func TestCacheManager_Search(t *testing.T) {
	fs := NewInMemoryFileSystem()
	store := downcache.NewMemoryPostStore()
	cm := downcache.NewDownCache(fs, store)

	ctx := context.Background()

	// Add some posts to the store
	_, _ = store.Create(ctx, &downcache.Post{PostType: "article", Slug: "post1", Name: "Post 1", Author: "John"})
	_, _ = store.Create(ctx, &downcache.Post{PostType: "article", Slug: "post2", Name: "Post 2", Author: "Jane"})
	_, _ = store.Create(ctx, &downcache.Post{PostType: "page", Slug: "about", Name: "About Us", Author: "John"})

	// Test Search
	options := downcache.FilterOptions{
		FilterAuthor:   "John",
		FilterPostType: downcache.PostType("article"),
	}
	posts, total, err := cm.Search(ctx, options)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.NotNil(t, posts)
	if len(posts) > 0 {
		assert.Equal(t, "Post 1", posts[0].Name)
	} else {
		t.Error("expected posts to be non-empty")
	}
}
