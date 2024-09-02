package downcache

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"go.abhg.dev/goldmark/frontmatter"
	"gopkg.in/yaml.v3"
)

type FrontmatterFormat string

const (
	FrontmatterTOML FrontmatterFormat = "toml"
	FrontmatterYAML FrontmatterFormat = "yaml"
)

// MarkdownProcessor handles markdown parsing and processing
type MarkdownProcessor interface {
	Process(input []byte) (*Post, error)
	GenerateFrontmatter(meta *PostMeta, format FrontmatterFormat) (string, error)
}

// DefaultMarkdownProcessor is the default implementation of the MarkdownProcessor interface
type DefaultMarkdownProcessor struct{}

func (d DefaultMarkdownProcessor) Process(input []byte) (*Post, error) {
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

	post, err := MarkdownToPost(md, input)
	if err != nil {
		return nil, fmt.Errorf("failed to convert markdown to post: %w", err)
	}

	return post, nil
}

func (d DefaultMarkdownProcessor) GenerateFrontmatter(meta *PostMeta, format FrontmatterFormat) (string, error) {
	var fm strings.Builder

	if meta == nil {
		return "", nil
	}

	switch format {
	case FrontmatterYAML:
		yamlData, err := yaml.Marshal(meta)
		if err != nil {
			return "", fmt.Errorf("failed to marshal YAML frontmatter: %w", err)
		}
		fm.Write(yamlData)

	case FrontmatterTOML:
		encoder := toml.NewEncoder(&fm)
		if err := encoder.Encode(meta); err != nil {
			return "", fmt.Errorf("failed to marshal TOML frontmatter: %w", err)
		}

	default:
		return "", fmt.Errorf("unsupported frontmatter format: %s", format)
	}

	return fm.String(), nil
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
	rawContent := string(content)

	if err := md.Convert(content, &buf, parser.WithContext(ctx)); err != nil {
		return nil, fmt.Errorf("failed to convert markdown: %w", err)
	}

	html := buf.String()
	meta := PostMeta{}
	data := frontmatter.Get(ctx)
	if data == nil {
		// No frontmatter found
		return &Post{
			Content: rawContent,
			HTML:    html,
		}, nil
	}

	if err := data.Decode(&meta); err != nil {
		return &Post{
			Content: rawContent,
			HTML:    html,
		}, fmt.Errorf("failed to decode frontmatter: %w", err)
	}

	if meta.Status == "" {
		meta.Status = "published"
	}

	if meta.Visibility == "" {
		meta.Visibility = "public"
	}

	return &Post{
		Author:            meta.Author,
		Content:           rawContent,
		HTML:              html,
		ETag:              GenerateETag(rawContent),
		EstimatedReadTime: EstimateReadingTime(rawContent),
		Pinned:            meta.Pinned,
		Photo:             meta.Photo,
		Properties:        meta.Properties,
		Published: sql.NullString{
			String: meta.Published,
			Valid:  strings.TrimSpace(meta.Published) != "",
		},
		Status:     meta.Status,
		Subtitle:   meta.Subtitle,
		Summary:    meta.Summary,
		Taxonomies: meta.Taxonomies,
		Name:       meta.Name,
		Visibility: meta.Visibility,
	}, nil
}
