package downcache_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hypergopher/downcache"
)

type TestPost struct {
	Slug              *string              `json:"slug"`              // Slug is the URL-friendly version of the name
	PostType          *string              `json:"postType"`          // PostType is the type of post (e.g. post, page)
	Author            *string              `json:"author"`            // Author is a list of authors
	Content           *string              `json:"content"`           // Content is the HTML content of the post
	ETag              *string              `json:"etag"`              // ETag is the entity tag
	EstimatedReadTime *string              `json:"estimatedReadTime"` // EstimatedReadTime is the estimated reading time
	Featured          *bool                `json:"featured"`          // Pinned is true if the post is featured
	Photo             *string              `json:"photo"`             // Photo is the URL of the featured image
	FileTimePath      *string              `json:"fileTimePath"`      // FileTimePath is the file time path in the format YYYY-MM-DD for the original file path
	Updated           *string              `json:"updated"`           // Updated is the last modified date
	Name              *string              `json:"name"`              // Name is the name/title of the post
	Properties        *map[string]string   `json:"properties"`        // Properties is a map of additional, arbitrary key-value pairs. This can be used to store additional metadata such as extra microformat properties.
	Published         *string              `json:"published"`         // Published is the published date
	Status            *string              `json:"status"`            // Status is the status of the post (should be one of draft, published, or archived)
	Subtitle          *string              `json:"subtitle"`          // Subtitle is the subtitle
	Summary           *string              `json:"summary"`           // Summary is the summary
	Taxonomies        *map[string][]string `json:"taxonomies"`        // Taxonomies is a map of taxonomies (e.g. tags, categories)
	Visibility        *string              `json:"visibility"`        // Visibility is the visibility of the post (should be one of public, private, or unlisted)
}

// mergePost merges two posts together, preferring the values in the base post
func mergePost(slug, fileTimePath string, basePost *downcache.Post, testPost *TestPost) *downcache.Post {
	copyPost := *basePost
	//copyPost.PostID = downcache.PostPathID(basePost.PostType, slug)
	copyPost.Slug = slug
	copyPost.FileTimePath = fileTimePath

	if testPost == nil {
		return &copyPost
	}

	if testPost.Slug != nil {
		copyPost.Slug = *testPost.Slug
	}

	if testPost.PostType != nil {
		copyPost.PostType = *testPost.PostType
	}

	if testPost.Author != nil {
		copyPost.Author = *testPost.Author
	}

	if testPost.Content != nil {
		copyPost.Content = *testPost.Content
	}

	if testPost.ETag != nil {
		copyPost.ETag = *testPost.ETag
	}

	if testPost.EstimatedReadTime != nil {
		copyPost.EstimatedReadTime = *testPost.EstimatedReadTime
	}

	if testPost.Featured != nil {
		copyPost.Pinned = *testPost.Featured
	}

	if testPost.Photo != nil {
		copyPost.Photo = *testPost.Photo
	}

	if testPost.FileTimePath != nil {
		copyPost.FileTimePath = *testPost.FileTimePath
	}

	if testPost.Updated != nil {
		copyPost.Updated = *testPost.Updated
	}

	if testPost.Name != nil {
		copyPost.Name = *testPost.Name
	}

	if testPost.Properties != nil {
		copyPost.Properties = *testPost.Properties
	}

	if testPost.Published != nil {
		copyPost.Published = sql.NullString{String: *testPost.Published, Valid: true}
	}

	if testPost.Status != nil {
		copyPost.Status = *testPost.Status
	}

	if testPost.Subtitle != nil {
		copyPost.Subtitle = *testPost.Subtitle
	}

	if testPost.Summary != nil {
		copyPost.Summary = *testPost.Summary
	}

	if testPost.Taxonomies != nil {
		copyPost.Taxonomies = *testPost.Taxonomies
	}

	if testPost.Visibility != nil {
		copyPost.Visibility = *testPost.Visibility
	}

	return &copyPost
}

func TestSerializeDeserialize(t *testing.T) {
	post := &downcache.Post{
		Slug:     "test",
		PostType: "articles",
		Author:   "author1",
	}
	bytes, err := post.Serialize()
	assert.NoError(t, err)
	assert.NotNil(t, bytes)
	deserializedPost, err := downcache.Deserialize(bytes)
	assert.NoError(t, err)
	assert.Equal(t, post, deserializedPost)
}

func TestPostMeta_Validate(t *testing.T) {
	tests := []struct {
		name          string
		meta          downcache.PostMeta
		expectedError error
	}{
		{
			name: "Valid",
			meta: downcache.PostMeta{
				Status:     "published",
				Visibility: "public",
			},
			expectedError: nil,
		},
		{
			name: "InvalidStatus",
			meta: downcache.PostMeta{
				Status:     "status",
				Visibility: "public",
			},
			expectedError: downcache.ErrInvalidPostMeta,
		},
		{
			name: "InvalidVisibility",
			meta: downcache.PostMeta{
				Status:     "published",
				Visibility: "visibility",
			},
			expectedError: downcache.ErrInvalidPostMeta,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.meta.Validate()
			if test.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, test.expectedError)
			}
		})
	}
}

func TestPostMethods(t *testing.T) {
	author1 := "author1"
	author2 := "author2"
	updatedTime := time.Now().Format("2006-01-02")
	publishedTime := "2024-08-01"
	baselinePost :=
		&downcache.Post{
			Slug:              "test",
			PostType:          "articles",
			Author:            "author1",
			Content:           "<h1>Test</h1>",
			ETag:              "",
			EstimatedReadTime: "< 1 min",
			Pinned:            false,
			Photo:             "/path/to/photo.jpg",
			FileTimePath:      "",
			Updated:           updatedTime,
			Name:              "Test",
			Properties:        map[string]string{},
			Published:         sql.NullString{String: publishedTime, Valid: true},
			Status:            "published",
			Subtitle:          "Subtitle test",
			Summary:           "Test summary",
			Taxonomies: map[string][]string{
				"tags": {
					"tag1",
				},
			},
			Visibility: "public",
		}

	cases := []struct {
		name                 string
		postID               string
		slug                 string
		fileTimePath         string
		slugWithoutTime      string
		slugWithYear         string
		slugWithYearMonth    string
		slugWithYearMonthDay string
		hasProperties        bool
		hasName              bool
		hasSubtitle          bool
		hasSummary           bool
		hasFileTimeInSlug    bool
		fileTimeInSlug       string
		hasPublished         bool
		publishedDate        string
		publishedYear        int
		hasUpdated           bool
		hasTaxonomies        bool
		hasTaxonomy          bool
		taxonomy             []string
		hasAuthor            bool
		hasPhoto             bool
		post                 *TestPost
	}{
		{
			name:                 "NoAuthors with published date",
			postID:               "articles/test",
			slug:                 "test",
			fileTimePath:         "",
			slugWithoutTime:      "test",
			slugWithYear:         "2024/test",
			slugWithYearMonth:    "2024/08/test",
			slugWithYearMonthDay: "2024/08/01/test",
			hasProperties:        false,
			hasName:              true,
			hasSubtitle:          true,
			hasSummary:           true,
			hasFileTimeInSlug:    false,
			fileTimeInSlug:       "",
			hasPublished:         true,
			publishedDate:        "Aug 1, 2024",
			publishedYear:        2024,
			hasUpdated:           true,
			hasTaxonomies:        true,
			hasTaxonomy:          true,
			taxonomy:             []string{"tag1"},
			hasAuthor:            true,
			hasPhoto:             true,
			post: &TestPost{
				Author: &author1,
			},
		},
		{
			name:                 "WithAuthors, no tags, file time date",
			postID:               "articles/foobar/2024-08-01-test",
			slug:                 "foobar/2024-08-01-test",
			fileTimePath:         "2024-08-01",
			slugWithoutTime:      "foobar/test",
			slugWithYear:         "2024/foobar/test",
			slugWithYearMonth:    "2024/08/foobar/test",
			slugWithYearMonthDay: "2024/08/01/foobar/test",
			hasProperties:        false,
			hasName:              true,
			hasSubtitle:          true,
			hasSummary:           true,
			hasFileTimeInSlug:    true,
			fileTimeInSlug:       "2024-08-01",
			hasPublished:         true,
			publishedDate:        "Aug 1, 2024",
			publishedYear:        2024,
			hasUpdated:           true,
			hasTaxonomies:        false,
			hasTaxonomy:          false,
			taxonomy:             nil,
			hasAuthor:            true,
			hasPhoto:             true,
			post: &TestPost{
				Author:     &author2,
				Taxonomies: &map[string][]string{},
				Published:  nil,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Merge the baseline post with the test post
			post := mergePost(tc.slug, tc.fileTimePath, baselinePost, tc.post)

			// Test each post method
			//assert.Equal(t, tc.postID, post.PostID)
			assert.Equal(t, tc.slug, post.Slug)
			if tc.post.Published != nil {
				assert.Equal(t, tc.slugWithoutTime, post.SlugWithoutDate())
				assert.Equal(t, tc.slugWithYear, post.SlugWithYear())
				assert.Equal(t, tc.slugWithYearMonth, post.SlugWithYearMonth())
				assert.Equal(t, tc.slugWithYearMonthDay, post.SlugWithYearMonthDay())
				assert.Equal(t, tc.hasPublished, post.HasPublished())
				assert.Equal(t, tc.publishedDate, post.PublishedDate())
				assert.Equal(t, tc.publishedYear, post.PublishedYear())
			}
			assert.Equal(t, tc.hasProperties, post.HasProperties())
			assert.Equal(t, tc.hasName, post.HasName())
			assert.Equal(t, tc.hasSubtitle, post.HasSubtitle())
			assert.Equal(t, tc.hasSummary, post.HasSummary())
			assert.Equal(t, tc.hasFileTimeInSlug, post.HasFileTimeInSlug())
			assert.Equal(t, tc.fileTimeInSlug, post.FileTimeInSlug())
			assert.Equal(t, tc.hasUpdated, post.HasUpdated())
			assert.Equal(t, tc.hasTaxonomies, post.HasTaxonomies())
			assert.Equal(t, tc.hasTaxonomy, post.HasTaxonomy("tags"))
			assert.Equal(t, tc.taxonomy, post.Taxonomy("tags"))
			assert.Equal(t, tc.hasAuthor, post.HasAuthor())
			assert.Equal(t, tc.hasPhoto, post.HasPhoto())
		})
	}
}

func TestPageID(t *testing.T) {
	assert.Equal(t, "blog/test", downcache.PostPathID("blog", "test"))
}

func TestIsValidPostPath(t *testing.T) {
	assert.True(t, downcache.IsValidPostPath("path"))
	assert.False(t, downcache.IsValidPostPath(" "))
	assert.False(t, downcache.IsValidPostPath(""))
}
