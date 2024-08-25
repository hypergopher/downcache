package downcache

import (
	"fmt"
	"slices"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"go.etcd.io/bbolt"
)

type matchOptions struct {
	field string
	value string
}

// GetPost retrieves a post by its Path ID. If the post does not exist, an error will be returned.
// The Path ID should be the post's type (e.g. page, post) plus the post's slug, without the file extension.
// For example, a post with the path "post/my-first-post.md" would have a Path ID of "post/my-first-post".
func (dg *DownCache) GetPost(pathID string) (*Post, error) {
	var doc *Post
	err := dg.boltIndex.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketPosts))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		id := strings.TrimSuffix(pathID, ".md")
		docBytes := b.Get([]byte(id))
		if docBytes == nil {
			return fmt.Errorf("post not found")
		}

		var err error
		doc, err = Deserialize(docBytes)
		if err != nil {
			return fmt.Errorf("error deserializing post: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error getting post %s: %w", pathID, err)
	}
	return doc, err
}

// GetPosts retrieves a list of posts based on the provided filter options.
func (dg *DownCache) GetPosts(filter FilterOptions) (Paginator, error) {
	checkField := ""
	checkValue := ""
	var options []matchOptions
	var extraFields []string

	if filter.PageNum < 1 {
		filter.PageNum = 1
	}

	if filter.PageSize < 1 {
		filter.PageSize = 10
	}

	switch filter.FilterType {
	case FilterTypeAuthor:
		checkField = "authors"
		checkValue = filter.FilterTerm
		extraFields = append(extraFields, "authors")
		options = append(options, matchOptions{"authors", checkValue})
	case FilterTypeTaxonomy:
		checkField = fmt.Sprintf("taxonomies.%s", filter.FilterKey)
		checkValue = filter.FilterTerm
		extraFields = append(extraFields, checkField)
		options = append(options, matchOptions{checkField, checkValue})
	}

	options = append(options, matchOptions{"search", filter.FilterSearch})

	if filter.FilterStatus != FilterTypeAny.String() {
		if filter.FilterStatus != "" {
			options = append(options, matchOptions{"status", filter.FilterStatus})
		} else {
			options = append(options, matchOptions{"status", "published"})
		}
	}

	if filter.FilterVisibility != FilterTypeAny.String() {
		if filter.FilterVisibility != "" {
			options = append(options, matchOptions{"visibility", filter.FilterVisibility})
		} else {
			options = append(options, matchOptions{"visibility", "public"})
		}
	}

	postsQuery := dg.searchQuery(
		filter.FilterPostType,
		options...,
	)

	request := dg.searchRequest(postsQuery, filter.PageNum, filter.PageSize, extraFields...)
	result, docs, err := dg.postsFromSearchRequest(request, checkField, checkValue)
	if err != nil {
		return Paginator{}, fmt.Errorf("error searching for posts: %w", err)
	}

	return dg.Paginator(docs, int(result.Total), filter.PageNum, filter.PageSize, filter.SplitFeatured), err
}

func (dg *DownCache) postsFromSearchRequest(request *bleve.SearchRequest, checkField, checkValue string) (*bleve.SearchResult, []*Post, error) {
	result, err := dg.bleveIndex.Search(request)
	if err != nil {
		return nil, nil, fmt.Errorf("error searching for posts: %w", err)
	}

	docs := make([]*Post, 0, result.Total)
	for _, hit := range result.Hits {
		if checkField == "" || slices.Contains(anyToStringSlice(hit.Fields[checkField]), checkValue) {
			doc, err := dg.GetPost(hit.ID)
			if err != nil {
				return nil, nil, fmt.Errorf("error getting post %s: %w", hit.ID, err)
			}
			docs = append(docs, doc)
		}
	}

	return result, docs, nil
}

func (dg *DownCache) searchRequest(query query.Query, pageNum, pageSize int, fields ...string) *bleve.SearchRequest {
	offset := (pageNum - 1) * pageSize
	request := bleve.NewSearchRequestOptions(query, pageSize, offset, true)
	request.SortBy([]string{
		"-featured",
		"-published",
		"name",
	})
	requestFields := []string{
		"slug",
		"name",
		"postType",
		"published",
		"updated",
		"authors",
	}

	request.Fields = append(requestFields, fields...)
	return request
}

func (dg *DownCache) searchQuery(postType PostTypeKey, matches ...matchOptions) *query.ConjunctionQuery {
	queries := make([]query.Query, 0, len(matches)+1)

	if postType != PostTypeKeyAny && postType != "" {
		typeQuery := bleve.NewMatchQuery(string(postType))
		typeQuery.SetField("postType")
		queries = append(queries, typeQuery)
	}

	for _, match := range matches {
		field := strings.TrimSpace(match.field)
		value := strings.TrimSpace(match.value)

		if field == "" || value == "" {
			continue
		}

		if strings.ToLower(field) == "search" {
			search := bleve.NewQueryStringQuery(value)
			queries = append(queries, search)
			continue
		}

		termQuery := bleve.NewMatchQuery(match.value)
		termQuery.SetField(match.field)
		queries = append(queries, termQuery)
	}

	return bleve.NewConjunctionQuery(queries...)
}
