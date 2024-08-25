package downcache

import "errors"

var (
	ErrPostExists         = errors.New("post already exists")
	ErrPostNotFound       = errors.New("post not found")
	ErrInvalidPostType    = errors.New("invalid post type")
	ErrInvalidPostSlug    = errors.New("invalid post slug")
	ErrInvalidPostMeta    = errors.New("invalid post metadata")
	ErrMissingPostContent = errors.New("missing post content")
)
