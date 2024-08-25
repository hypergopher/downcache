package downcache

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"go.abhg.dev/goldmark/frontmatter"
)

type MarkdownParserFunc func(input []byte) (*Post, error)

// DefaultMarkdownParser returns a MarkdownParserFunc that uses the default goldmark parser with the following extensions:
// - GFM
// - Typographer
// - Footnote
// - Frontmatter
// It also enables the following parser options:
// - AutoHeadingID
// - Attribute
func DefaultMarkdownParser() MarkdownParserFunc {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
			extension.Footnote,
			&frontmatter.Extender{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
		),
	)

	return func(input []byte) (*Post, error) {
		return MarkdownToPost(md, input)
	}
}

// ReadFile reads a markdown file from the filesystem and converts it to a Post.
func ReadFile(markdownParser MarkdownParserFunc, rootPath, path string, typeRule PostType) (*Post, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// convert markdown to post
	doc, err := markdownParser(file)
	if err != nil {
		return nil, fmt.Errorf("failed to convert markdown to post: %w", err)
	}

	doc.Updated = stat.ModTime()
	doc.ETag = GenerateETag(doc.Content)
	doc.EstimatedReadTime = EstimateReadingTime(doc.Content)

	slugPath := SlugifyPath(rootPath, path, typeRule)

	// If the file has a date in the path, and the post doesn't have a published date in the frontmatter,
	// set the published date to the file's date.
	if slugPath.FileTime != nil && !doc.HasPublished() {
		doc.Published = *slugPath.FileTime
	}

	doc.Slug = slugPath.Slug
	doc.PostType = string(typeRule.TypeKey)
	doc.FileTimePath = slugPath.FileTimePath

	return doc, nil
}

// GenerateETag generates an ETag for the content.
func GenerateETag(content string) string {
	hash := sha256.New()
	hash.Write([]byte(content))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// EstimateReadingTime estimates the reading time of the content.
func EstimateReadingTime(content string) string {
	// Define reading speed in words per minute
	const wordsPerMinute = float64(200)

	// Count the number of words in the content
	words := float64(len(strings.Fields(content)))

	// Calculate the estimated reading time in minutes
	minutes := words / wordsPerMinute

	// Return the estimated reading time in minutes
	if minutes < 1 {
		return "< 1 min"
	} else if minutes < 60 {
		return fmt.Sprintf("%d min", int(minutes))
	} else {
		hours := minutes / 60
		minutes = minutes - (hours * 60)
		return fmt.Sprintf("%d hr %d min", int(hours), int(minutes))
	}
}

// MarkdownToPost converts markdown content to a Post.
func MarkdownToPost(md goldmark.Markdown, content []byte) (*Post, error) {
	var buf bytes.Buffer
	ctx := parser.NewContext()
	if err := md.Convert(content, &buf, parser.WithContext(ctx)); err != nil {
		return nil, fmt.Errorf("failed to convert markdown: %w", err)
	}

	html := buf.String()
	meta := PostMeta{}
	data := frontmatter.Get(ctx)
	if data == nil {
		// No frontmatter found
		return &Post{
			Content: html,
		}, nil
	}

	if err := data.Decode(&meta); err != nil {
		return &Post{
			Content: html,
		}, fmt.Errorf("failed to decode frontmatter: %w", err)
	}

	if meta.Status == "" {
		meta.Status = "published"
	}

	if meta.Visibility == "" {
		meta.Visibility = "public"
	}

	return &Post{
		Authors:    meta.Authors,
		Content:    html,
		Featured:   meta.Featured,
		Photo:      meta.Photo,
		Properties: meta.Properties,
		Published:  meta.Published,
		Status:     meta.Status,
		Subtitle:   meta.Subtitle,
		Summary:    meta.Summary,
		Taxonomies: meta.Taxonomies,
		Name:       meta.Name,
		Visibility: meta.Visibility,
	}, nil
}
