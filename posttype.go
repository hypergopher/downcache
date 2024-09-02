package downcache

import "slices"

// PostType is a string key that represents a post type and is used to determine the directory where posts of the given type are stored.
type PostType string

// PostTypes is a slice of PostType.
type PostTypes []PostType

const (
	PostTypeKeyPage     PostType = "pages"
	PostTypeKeyArticle  PostType = "articles"
	PostTypeKeyNote     PostType = "notes"
	PostTypeKeyLink     PostType = "links"
	PostTypeKeyBookmark PostType = "bookmarks"
	PostTypeKeyAny      PostType = "any"
)

// String returns the string representation of the PostType.
func (pt PostType) String() string {
	return string(pt)
}

// IsAny returns true if the PostType is PostTypeKeyAny.
func (pt PostType) IsAny() bool {
	return pt == PostTypeKeyAny || pt == ""
}

// HasPostType returns true if the PostType is not empty.
func (pts PostTypes) HasPostType(key string) bool {
	return slices.Contains(pts, PostType(key))
}

func DefaultPostTypes() PostTypes {
	return PostTypes{
		PostTypeKeyArticle,
		PostTypeKeyPage,
		PostTypeKeyNote,
		PostTypeKeyLink,
		PostTypeKeyBookmark,
	}
}
