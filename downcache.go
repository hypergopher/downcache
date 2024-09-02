package downcache

import (
	"context"
	"fmt"
)

// DownCache is the main entry point for the markdown cache system
type DownCache struct {
	fs    FileSystemManager
	store PostStore
}

func NewDownCache(fs FileSystemManager, store PostStore) *DownCache {
	return &DownCache{fs: fs, store: store}
}

func (cm *DownCache) SyncAll(ctx context.Context) error {
	posts, errs := cm.fs.Walk(ctx)

	for post := range posts {
		_, err := cm.store.Create(ctx, post)
		if err != nil {
			// If the post already exists, update it
			if err := cm.store.Update(ctx, post.PostType, post.Slug, post); err != nil {
				return fmt.Errorf("error updating existing post %s/%s: %w", post.PostType, post.Slug, err)
			}
		}
	}

	// Check for any errors from Walk
	for err := range errs {
		return fmt.Errorf("error walking filesystem: %w", err)
	}

	return nil
}

func (cm *DownCache) Create(ctx context.Context, post *Post) (*Post, error) {
	// Write to filesystem
	if err := cm.fs.Write(ctx, post); err != nil {
		return nil, fmt.Errorf("error writing to filesystem: %w", err)
	}

	// Add to store
	newPost, err := cm.store.Create(ctx, post)
	if err != nil {
		// Rollback: delete from filesystem if store add fails
		if delErr := cm.fs.Delete(ctx, post.PostType, post.Slug); delErr != nil {
			return nil, fmt.Errorf("failed to add to store and rollback failed: %v, %w", delErr, err)
		}
		return nil, fmt.Errorf("error adding to store: %w", err)
	}

	return newPost, nil
}

func (cm *DownCache) Update(ctx context.Context, oldType, oldSlug string, post *Post) error {
	// If the type or slug has changed, move the file
	if oldType != post.PostType || oldSlug != post.Slug {
		if err := cm.fs.Move(ctx, oldType, oldSlug, post.PostType, post.Slug); err != nil {
			return fmt.Errorf("error moving file: %w", err)
		}
	}

	// Write to filesystem
	if err := cm.fs.Write(ctx, post); err != nil {
		return fmt.Errorf("error writing to filesystem: %w", err)
	}

	// Update in store
	if err := cm.store.Update(ctx, oldType, oldSlug, post); err != nil {
		// Rollback: move file back or revert content if update fails
		if oldType != post.PostType || oldSlug != post.Slug {
			if mvErr := cm.fs.Move(ctx, post.PostType, post.Slug, oldType, oldSlug); mvErr != nil {
				return fmt.Errorf("failed to update store and rollback failed: %v, %w", mvErr, err)
			}
		}
		return fmt.Errorf("error updating in store: %w", err)
	}

	return nil
}

func (cm *DownCache) Delete(ctx context.Context, postType, slug string) error {
	// Delete from filesystem
	if err := cm.fs.Delete(ctx, postType, slug); err != nil {
		return fmt.Errorf("error deleting from filesystem: %w", err)
	}

	// Delete from store
	if err := cm.store.Delete(ctx, postType, slug); err != nil {
		// Note: We don't rollback the filesystem delete here, as the file is considered the source of truth
		return fmt.Errorf("error deleting from store: %w", err)
	}

	return nil
}

func (cm *DownCache) Get(ctx context.Context, postType, slug string) (*Post, error) {
	// Try to get from store first (it's faster)
	post, err := cm.store.Get(ctx, postType, slug)
	if err == nil {
		return post, nil
	}

	// If not in store, try to get from filesystem
	post, err = cm.fs.Read(ctx, postType, slug)
	if err != nil {
		return nil, fmt.Errorf("post not found in store or filesystem: %w", err)
	}

	// Add to store for future fast retrieval
	newPost, err := cm.store.Create(ctx, post)
	if err != nil {
		// Log the error but don't fail the operation
		fmt.Printf("Failed to add post to store after filesystem retrieval: %v\n", err)
	}

	return newPost, nil
}

//func (cm *DownCache) List(ctx context.Context, postType string) ([]*Post, error) {
//	return cm.store.List(ctx, postType)
//}

func (cm *DownCache) Search(ctx context.Context, filter FilterOptions) ([]*Post, int, error) {
	return cm.store.Search(ctx, filter)
}
