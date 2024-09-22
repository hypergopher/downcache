package downcache

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gosimple/slug"
)

type SlugPath struct {
	Slug         string
	FileTimePath string
	FileTime     *time.Time
	PostType     PostType
}

func hasFileTimeInSlug(slug string) bool {
	return len(slug) > 11 && slug[4] == '-' && slug[7] == '-' && slug[10] == '-'
}

// SlugifyPath transforms a full OS path into a slugified path.
// - It trims the `rootPath` from the beginning of the `fullPath` to get the relative path.
// - The slug is then a combination of the `postType` and the relative path.
// - It removes leading and trailing slashes, and ensures no leading slash remains.
// - It finds the extension based on the last period in the path and trims it from the path.
// - If the file part of a path starts with an RFC3339 date (2006-01-02), it extracts it and removes it from the path. Note, it does not include the time part.
// - It trims the "/index" suffix if it exists.
// - It replaces all path separators with browser-compatible forward slashes.
// - Finally, it slugifies each path part using the slug package.
//
// The function returns a SlugPath struct with the slugified path, the file time path, the file time, and the post type.
func SlugifyPath(rootPath, fullPath string, postType PostType) SlugPath {
	if fullPath == "" {
		return SlugPath{}
	}

	// When we get the fullPath, it contains the full OS path, so we need to trim
	// the rootPath from the beginning to get the relative path, which is all we need
	// for the slug. The slug is then a combination of the postType and the relative path.
	slugPath := strings.TrimPrefix(fullPath, rootPath)

	// Trim and remove leading and trailing slashes
	trimmedPath := strings.TrimSpace(strings.Trim(slugPath, "/"))
	trimmedPath = strings.TrimPrefix(trimmedPath, postType.String())

	// Ensure no leading slash remains, which could happen if the original path
	// had an extra slash after the prefix (e.g., "/articles/")
	trimmedPath = strings.TrimPrefix(trimmedPath, "/")

	// Get the filepath extension
	extension := filepath.Ext(trimmedPath)
	if extension != "" {
		trimmedPath = strings.TrimSuffix(trimmedPath, extension)
	}
	slugPath = trimmedPath

	// find the last path separator
	lastPathSeparator := strings.LastIndex(slugPath, string(os.PathSeparator))

	// if the last path separator is found, get the last part of the path
	slugFile := slugPath
	if lastPathSeparator != -1 {
		slugFile = slugPath[lastPathSeparator+1:]
	}

	var fileTime *time.Time
	fileTimePath := ""
	if hasFileTimeInSlug(slugFile) {
		possibleDatePath := slugFile[:10]
		if parsedTime, err := time.Parse("2006-01-02", possibleDatePath); err == nil {
			fileTime = &parsedTime
			fileTimePath = possibleDatePath
			// slugPath = strings.TrimSuffix(slugPath[11:], "/")
		}
	}

	// If the path ends with "/index", remove it, we'll use the directory name as the slug
	if strings.HasSuffix(slugPath, "/index") {
		slugPath = strings.TrimSuffix(slugPath, "/index")
	}

	// Make sure all path separators are replaced with browser-compatible forward slashes
	slugPath = strings.ReplaceAll(slugPath, string(os.PathSeparator), "/")

	// Slugify each path part
	parts := strings.Split(slugPath, "/")
	for i, part := range parts {
		parts[i] = slug.Make(part)
	}

	// Return the slugified path, the file time path, and the file time
	return SlugPath{
		Slug:         strings.Join(parts, "/"),
		FileTime:     fileTime,
		FileTimePath: fileTimePath,
		PostType:     postType,
	}
}
