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

type KeyValueFilter struct {
	Key   string
	Value string
}

// FilterOptions contains the options to filter posts.
type FilterOptions struct {
	PageNum            int              // The page number to retrieve
	PageSize           int              // The number of items per page
	SortBy             []string         // The frontmatter fields to sort by. Default is ["-featured", "-published", "name]
	FilterAuthor       string           // The authors to filter by
	FilterProperties   []KeyValueFilter // The frontmatter fields to filter by
	FilterTaxonomies   []KeyValueFilter // The taxonomies to filter by
	FilterSearch       string           // A search string to filter by. Searches the post content, title, etc.
	FilterPostType     PostType         // The type of post to filter by (e.g. PostTypeKeyArticle, PostTypeKeyPage). Default is PostTypeKeyAny.
	FilterStatus       string           // The status of the post to filter by (e.g. "published", "draft"). Default is "published".
	FilterVisibility   string           // The visibility of the post to filter by (e.g. "public", "private"). Default is "public".
	SplitPinned        bool             // Whether to split featured items from the main list
	IncludeUnpublished bool
}
