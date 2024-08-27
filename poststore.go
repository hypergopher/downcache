package downcache

type PostStore interface {
	// Init initializes the post store, such as creating the necessary tables or indexes.
	Init() error
	// Create creates a new post.
	Create(post *Post) (*Post, error)
	// Update updates an existing post.
	Update(post *Post) error
	// Delete deletes a post.
	Delete(post *Post) error
	// GetBySlug retrieves a post by its slug.
	GetBySlug(slug string) (*Post, error)
	// Search searches for posts based on the provided filter options.
	Search(opts FilterOptions) ([]*Post, error)
	// Close closes the post store.
	Close() error
}
