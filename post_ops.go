package downcache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// CreatePost creates a new post on the filesystem and indexes it. If the post already exists, an error will be returned.
func (dg *DownCache) CreatePost(postType, path, content string, meta *PostMeta) (string, error) {
	if err := dg.validatePost(postType, path, content, meta); err != nil {
		return "", err
	}

	filePath := filepath.Join(dg.markDir, postType, path+".md")

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		return filePath, ErrPostExists
	}

	return dg.savePost(postType, path, content, meta)
}

// UpdatePost updates an existing post on the filesystem and indexes it. If the post does not exist, an error will be returned.
func (dg *DownCache) UpdatePost(postType, path, content string, meta *PostMeta) (string, error) {
	if err := dg.validatePost(postType, path, content, meta); err != nil {
		return "", err
	}

	filePath := filepath.Join(dg.markDir, postType, path+".md")

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		return filePath, ErrPostNotFound
	}

	return dg.savePost(postType, path, content, meta)
}

// DeletePost deletes a post from the filesystem and deindexes it. If the post does not exist, an error will be returned.
func (dg *DownCache) DeletePost(postType, path string) error {
	if err := dg.validatePost(postType, path, "deleting", &PostMeta{}); err != nil {
		return err
	}

	filePath := filepath.Join(dg.markDir, postType, path+".md")
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	if err := dg.DeIndexPost(PostID(postType, path)); err != nil {
		return fmt.Errorf("post was deleted but failed to deindex: %w", err)
	}

	return nil
}

func (dg *DownCache) validatePost(postType, path, content string, meta *PostMeta) error {
	if !dg.postTypes.IsValidTypeKey(string(postType)) {
		return ErrInvalidPostType
	}

	if !IsValidPostPath(path) {
		return ErrInvalidPostSlug
	}

	if strings.TrimSpace(content) == "" {
		return ErrMissingPostContent
	}

	if meta != nil {
		if err := meta.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// savePost saves a post to the filesystem and indexes it. If the post already exists, it will be overwritten.
func (dg *DownCache) savePost(postType, path, content string, meta *PostMeta) (string, error) {
	filePath := filepath.Join(dg.markDir, postType, path+".md")

	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if meta == nil {
		meta = &PostMeta{}
	}

	// Update post metadata
	meta.Updated = time.Now()

	// Generate frontmatter
	frontmatter, err := dg.generateFrontmatter(meta)
	if err != nil {
		return filePath, fmt.Errorf("failed to generate frontmatter: %w", err)
	}

	// Combine frontmatter and content
	var fileContent string
	switch dg.frontmatterFormat {
	case FrontmatterYAML:
		fileContent = fmt.Sprintf("---\n%s---\n\n%s", frontmatter, content)
	case FrontmatterTOML:
		fileContent = fmt.Sprintf("+++\n%s+++\n\n%s", frontmatter, content)
	}

	// Write to file
	if err := os.WriteFile(filePath, []byte(fileContent), 0644); err != nil {
		return filePath, fmt.Errorf("failed to write file: %w", err)
	}

	doc := &Post{
		ID:                PostID(postType, path),
		Slug:              path,
		PostType:          postType,
		Author:            meta.Authors,
		Content:           content,
		ETag:              GenerateETag(content),
		EstimatedReadTime: EstimateReadingTime(content),
		Pinned:            meta.Pinned,
		Photo:             meta.Photo,
		FileTimePath:      path,
		Updated:           meta.Updated,
		Name:              meta.Name,
		Properties:        meta.Properties,
		Published:         meta.Published,
		Status:            meta.Status,
		Subtitle:          meta.Subtitle,
		Summary:           meta.Summary,
		Taxonomies:        meta.Taxonomies,
		Visibility:        meta.Visibility,
	}

	err = dg.IndexPost(doc)
	if err != nil {
		return filePath, fmt.Errorf("post was saved but failed to index: %w", err)
	}

	return filePath, nil
}

func (dg *DownCache) generateFrontmatter(meta *PostMeta) (string, error) {
	var frontmatter strings.Builder

	if meta == nil {
		return "", nil
	}

	switch dg.frontmatterFormat {
	case FrontmatterYAML:
		yamlData, err := yaml.Marshal(meta)
		if err != nil {
			return "", fmt.Errorf("failed to marshal YAML frontmatter: %w", err)
		}
		frontmatter.Write(yamlData)

	case FrontmatterTOML:
		encoder := toml.NewEncoder(&frontmatter)
		if err := encoder.Encode(meta); err != nil {
			return "", fmt.Errorf("failed to marshal TOML frontmatter: %w", err)
		}

	default:
		return "", fmt.Errorf("unsupported frontmatter format: %s", dg.frontmatterFormat)
	}

	return frontmatter.String(), nil
}
