# DownCache

Status: **Experimental**

DownCache is a Go package that helps you organize, index, and search collections of Markdown files. It’s useful for projects with many Markdown posts, such as static site generators, documentation systems, or content management systems. It uses the[yuin/goldmark](￼) package to convert Markdown to HTML.

## What it does

- Indexes markdown files, including their frontmatter metadata
- It lets you search through all your markdown content
- It categorizes posts (e.g., pages, posts, custom types) based on where they're stored and what's in their frontmatter
- Supports both full and incremental reindexing to keep your index up-to-date
- Uses [BBolt](https://github.com/etcd-io/bbolt) to cache the markdown content and frontmatter for fast access
- Uses [Bleve](https://github.com/blevesearch/bleve) to index and search through the markdown content
- It can be extended with custom post types and frontmatter parsing rules
- Takes inspiration from IndieWeb microformats for frontmatter naming and structure
- Converts markdown content to HTML using the [yuin/goldmark](https://github.com/yuin/goldmark) package
- Supports creating, updating, and deleting markdown files and updating the indexes accordingly

It's designed to augment an existing Go application, so you can use it to add search and organization features to your Markdown-based projects. It doesn't handle routing, serving, or rendering HTML but can help you build those features into your application. It does not provide a user interface for managing content but can be used to build one.

## Why?

I had a series of sites to build and wanted to use markdown files to store the content. I also
wanted to search through the content and organize it. In addition, I wanted to add custom metadata to the Markdown files and make that metadata searchable. This package enables me to keep the posts in plaintext markdown files and still search through them from a web interface without needing a separate database or external search engine. The Markdown indexing and searching are ephemeral and can be rebuilt at any time. 

### Issues I wanted to address:

1. Keep all content in plaintext markdown files, as the source of truth
2. Use a fast, embedded database for caching converted content and metadata
3. Find specific content across many files quickly (e.g., searching full-text, tags, or other metadata)
4. Organize posts based on their type or other metadata
5. Handle different types of posts (like articles, pages, notes, bookmarks?) in one system
6. Use microformats for frontmatter to make it easier to work with the data
7. Convert markdown content to HTML for display
8. Have the ability to create, update, and delete Markdown posts programmatically, and have the indexes updated accordingly

## Getting started

Here's a quick example of how to use it:

```go
package main

import "github.com/hypergopher/downcache"

func main() {
	// A directory with markdown files
	markPath := "/path/to/markdown"

	// A directory to store the bbolt and bleve indexes
	dataPath := "/path/to/data"

	// A set of authors to associate with the markdown files
	authors := map[string]downcache.Author{
		"author1": {
			Name:      "Author 1",
			AvatarURL: "/images/author1.jpg",
			Links: []downcache.AuthorLink{
				{
					Name: "Mastodon",
					Icon: "mastodon",
					URL:  "https://example.social/@author1",
				},
			},
		},
	}

	// A set of taxonomies to associate with the markdown files
	taxonomies := map[string]string{
		"tags":       "tag",
		"categories": "category",
	}

	hd, err := downcache.NewDownCache(downcache.Options{
		MarkDir:      markPath,
		DataDir:      dataPath,
		Authors:      authors,
		Taxonomies:   taxonomies,
		ClearIndexes: true,
		Reindex:      true,
		Logger:       nil,
	})

	defer hd.Close()

	// Index everything
	hd.Reindex()

	// Get a post
	paginator, err := hd.GetPost("path/to/post-slug")

	// Get all articles (paginated)
	paginator, err := hd.GetPosts(downcache.FilterOptions{
		PageNum:              1,
		PageSize:             10,
		FilterByPostType: downcache.PostTypePost,
	})

	// Search for posts
	paginator, err := hd.GetPosts(downcache.FilterOptions{
		PageNum:              1,
		PageSize:             10,
		FilterByPostType: downcache.PostTypePost,
		FilterBySearch:       "your search query",
	})

	// Get posts by tag
	paginator, err := hd.GetPosts(downcache.FilterOptions{
		PageNum:    1,
		PageSize:   10,
		FilterType: downcache.FilterTypeTaxonomy,
		FilterKey:  "tags",
		FilterTerm: "tag3",
	})

	// Get posts by author
	paginator, err := hd.GetPosts(downcache.FilterOptions{
		PageNum:    1,
		PageSize:   10,
		FilterType: downcache.FilterTypeAuthor,
		FilterTerm: "author1",
	})
}
```

## Where you might use this

- In a static site generator to add search and help organize content
- For a documentation system to manage and search through lots of docs
- As part of a content management system for handling blog posts or articles
- To create searchable collections of markdown-based knowledge articles

## License

This project is under the Apache 2.0 License - check out the [LICENSE](LICENSE) file for details.

## Frontmatter

The frontmatter for each markdown file can be in YAML or TOML format. Here's an example of what it might look like:

```yaml 
---
name: "Page 1"
summary: "Page 1 summary"
status: "published"
published: "2021-01-01T00:00:00Z"
authors:
  - author1
taxonomies:
  categories:
    - cat1
    - cat2
  tags:
    - tag1
    - tag2
---
```

```toml
+++
name = "Page 1"
summary = "Page 1 summary"
status = "published"
published = "2021-01-01T00:00:00Z"
authors = ["author1"]

[taxonomies]
categories = ["cat1", "cat2"]
tags = ["tag1", "tag2"]

[properties]
key1 = "value1"
key2 = "value2"
+++
```

### Available frontmatter fields

Frontmatter fields adhere to the [h-entry](https://indieweb.org/h-entry) microformat. The following fields are available:

- `authors` (array of strings): The authors of the post. Each string represents a key in the `Authors` map passed into DownCache.
- `featured` (bool): Whether the post is featured
- `photo` (string): The URL of a featured image
- `name` (string): The name/title of the post
- `properties` (map[string]any): Arbitrary key-value pairs for additional metadata, such as extra microformat properties.
- `published` (time.Time): The time the post was published (Use RFC3339 format like "2006-01-02T00:00:00Z" or " 2006-01-02")
- `status` (string): The status of the post (draft or published). If empty, the post is considered published.
- `subtitle` (string): A subtitle for the post
- `summary` (string): A summary of the post
- `taxonomies` (map[string][]string): The taxonomies associated with the post
- `visibility` (string): The visibility of the post (public, private, or unlisted). If empty, the post is
  considered public.

When working with status (published, draft) or visibility (public, private, unlisted), it is up to the caller to
interpret these values as needed and to show/hide posts accordingly.

### (Optional) Dates in filenames

If you want to use optional dates in your filenames, you can use the following format:

```
YYYY-MM-DD-post-slug.md
```

This will allow DownCache to extract the date from the filename and use it as the published date for the post. 

If a `published` field is present in the frontmatter, it will take precedence over the date in the filename.

The published date will not be set if no date is found in the filename or frontmatter.

The slug will always have the date in the filename, but you can use the following methods to get a slug without the embedded filename date. Depending on your needs, however, this may cause conflicts if you have multiple posts with the same slug but different dates.

- `SlugWithoutDate()` on a `Post` struct. For example, `foobar/2024-08-21-post-slug` would become `foobar/post-slug`.
- `SlugWithYear()` on a `Post` struct. For example, `foobar/2024-08-21-post-slug` would become `2024/foobar/post-slug`.
- `SlugWithYearMonth()` on a `Post` struct. For example, `foobar/2024-08-21-post-slug` would become `2024/08/foobar/post-slug`.
- `SlugWithYearMonthDay()` on a `Post` struct. For example, `foobar/2024-08-21-post-slug` would become `2024/08/21/foobar/post-slug`.

## TODO

- [ ] Improve documentation
- [ ] Implement incremental reindexing 
- [x] Align better with microformat properties

## Possible future features

- [ ] Additional microformat properties
  - [ ] `location` (string): The location the entry was posted from
  - [ ] `syndication` (array of strings): URLs of syndicated copies of the entry
  - [ ] `in-reply-to` (string): URL of the post this post is in reply to
  - [ ] `repost-of` (string): URL of the post this post is a repost of
  - [ ] `like-of` (string): URL of the post this post is a like of
  - [ ] `bookmark-of` (string): URL of the post this post is a bookmark of
  - [ ] `rsvp` (string): RSVP status of the post
  - [ ] `video` (string): URL of a video related to the post
- [ ] Custom post types. This may already be handled by the the TypeRules field in the Options struct.
- [ ] Custom frontmatter parsing rules
- [ ] Custom query parsing rules
- [ ] Implement a Micropub endpoint
- [ ] Implement a JSON Feed endpoint
- [ ] Implement a RSS Feed endpoint

