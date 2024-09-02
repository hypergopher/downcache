package downcache

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Post represents a Markdown post
type Post struct {
	ID                int64               `json:"id"`                // ID is the unique identifier for the post
	PostID            string              `json:"post_id"`           // PostID is the unique identifier for the post (post type + slug)
	Slug              string              `json:"slug"`              // Slug is the URL-friendly version of the name
	PostType          string              `json:"postType"`          // PostType is the type of post (e.g. post, page)
	Author            string              `json:"author"`            // Author is a list of author
	Content           string              `json:"content"`           // Content is raw content of the post
	HTML              string              `json:"html"`              // HTML is the HTML content of the post
	ETag              string              `json:"etag"`              // ETag is the entity tag
	EstimatedReadTime string              `json:"estimatedReadTime"` // EstimatedReadTime is the estimated reading time
	Pinned            bool                `json:"pinned"`            // Pinned is true if the post is pinned
	Photo             string              `json:"photo"`             // Photo is the URL of the featured image
	FileTimePath      string              `json:"fileTimePath"`      // FileTimePath is the file time path in the format YYYY-MM-DD for the original file path
	Name              string              `json:"name"`              // Name is the name/title of the post
	Properties        map[string]string   `json:"properties"`        // Properties is a map of additional, arbitrary key-value pairs. This can be used to store additional metadata such as extra microformat properties.
	Published         sql.NullString      `json:"published"`         // Published is the published date
	Status            string              `json:"status"`            // Status is the status of the post (should be one of draft, published, or archived)
	Subtitle          string              `json:"subtitle"`          // Subtitle is the subtitle
	Summary           string              `json:"summary"`           // Summary is the summary
	Taxonomies        map[string][]string `json:"taxonomies"`        // Taxonomies is a map of taxonomies (e.g. tags, categories)
	Visibility        string              `json:"visibility"`        // Visibility is the visibility of the post (should be one of public, private, or unlisted)
	Created           string              `json:"created"`           // Created is the creation date
	Updated           string              `json:"updated"`           // Updated is the last modified date
	publishedTime     time.Time           // publishedDate is the parsed published date
	pageID            string              // pageID is the unique identifier for the post
}

// PostMeta represents the frontmatter of a post
type PostMeta struct {
	Author     string              `yaml:"author,omitempty" toml:"author,omitempty"`
	Pinned     bool                `yaml:"pinned,omitempty" toml:"pinned,omitempty"`
	Name       string              `yaml:"name,omitempty" toml:"name,omitempty"`
	Photo      string              `yaml:"photo,omitempty" toml:"photo,omitempty"`
	Properties map[string]string   `yaml:"properties,omitempty" toml:"properties,omitempty"`
	Published  string              `yaml:"published,omitempty" toml:"published,omitempty"`
	Status     string              `yaml:"status,omitempty" toml:"status,omitempty"`
	Subtitle   string              `yaml:"subtitle,omitempty" toml:"subtitle,omitempty"`
	Summary    string              `yaml:"summary,omitempty" toml:"summary,omitempty"`
	Taxonomies map[string][]string `yaml:"taxonomies,omitempty" toml:"taxonomies,omitempty"`
	//Updated    time.Time           `yaml:"updated,omitempty" toml:"updated,omitempty"`
	Visibility string `yaml:"visibility,omitempty" toml:"visibility,omitempty"`
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

func IsValidPostPath(path string) bool {
	return strings.TrimSpace(path) != ""
}

// PostPathID returns the unique identifier for a page of the specified type and slug
func PostPathID(postType, slug string) string {
	return fmt.Sprintf("%s/%s", postType, slug)
}

func (p *Post) Meta() *PostMeta {
	return &PostMeta{
		Author:     p.Author,
		Pinned:     p.Pinned,
		Name:       p.Name,
		Photo:      p.Photo,
		Properties: p.Properties,
		Published:  p.Published.String,
		Status:     p.Status,
		Subtitle:   p.Subtitle,
		Summary:    p.Summary,
		Taxonomies: p.Taxonomies,
		Visibility: p.Visibility,
	}
}

// SlugWithoutDate returns the slug without a file time path (if it exists)
func (p *Post) SlugWithoutDate() string {
	if p.HasFileTimeInSlug() {
		// Find the last slash in the file time path
		lastSlash := strings.LastIndex(p.Slug, "/")

		// Get the last part of the slug after the last slash
		filePart := p.Slug[lastSlash+1:]

		// Find the 2006-01-02 date in the file part
		if hasFileTimeInSlug(filePart) {
			// Return the first part of the file part before the date + the rest of the slug after the date
			return p.Slug[:lastSlash] + "/" + filePart[11:]
		}
	}
	return p.Slug
}

// SlugWithYear returns the slug with the published year prepended as a directory (if it exists)
func (p *Post) SlugWithYear() string {
	if p.HasPublished() {
		return fmt.Sprintf("%d/%s", p.publishedTime.Year(), p.SlugWithoutDate())
	}
	return p.Slug
}

// SlugWithYearMonth returns the slug with the published year and month prepended as a directory (if it exists)
func (p *Post) SlugWithYearMonth() string {
	if p.HasPublished() {
		return fmt.Sprintf("%d/%02d/%s", p.publishedTime.Year(), p.publishedTime.Month(), p.SlugWithoutDate())
	}
	return p.Slug
}

// SlugWithYearMonthDay returns the slug with the published year, month, and day prepended as a directory (if it exists)
func (p *Post) SlugWithYearMonthDay() string {
	if p.HasPublished() {
		return fmt.Sprintf("%d/%02d/%02d/%s", p.publishedTime.Year(), p.publishedTime.Month(), p.publishedTime.Day(), p.SlugWithoutDate())
	}
	return p.Slug
}

// HasProperties returns true if the post has additional/arbitrary metadata properties
func (p *Post) HasProperties() bool {
	return len(p.Properties) > 0
}

// HasName returns true if the post has a non-empty name
func (p *Post) HasName() bool {
	return p.Name != ""
}

// HasSubtitle returns true if the post has a subtitle
func (p *Post) HasSubtitle() bool {
	return p.Subtitle != ""
}

// HasSummary returns true if the post has a summary
func (p *Post) HasSummary() bool {
	return p.Summary != ""
}

// HasFileTimeInSlug returns true if the post has a file time path. This is the date part of the original file path.
func (p *Post) HasFileTimeInSlug() bool {
	return p.FileTimePath != ""
}

// FileTimeInSlug returns the file date
func (p *Post) FileTimeInSlug() string {
	if p.HasFileTimeInSlug() {
		return p.FileTimePath[:10]
	}
	return ""
}

// HasPublished returns true if the post has a published date
func (p *Post) HasPublished() bool {
	if p.Published.Valid {
		// Attempt to parse the published date
		dt, err := time.Parse("2006-01-02", p.Published.String)
		if err == nil {
			dt, err = time.Parse("2006-01-02 15:04:05", p.Published.String)
			if err == nil {
				dt, err = time.Parse(time.RFC3339, p.Published.String)
				if err != nil {
					return false
				}
			}
		}
		p.publishedTime = dt
		return !p.publishedTime.IsZero()
	}

	return false
}

// PublishedTime returns the published date as a time.Time
func (p *Post) PublishedTime() time.Time {
	if p.HasPublished() {
		return p.publishedTime
	}
	return time.Time{}
}

// PublishedDate returns the published date in the format Jan 2, 2006
func (p *Post) PublishedDate() string {
	if !p.HasPublished() {
		return ""
	}

	return p.publishedTime.Format("Jan 2, 2006")
}

// PublishedYear returns the year of the published date
func (p *Post) PublishedYear() int {
	if !p.HasPublished() {
		return 0
	}

	return p.publishedTime.Year()
}

// HasUpdated returns true if the post has a last modified date
func (p *Post) HasUpdated() bool {
	return p.Updated != ""
}

// HasAuthor returns true if the post has author
func (p *Post) HasAuthor() bool {
	return len(p.Author) > 0
}

// HasTaxonomies returns true if the post has taxonomies
func (p *Post) HasTaxonomies() bool {
	return p.Taxonomies != nil && len(p.Taxonomies) > 0
}

// HasTaxonomy returns true if the post has the specified taxonomy
func (p *Post) HasTaxonomy(taxonomy string) bool {
	if !p.HasTaxonomies() {
		return false
	}
	_, ok := p.Taxonomies[taxonomy]
	return ok
}

// Taxonomy returns the specified taxonomy
func (p *Post) Taxonomy(taxonomy string) []string {
	if !p.HasTaxonomy(taxonomy) {
		return nil
	}
	return p.Taxonomies[taxonomy]
}

// HasPhoto returns true if the post has a featured image
func (p *Post) HasPhoto() bool {
	return p.Photo != ""
}

// Serialize serializes the post to a byte slice
func (p *Post) Serialize() ([]byte, error) {
	return json.Marshal(p)
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
