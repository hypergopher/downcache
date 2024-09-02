package downcache

// Paginator is a struct that holds information about pagination, such as the total number of pages, the current page,
// the next and previous pages, the page size, whether there are more pages, whether there are posts,
// the total number of posts, all posts, featured posts, non-featured posts, and whether the paginator is visible.
type Paginator struct {
	TotalPages       int
	CurrentPage      int
	NextPage         int
	PrevPage         int
	PageSize         int
	HasNext          bool
	HasPrev          bool
	HasPosts         bool
	TotalPosts       int
	AllPosts         []*Post
	FeaturedPosts    []*Post
	NonFeaturedPosts []*Post
	Visible          bool // True by default, but can be set to false in the view. E.g. on the home page.
}

// NewPaginator returns a Paginator struct with the given parameters.
func NewPaginator(docs []*Post, total, currentPage, pageSize int, includeFeatured bool) Paginator {
	totalPages := (total + pageSize - 1) / pageSize
	nextPage := currentPage + 1
	prevPage := currentPage - 1
	hasNext := currentPage < totalPages
	hasPrev := currentPage > 1
	lenDocs := len(docs)
	hasDocs := lenDocs > 0

	if nextPage > totalPages {
		nextPage = totalPages
	}

	if prevPage < 1 {
		prevPage = 1
	}

	// Split docs into featured and non-featured
	featured := make([]*Post, 0)
	nonFeatured := make([]*Post, 0)

	if includeFeatured {
		for _, doc := range docs {
			if doc.Pinned {
				featured = append(featured, doc)
			} else {
				nonFeatured = append(nonFeatured, doc)
			}
		}
	} else {
		nonFeatured = docs
	}

	return Paginator{
		TotalPages:       totalPages,
		CurrentPage:      currentPage,
		NextPage:         nextPage,
		PrevPage:         prevPage,
		PageSize:         pageSize,
		HasNext:          hasNext,
		HasPrev:          hasPrev,
		HasPosts:         hasDocs,
		TotalPosts:       lenDocs,
		AllPosts:         docs,
		FeaturedPosts:    featured,
		NonFeaturedPosts: nonFeatured,
		Visible:          true,
	}
}
