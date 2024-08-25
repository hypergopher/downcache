package downcache

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Post represents a Markdown post
type Post struct {
	Slug              string              `json:"slug"`              // Slug is the URL-friendly version of the name
	PostType          string              `json:"postType"`          // PostType is the type of post (e.g. post, page)
	Authors           []string            `json:"authors"`           // Authors is a list of authors
	Content           string              `json:"content"`           // Content is the HTML content of the post
	ETag              string              `json:"etag"`              // ETag is the entity tag
	EstimatedReadTime string              `json:"estimatedReadTime"` // EstimatedReadTime is the estimated reading time
	Featured          bool                `json:"featured"`          // Featured is true if the post is featured
	Photo             string              `json:"photo"`             // Photo is the URL of the featured image
	FileTimePath      string              `json:"fileTimePath"`      // FileTimePath is the file time path in the format YYYY-MM-DD for the original file path
	Updated           time.Time           `json:"updated"`           // Updated is the last modified date
	Name              string              `json:"name"`              // Name is the name/title of the post
	Properties        map[string]any      `json:"properties"`        // Properties is a map of additional, arbitrary key-value pairs. This can be used to store additional metadata such as extra microformat properties.
	Published         time.Time           `json:"published"`         // Published is the published date
	Status            string              `json:"status"`            // Status is the status of the post (should be one of draft, published, or archived)
	Subtitle          string              `json:"subtitle"`          // Subtitle is the subtitle
	Summary           string              `json:"summary"`           // Summary is the summary
	Taxonomies        map[string][]string `json:"taxonomies"`        // Taxonomies is a map of taxonomies (e.g. tags, categories)
	Visibility        string              `json:"visibility"`        // Visibility is the visibility of the post (should be one of public, private, or unlisted)
	pageID            string              // pageID is the unique identifier for the post
}

// PostMeta represents the frontmatter of a post
type PostMeta struct {
	Authors    []string            `yaml:"authors,omitempty" toml:"authors,omitempty"`
	Featured   bool                `yaml:"featured,omitempty" toml:"featured,omitempty"`
	Photo      string              `yaml:"photo,omitempty" toml:"photo,omitempty"`
	Updated    time.Time           `yaml:"updated,omitempty" toml:"updated,omitempty"`
	Name       string              `yaml:"name,omitempty" toml:"name,omitempty"`
	Properties map[string]any      `yaml:"properties,omitempty" toml:"properties,omitempty"`
	Published  time.Time           `yaml:"published,omitempty" toml:"published,omitempty"`
	Status     string              `yaml:"status,omitempty" toml:"status,omitempty"`
	Subtitle   string              `yaml:"subtitle,omitempty" toml:"subtitle,omitempty"`
	Summary    string              `yaml:"summary,omitempty" toml:"summary,omitempty"`
	Taxonomies map[string][]string `yaml:"taxonomies,omitempty" toml:"taxonomies,omitempty"`
	Visibility string              `yaml:"visibility,omitempty" toml:"visibility,omitempty"`
}

func (dm *PostMeta) Validate() error {
	// Status must be one of draft, published, or archived
	switch dm.Status {
	case "draft", "published", "archived", "":
		break
	default:
		return fmt.Errorf("%w: status '%s' is not valid", ErrInvalidPostMeta, dm.Status)
	}

	// Visibility must be one of public, private, or unlisted
	switch dm.Visibility {
	case "public", "private", "unlisted", "":
		break
	default:
		return fmt.Errorf("%w: visibility '%s' is not valid", ErrInvalidPostMeta, dm.Visibility)
	}

	return nil
}

// ID returns the unique identifier for the post
func (d *Post) ID() string {
	if d.pageID == "" {
		d.pageID = PageID(d.PostType, d.Slug)
	}
	return d.pageID
}

func IsValidPostPath(path string) bool {
	return strings.TrimSpace(path) != ""
}

// PageID returns the unique identifier for a page of the specified type and slug
func PageID(postType, slug string) string {
	return fmt.Sprintf("%s/%s", postType, slug)
}

// SlugWithoutDate returns the slug without a file time path (if it exists)
func (d *Post) SlugWithoutDate() string {
	if d.HasFileTimeInSlug() {
		// Find the last slash in the file time path
		lastSlash := strings.LastIndex(d.Slug, "/")

		// Get the last part of the slug after the last slash
		filePart := d.Slug[lastSlash+1:]

		// Find the 2006-01-02 date in the file part
		if hasFileTimeInSlug(filePart) {
			// Return the first part of the file part before the date + the rest of the slug after the date
			return d.Slug[:lastSlash] + "/" + filePart[11:]
		}
	}
	return d.Slug
}

// SlugWithYear returns the slug with the published year prepended as a directory (if it exists)
func (d *Post) SlugWithYear() string {
	if d.HasPublished() {
		return fmt.Sprintf("%d/%s", d.Published.Year(), d.SlugWithoutDate())
	}
	return d.Slug
}

// SlugWithYearMonth returns the slug with the published year and month prepended as a directory (if it exists)
func (d *Post) SlugWithYearMonth() string {
	if d.HasPublished() {
		return fmt.Sprintf("%d/%02d/%s", d.Published.Year(), d.Published.Month(), d.SlugWithoutDate())
	}
	return d.Slug
}

// SlugWithYearMonthDay returns the slug with the published year, month, and day prepended as a directory (if it exists)
func (d *Post) SlugWithYearMonthDay() string {
	if d.HasPublished() {
		return fmt.Sprintf("%d/%02d/%02d/%s", d.Published.Year(), d.Published.Month(), d.Published.Day(), d.SlugWithoutDate())
	}
	return d.Slug
}

// HasProperties returns true if the post has additional/arbitrary metadata properties
func (d *Post) HasProperties() bool {
	return len(d.Properties) > 0
}

// HasName returns true if the post has a non-empty name
func (d *Post) HasName() bool {
	return d.Name != ""
}

// HasSubtitle returns true if the post has a subtitle
func (d *Post) HasSubtitle() bool {
	return d.Subtitle != ""
}

// HasSummary returns true if the post has a summary
func (d *Post) HasSummary() bool {
	return d.Summary != ""
}

// HasFileTimeInSlug returns true if the post has a file time path. This is the date part of the original file path.
func (d *Post) HasFileTimeInSlug() bool {
	return d.FileTimePath != ""
}

// FileTimeInSlug returns the file date
func (d *Post) FileTimeInSlug() string {
	if d.HasFileTimeInSlug() {
		return d.FileTimePath[:10]
	}
	return ""
}

// HasPublished returns true if the post has a published date
func (d *Post) HasPublished() bool {
	return !d.Published.IsZero()
}

// PublishedDate returns the published date in the format Jan 2, 2006
func (d *Post) PublishedDate() string {
	if !d.HasPublished() {
		return ""
	}

	return d.Published.Format("Jan 2, 2006")
}

// PublishedYear returns the year of the published date
func (d *Post) PublishedYear() int {
	if !d.HasPublished() {
		return 0
	}

	return d.Published.Year()
}

// HasUpdated returns true if the post has a last modified date
func (d *Post) HasUpdated() bool {
	return !d.Updated.IsZero()
}

// HasAuthors returns true if the post has authors
func (d *Post) HasAuthors() bool {
	return len(d.Authors) > 0
}

// HasTaxonomies returns true if the post has taxonomies
func (d *Post) HasTaxonomies() bool {
	return d.Taxonomies != nil && len(d.Taxonomies) > 0
}

// HasTaxonomy returns true if the post has the specified taxonomy
func (d *Post) HasTaxonomy(taxonomy string) bool {
	if !d.HasTaxonomies() {
		return false
	}
	_, ok := d.Taxonomies[taxonomy]
	return ok
}

// Taxonomy returns the specified taxonomy
func (d *Post) Taxonomy(taxonomy string) []string {
	if !d.HasTaxonomy(taxonomy) {
		return nil
	}
	return d.Taxonomies[taxonomy]
}

// HasPhoto returns true if the post has a featured image
func (d *Post) HasPhoto() bool {
	return d.Photo != ""
}

// Serialize serializes the post to a byte slice
func (d *Post) Serialize() ([]byte, error) {
	return json.Marshal(d)
}

// Deserialize deserializes the byte slice to a post
func Deserialize(data []byte) (*Post, error) {
	var doc Post
	err := json.Unmarshal(data, &doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}
