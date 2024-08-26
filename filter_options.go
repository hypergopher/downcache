package downcache

type FilterType string

const (
	FilterTypeAny      FilterType = "any"
	FilterTypeAuthor   FilterType = "author"
	FilterTypeTaxonomy FilterType = "taxonomy"
)

func (ft FilterType) String() string {
	return string(ft)
}

// FilterOptions contains the options to filter posts.
type FilterOptions struct {
	PageNum            int         // The page number to retrieve
	PageSize           int         // The number of items per page
	SortBy             []string    // The frontmatter fields to sort by. Default is ["-featured", "-published", "name]
	FilterType         FilterType  // The type of filter to apply (author, article, taxonomy)
	FilterKey          string      // The key to filter by (e.g. "categories", "tags"). Only used for taxonomy filters currently.
	FilterTerm         string      // The term to filter by (e.g. "cat3", "tag3", "author1"). Used with FilterTypeAuthor and FilterTypeTaxonomy.
	FilterSearch       string      // A search string to filter by. Searches the post content, title, etc.
	FilterPostType     PostTypeKey // The type of post to filter by (e.g. PostTypeKeyArticle, PostTypeKeyPage). Default is PostTypeKeyAny.
	FilterStatus       string      // The status of the post to filter by (e.g. "published", "draft"). Default is "published".
	FilterVisibility   string      // The visibility of the post to filter by (e.g. "public", "private"). Default is "public".
	SplitFeatured      bool        // Whether to split featured items from the main list
	IncludeUnpublished bool
}
