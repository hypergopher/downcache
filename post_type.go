package downcache

import "strings"

// PostTypeKey is a string key that represents a post type.
type PostTypeKey string

// PostTypesMap is a map of PostTypeKey to PostType.
type PostTypesMap map[PostTypeKey]PostType

const (
	PostTypeKeyPage     PostTypeKey = "pages"
	PostTypeKeyArticle  PostTypeKey = "articles"
	PostTypeKeyNote     PostTypeKey = "notes"
	PostTypeKeyLink     PostTypeKey = "links"
	PostTypeKeyBookmark PostTypeKey = "bookmarks"
	PostTypeKeyAny      PostTypeKey = "any"
)

// String returns the string representation of the PostTypeKey.
func (ptk PostTypeKey) String() string {
	return string(ptk)
}

// PostType defines a rule for determining the type of post based on its path and frontmatter.
type PostType struct {
	TypeKey          PostTypeKey               `json:"type"`
	DirPattern       string                    `json:"dirPattern"`
	FrontmatterCheck func(map[string]any) bool `json:"-"`
}

// IsValidTypeKey returns true if the given key is a valid PostTypeKey in the PostTypesMap.
func (ptm PostTypesMap) IsValidTypeKey(key string) bool {
	_, ok := ptm[PostTypeKey(key)]
	return ok
}

// PostTypeFromPath returns the PostType for the given file path. If no rule matches, an empty PostType is returned.
func (ptm PostTypesMap) PostTypeFromPath(filePath string) PostType {
	for _, rule := range ptm {
		if strings.HasPrefix(filePath, rule.DirPattern) {
			return rule
		}
	}
	return PostType{}
}

// DefaultPostTypes returns the default post types for a DownCache instance.
// The default rules are:
// - Posts are in the "articles/" directory and have a "date" frontmatter key.
// - Pages are in the "pages/" directory and have no specific frontmatter requirements.
func DefaultPostTypes() PostTypesMap {
	return PostTypesMap{
		PostTypeKeyArticle: {
			TypeKey:    PostTypeKeyArticle,
			DirPattern: "articles/",
		},
		PostTypeKeyPage: {
			TypeKey:    PostTypeKeyPage,
			DirPattern: "pages/",
		},
		PostTypeKeyNote: {
			TypeKey:    PostTypeKeyNote,
			DirPattern: "notes/",
		},
		PostTypeKeyLink: {
			TypeKey:    PostTypeKeyLink,
			DirPattern: "links/",
		},
		PostTypeKeyBookmark: {
			TypeKey:    PostTypeKeyBookmark,
			DirPattern: "bookmarks/",
		},
	}
	//return []PostType{
	//	{
	//		TypeKey:    PostTypeKeyArticle,
	//		DirPattern: "articles/",
	//		FrontmatterCheck: func(fm map[string]any) bool {
	//			_, hasDate := fm["date"]
	//			return hasDate
	//		},
	//	},
	//	{
	//		TypeKey:    PostTypeKeyPage,
	//		DirPattern: "pages/",
	//		FrontmatterCheck: func(fm map[string]any) bool {
	//			return true // All posts in the pages directory are considered pages
	//		},
	//	},
	//}
}

//// TypeRuleFromPath returns the appropriate PostType for the given file path. If no rule matches, an empty PostType is returned.
//func TypeRuleFromPath(filePath string, postTypes []PostType) PostType {
//	for _, rule := range postTypes {
//		if strings.HasPrefix(filePath, rule.DirPattern) {
//			return rule
//		}
//	}
//	return PostType{}
//}
