package downcache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileSystemManager handles file system operations for markdown files
type FileSystemManager interface {
	Walk(ctx context.Context) (<-chan *Post, <-chan error)
	Read(ctx context.Context, postType, slug string) (*Post, error)
	Write(ctx context.Context, post *Post) error
	Delete(ctx context.Context, postType, slug string) error
	Move(ctx context.Context, oldType, oldSlug, newType, newSlug string) error
}

// LocalFileSystemManager implements FileSystemManager for the local file system
type LocalFileSystemManager struct {
	rootDir string
	proc    MarkdownProcessor
	format  FrontmatterFormat
}

func NewLocalFileSystemManager(rootDir string, proc MarkdownProcessor, format FrontmatterFormat) *LocalFileSystemManager {
	return &LocalFileSystemManager{rootDir: rootDir, proc: proc, format: format}
}

func (fs *LocalFileSystemManager) Walk(ctx context.Context) (<-chan *Post, <-chan error) {
	posts := make(chan *Post)
	errs := make(chan error, 1)

	go func() {
		defer close(posts)
		defer close(errs)

		err := filepath.Walk(fs.rootDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}

			relPath, err := filepath.Rel(fs.rootDir, path)
			if err != nil {
				return err
			}

			parts := strings.Split(relPath, string(os.PathSeparator))
			if len(parts) < 2 {
				return fmt.Errorf("invalid file path structure: %s", relPath)
			}

			postType := parts[0]
			slug := SlugifyPath(fs.rootDir, path, PostType(postType))

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			post, err := fs.proc.Process(content)
			if err != nil {
				return fmt.Errorf("error processing markdown file %s: %w", path, err)
			}

			post.PostType = postType
			post.Slug = slug.Slug
			post.Created = info.ModTime().String()
			post.Updated = info.ModTime().String()

			select {
			case posts <- post:
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		})

		if err != nil {
			errs <- err
		}
	}()

	return posts, errs
}

func (fs *LocalFileSystemManager) Read(_ context.Context, postType, slug string) (*Post, error) {
	path := fs.buildPath(postType, slug)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	post, err := fs.proc.Process(content)
	if err != nil {
		return nil, fmt.Errorf("error processing markdown file %s: %w", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	post.PostType = postType
	post.Slug = slug
	post.Created = info.ModTime().String()
	post.Updated = info.ModTime().String()

	return post, nil
}

func (fs *LocalFileSystemManager) Write(_ context.Context, post *Post) error {
	path := fs.buildPath(post.PostType, post.Slug)

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	// Generate frontmatter
	frontmatter, err := fs.proc.GenerateFrontmatter(post.Meta(), FrontmatterYAML)
	if err != nil {
		return err
	}

	// Combine frontmatter and content
	switch fs.format {
	case FrontmatterYAML:
		post.Content = fmt.Sprintf("---\n%s---\n\n%s", frontmatter, post.Content)
	case FrontmatterTOML:
		post.Content = fmt.Sprintf("+++\n%s+++\n\n%s", frontmatter, post.Content)
	default:
		return fmt.Errorf("unsupported frontmatter format: %s", fs.format)
	}

	return os.WriteFile(path, []byte(post.Content), 0644)
}

func (fs *LocalFileSystemManager) Delete(_ context.Context, postType, slug string) error {
	path := fs.buildPath(postType, slug)
	err := os.Remove(path)
	if err != nil {
		return err
	}

	//// Remove empty directories
	//dir := filepath.Dir(path)
	//for dir != fs.rootDir {
	//	err = os.Remove(dir)
	//	if err != nil {
	//		if !os.IsNotExist(err) {
	//			return err
	//		}
	//		break
	//	}
	//	dir = filepath.Dir(dir)
	//}

	return nil
}

func (fs *LocalFileSystemManager) Move(_ context.Context, oldType, oldSlug, newType, newSlug string) error {
	oldPath := fs.buildPath(oldType, oldSlug)
	newPath := fs.buildPath(newType, newSlug)

	// Ensure the directory for the new path exists
	err := os.MkdirAll(filepath.Dir(newPath), 0755)
	if err != nil {
		return err
	}

	// Move the file
	err = os.Rename(oldPath, newPath)
	if err != nil {
		return err
	}

	//// Remove empty directories from the old path
	//oldDir := filepath.Dir(oldPath)
	//for oldDir != fs.rootDir {
	//	err = os.Remove(oldDir)
	//	if err != nil {
	//		if !os.IsNotExist(err) {
	//			return err
	//		}
	//		break
	//	}
	//	oldDir = filepath.Dir(oldDir)
	//}

	return nil
}

func (fs *LocalFileSystemManager) buildPath(postType, slug string) string {
	return filepath.Join(fs.rootDir, postType, slug+".md")
}
