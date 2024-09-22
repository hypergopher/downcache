package downcache

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type CacheStore interface {
	// Init initializes the post store, such as creating the necessary tables or indexes.
	Init() error
	// Clear clears all data from the post store and resets the store.
	Clear(ctx context.Context) error
	// Close closes the post store.
	Close() error
	// Create creates a new post.
	Create(ctx context.Context, post *Post) (*Post, error)
	// Delete deletes a post.
	Delete(ctx context.Context, postType, slug string) error
	// Get retrieves a post by its slug.
	Get(ctx context.Context, postType, slug string) (*Post, error)
	// GetTaxonomies returns a list of taxonomies.
	GetTaxonomies(ctx context.Context) ([]string, error)
	// GetTaxonomyTerms returns a list of terms for a given taxonomy.
	GetTaxonomyTerms(ctx context.Context, taxonomy string) ([]string, error)
	// Search searches for posts based on the provided filter options.
	Search(ctx context.Context, opts FilterOptions) ([]*Post, int, error)
	// Update updates an existing post.
	Update(ctx context.Context, oldType, oldSlug string, post *Post) error
}

// MemoryCacheStore implements CacheStore interface using in-memory storage
type MemoryCacheStore struct {
	posts map[string]*Post
	mu    sync.RWMutex
}

// NewMemoryCacheStore creates a new MemoryCacheStore
func NewMemoryCacheStore() *MemoryCacheStore {
	return &MemoryCacheStore{
		posts: make(map[string]*Post),
	}
}

// Init initializes the post store
func (m *MemoryCacheStore) Init() error {
	return nil
}

// Clear clears all data from the post store
func (m *MemoryCacheStore) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.posts = make(map[string]*Post)
	return nil
}

// Close closes the post store
func (m *MemoryCacheStore) Close() error {
	return nil
}

// Create adds a new post to the store
func (m *MemoryCacheStore) Create(ctx context.Context, post *Post) (*Post, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.makeKey(post.PostType, post.Slug)
	if _, exists := m.posts[key]; exists {
		return nil, fmt.Errorf("post already exists: %s", key)
	}

	m.posts[key] = post
	return post, nil
}

// Update updates an existing post in the store
func (m *MemoryCacheStore) Update(ctx context.Context, oldType, oldSlug string, post *Post) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// key := m.makeKey(post.PostType, post.Slug)
	key := m.makeKey(oldType, oldSlug)
	if _, exists := m.posts[key]; !exists {
		return fmt.Errorf("post not found: %s", key)
	}

	delete(m.posts, key)
	m.posts[m.makeKey(post.PostType, post.Slug)] = post
	return nil
}

// Delete removes a post from the store
func (m *MemoryCacheStore) Delete(ctx context.Context, postType, slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.makeKey(postType, slug)
	if _, exists := m.posts[key]; !exists {
		return fmt.Errorf("post not found: %s", key)
	}

	delete(m.posts, key)
	return nil
}

// Get retrieves a post from the store
func (m *MemoryCacheStore) Get(ctx context.Context, postType, slug string) (*Post, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.makeKey(postType, slug)
	post, exists := m.posts[key]
	if !exists {
		return nil, fmt.Errorf("post not found: %s", key)
	}

	return post, nil
}

// Search searches for posts based on the provided FilterOptions
func (m *MemoryCacheStore) Search(ctx context.Context, options FilterOptions) ([]*Post, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filtered []*Post

	for _, post := range m.posts {
		if m.postMatchesFilters(post, options) {
			filtered = append(filtered, post)
		}
	}

	totalCount := len(filtered)

	// Sort the filtered posts
	m.sortPosts(filtered, options.SortBy)

	// Split pinned items if required
	var pinned []*Post
	if options.SplitPinned {
		pinned, filtered = m.splitPinned(filtered)
	}

	if options.PageNum <= 0 {
		options.PageNum = 1
	}

	if options.PageSize <= 0 {
		options.PageSize = 10
	}

	// Paginate the results
	start, end := m.getPaginationBounds(options.PageNum, options.PageSize, len(filtered))
	paginatedResults := filtered[start:end]

	// Prepend pinned items if split
	if options.SplitPinned {
		paginatedResults = append(pinned, paginatedResults...)
	}

	return paginatedResults, totalCount, nil
}

// GetTaxonomies returns a list of taxonomies.
// TODO: This is inefficient and should be optimized for large datasets.
func (m *MemoryCacheStore) GetTaxonomies(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var taxonomies []string
	for _, post := range m.posts {
		for taxonomy := range post.Taxonomies {
			taxonomies = append(taxonomies, taxonomy)
		}
	}

	return unique(taxonomies), nil
}

// GetTaxonomyTerms returns a list of terms for a given taxonomy.
// TODO: This is inefficient and should be optimized for large datasets.
func (m *MemoryCacheStore) GetTaxonomyTerms(ctx context.Context, taxonomy string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var terms []string
	for _, post := range m.posts {
		for _, term := range post.Taxonomies[taxonomy] {
			terms = append(terms, term)
		}
	}

	return unique(terms), nil
}

// postMatchesFilters checks if a post matches the provided filters
func (m *MemoryCacheStore) postMatchesFilters(post *Post, options FilterOptions) bool {
	if options.FilterPostType != PostTypeKeyAny && string(options.FilterPostType) != post.PostType {
		return false
	}

	if options.FilterStatus != "" && options.FilterStatus != post.Status {
		return false
	}

	if options.FilterVisibility != "" && options.FilterVisibility != post.Visibility {
		return false
	}

	//if !options.IncludeUnpublished && post.Status != "published" {
	//	return false
	//}

	if options.FilterAuthor != "" && !strings.Contains(post.Author, options.FilterAuthor) {
		return false
	}

	if options.FilterSearch != "" && !strings.Contains(strings.ToLower(post.Content), strings.ToLower(options.FilterSearch)) {
		return false
	}

	for _, prop := range options.FilterProperties {
		if !m.matchesKeyValueFilter(post.Properties, prop) {
			return false
		}
	}

	for _, tax := range options.FilterTaxonomies {
		if !m.matchesKeyValueFilterSlice(post.Taxonomies, tax) {
			return false
		}
	}

	return true
}

// matchesKeyValueFilter checks if a map contains a key-value pair
func (m *MemoryCacheStore) matchesKeyValueFilter(data map[string]string, filter KeyValueFilter) bool {
	value, exists := data[filter.Key]
	if !exists {
		return false
	}

	// Implement type-specific comparisons here
	// This is a simple string comparison; extend as needed for other types
	return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", filter.Value)
}

// matchesKeyValueFilterSlice checks if a map contains a key-value pair in a slice
func (m *MemoryCacheStore) matchesKeyValueFilterSlice(data map[string][]string, filter KeyValueFilter) bool {
	values, exists := data[filter.Key]
	if !exists {
		return false
	}

	for _, value := range values {
		if value == filter.Value {
			return true
		}
	}

	return false
}

// sortPosts sorts the posts based on the provided sort fields
func (m *MemoryCacheStore) sortPosts(posts []*Post, sortBy []string) {
	sort.Slice(posts, func(i, j int) bool {
		for _, field := range sortBy {
			descending := false
			if strings.HasPrefix(field, "-") {
				descending = true
				field = field[1:]
			}

			var comparison int
			switch field {
			case "pinned":
				comparison = compareBool(posts[i].Pinned, posts[j].Pinned)
			case "published":
				comparison = compareTime(posts[i].PublishedTime(), posts[j].PublishedTime())
			case "name":
				comparison = strings.Compare(posts[i].Slug, posts[j].Slug)
			// Add more cases for other fields as needed
			default:
				continue
			}

			if comparison != 0 {
				if descending {
					return comparison > 0
				}
				return comparison < 0
			}
		}
		return false
	})
}

// compareBool compares two booleans
func compareBool(a, b bool) int {
	if a == b {
		return 0
	}
	if a {
		return 1
	}
	return -1
}

// compareTime compares two time.Time values
func compareTime(a, b time.Time) int {
	switch {
	case a.Before(b):
		return -1
	case a.After(b):
		return 1
	default:
		return 0
	}
}

// splitPinned separates pinned posts from non-pinned posts
func (m *MemoryCacheStore) splitPinned(posts []*Post) (pinned, nonPinned []*Post) {
	for _, post := range posts {
		if post.Pinned {
			pinned = append(pinned, post)
		} else {
			nonPinned = append(nonPinned, post)
		}
	}
	return pinned, nonPinned
}

// getPaginationBounds calculates the start and end indices for pagination
func (m *MemoryCacheStore) getPaginationBounds(pageNum, pageSize, totalItems int) (start, end int) {
	start = (pageNum - 1) * pageSize
	end = start + pageSize
	if end > totalItems {
		end = totalItems
	}
	return start, end
}

// makeKey creates a unique key for a post based on its type and slug
func (m *MemoryCacheStore) makeKey(postType, slug string) string {
	return fmt.Sprintf("%s:%s", postType, slug)
}

func unique(slice []string) []string {
	result := make([]string, 0, len(slice))
	inResult := map[string]bool{}
	for _, item := range slice {
		if _, ok := inResult[item]; !ok {
			inResult[item] = true
			result = append(result, item)
		}
	}
	return result
}
