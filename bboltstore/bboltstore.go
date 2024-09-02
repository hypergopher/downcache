package bboltstore

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"go.etcd.io/bbolt"

	"github.com/hypergopher/downcache"
)

const (
	bboltFile        = "downcache.db"
	bleveFile        = "downcache.bleve"
	bucketPosts      = "posts"
	bucketTaxonomies = "taxonomies"
)

type matchOptions struct {
	field string
	value string
}

type BBoltStore struct {
	bleveIndex bleve.Index
	boltIndex  *bbolt.DB
	dataDir    string // DataDir is the directory where DownCacheOld will store its indexes.
	logger     *slog.Logger
	mu         sync.Mutex
	taxonomies []string
}

// New creates a new DownCacheOld instance.
func New(dataDir string, logger *slog.Logger) *BBoltStore {
	return &BBoltStore{
		dataDir: dataDir,
		logger:  logger,
	}
}

// Init initializes the BBolt and Bleve indexes
func (bbs *BBoltStore) Init() error {
	boltIndex, err := bbs.initBolt()
	if err != nil {
		return fmt.Errorf("failed to initialize bbolt: %w", err)
	}
	bbs.boltIndex = boltIndex

	bleveIndex, err := bbs.initBleve()
	if err != nil {
		return fmt.Errorf("failed to initialize bleve: %w", err)
	}
	bbs.bleveIndex = bleveIndex

	return nil
}

func (bbs *BBoltStore) Clear() error {
	if err := bbs.Close(); err != nil {
		return fmt.Errorf("failed to close indexes: %w", err)
	}

	// Remove the bolt and bleve files
	boltPath := filepath.Join(bbs.dataDir, bboltFile)
	blevePath := filepath.Join(bbs.dataDir, bleveFile)

	if err := os.Remove(boltPath); err != nil {
		return fmt.Errorf("failed to remove bolt file: %w", err)
	}

	if err := os.RemoveAll(blevePath); err != nil {
		return fmt.Errorf("failed to remove bleve file: %w", err)
	}

	// Reinitialize the indexes
	boltIndex, err := bbs.initBolt()
	if err != nil {
		return fmt.Errorf("failed to reinitialize bolt: %w", err)
	}

	bleveIndex, err := bbs.initBleve()
	if err != nil {
		return fmt.Errorf("failed to reinitialize bleve: %w", err)
	}

	bbs.boltIndex = boltIndex
	bbs.bleveIndex = bleveIndex

	return nil
}

func (bbs *BBoltStore) Close() error {
	if bbs.boltIndex != nil {
		if err := bbs.boltIndex.Close(); err != nil {
			return err
		}
	}

	if bbs.bleveIndex != nil {
		return bbs.bleveIndex.Close()
	}

	return nil
}

func (bbs *BBoltStore) Create(post *downcache.Post) (*downcache.Post, error) {
	bbs.mu.Lock()
	defer bbs.mu.Unlock()

	// Recover from panic
	defer func() {
		if r := recover(); r != nil {
			bbs.logger.Error("panic while indexing post", slog.String("error", fmt.Sprintf("%v", r)))
		}
	}()

	currentPage, _ := bbs.GetBySlug(downcache.PostPathID())

	err := bbs.boltIndex.Update(func(tx *bbolt.Tx) error {
		if currentPage != nil {
			// Remove the existing taxonomies
			for taxonomy, terms := range currentPage.Taxonomies {
				for _, term := range terms {
					if !slices.Contains(post.Taxonomies[taxonomy], term) {
						if err := bbs.updateTaxonomyCount(tx, taxonomy, term, -1); err != nil {
							bbs.logger.Error("failed to update taxonomy count",
								slog.String("taxonomy", taxonomy),
								slog.String("error", err.Error()))
							continue
						}
					}
				}
			}
		}

		b := tx.Bucket([]byte(bucketPosts))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		postBytes, err := post.Serialize()
		if err != nil {
			return fmt.Errorf("failed to serialize post: %w", err)
		}

		if err := b.Put([]byte(post.PostID), postBytes); err != nil {
			return fmt.Errorf("failed to put post in bucket: %w", err)
		}

		// Update the taxonomies
		for taxonomy, terms := range post.Taxonomies {
			for _, term := range terms {
				if err := bbs.updateTaxonomyCount(tx, taxonomy, term, 1); err != nil {
					bbs.logger.Error("failed to update taxonomy count",
						slog.String("taxonomy", taxonomy),
						slog.String("error", err.Error()))
					continue
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update post in bolt: %w", err)
	}

	// Index in Bleve
	if err := bbs.bleveIndex.Index(post.PostID, post); err != nil {
		return nil, fmt.Errorf("failed to index post in bleve: %w", err)
	}

	return post, nil
}

func (bbs *BBoltStore) Delete(slug string) error {
	post, err := bbs.GetBySlug(slug)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}

	if post == nil {
		return nil
	}

	if err := bbs.boltIndex.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketPosts))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		if err := b.Delete([]byte(slug)); err != nil {
			return fmt.Errorf("failed to delete post: %w", err)
		}

		// Remove the taxonomies
		for taxonomy, terms := range post.Taxonomies {
			for _, term := range terms {
				if err := bbs.updateTaxonomyCount(tx, taxonomy, term, -1); err != nil {
					bbs.logger.Error("failed to update taxonomy count",
						slog.String("taxonomy", taxonomy),
						slog.String("error", err.Error()))
					continue
				}
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to update bolt: %w", err)
	}

	if err := bbs.bleveIndex.Delete(slug); err != nil {
		return fmt.Errorf("failed to delete post from bleve: %w", err)
	}

	return nil
}

func (bbs *BBoltStore) GetBySlug(slug string) (*downcache.Post, error) {
	var post *downcache.Post
	err := bbs.boltIndex.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketPosts))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		id := strings.TrimSuffix(slug, ".md")
		postBytes := b.Get([]byte(id))
		if postBytes == nil {
			return fmt.Errorf("post not found")
		}

		var err error
		post, err = downcache.Deserialize(postBytes)
		if err != nil {
			return fmt.Errorf("error deserializing post: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error getting post %s: %w", slug, err)
	}
	return post, err
}

// GetTaxonomies returns a list of taxonomies.
func (bbs *BBoltStore) GetTaxonomies() ([]string, error) {
	return bbs.taxonomies, nil
}

// GetTaxonomyTerms returns a list of terms for a given taxonomy.
func (bbs *BBoltStore) GetTaxonomyTerms(taxonomy string) ([]string, error) {
	var terms []string
	err := bbs.boltIndex.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketTaxonomies))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		cursor := b.Cursor()
		prefix := []byte(taxonomy + ":")
		for k, _ := cursor.Seek(prefix); k != nil && strings.HasPrefix(string(k), taxonomy); k, _ = cursor.Next() {
			term := strings.TrimPrefix(string(k), taxonomy+":")
			terms = append(terms, term)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error getting taxonomy terms: %w", err)
	}

	return terms, nil
}

func (bbs *BBoltStore) Search(filter downcache.FilterOptions) (downcache.Paginator, error) {
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
	case downcache.FilterTypeAuthor:
		checkField = "authors"
		checkValue = filter.FilterTerm
		extraFields = append(extraFields, "authors")
		options = append(options, matchOptions{"authors", checkValue})
	case downcache.FilterTypeTaxonomy:
		checkField = fmt.Sprintf("taxonomies.%s", filter.FilterKey)
		checkValue = filter.FilterTerm
		extraFields = append(extraFields, checkField)
		options = append(options, matchOptions{checkField, checkValue})
	}

	options = append(options, matchOptions{"search", filter.FilterSearch})

	if filter.FilterStatus != downcache.FilterTypeAny.String() {
		if filter.FilterStatus != "" {
			options = append(options, matchOptions{"status", filter.FilterStatus})
		} else {
			options = append(options, matchOptions{"status", "published"})
		}
	}

	if filter.FilterVisibility != downcache.FilterTypeAny.String() {
		if filter.FilterVisibility != "" {
			options = append(options, matchOptions{"visibility", filter.FilterVisibility})
		} else {
			options = append(options, matchOptions{"visibility", "public"})
		}
	}

	postsQuery := bbs.searchQuery(
		filter.FilterPostType,
		options...,
	)

	request := bbs.searchRequest(postsQuery, filter.PageNum, filter.PageSize, filter.SortBy, extraFields...)
	result, posts, err := bbs.postsFromSearchRequest(request, checkField, checkValue)
	if err != nil {
		return downcache.Paginator{}, fmt.Errorf("error searching for posts: %w", err)
	}

	return downcache.NewPaginator(posts, int(result.Total), filter.PageNum, filter.PageSize, filter.SplitPinned), err
}

func (bbs *BBoltStore) initBolt() (*bbolt.DB, error) {
	var err error
	boltPath := filepath.Join(bbs.dataDir, bboltFile)
	boltIndex, err := bbolt.Open(boltPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bbolt index: %w", err)
	}

	err = boltIndex.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketPosts))
		if err != nil {
			return fmt.Errorf("failed to create posts bucket: %w", err)
		}

		_, err = tx.CreateBucketIfNotExists([]byte(bucketTaxonomies))
		if err != nil {
			return fmt.Errorf("failed to create taxonomies bucket: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	return boltIndex, nil
}

func (bbs *BBoltStore) initBleve() (bleve.Index, error) {
	index, err := bleve.Open(filepath.Join(bbs.dataDir, bleveFile))
	if errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		bbs.logger.Debug("Creating new bleve index")
		indexMapping := bbs.defineBleveMapping()
		index, err = bleve.NewUsing(filepath.Join(bbs.dataDir, bleveFile), indexMapping, bleve.Config.DefaultIndexType, bleve.Config.DefaultKVStore, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create bleve index: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to open bleve index: %w", err)
	}

	return index, nil
}

func (bbs *BBoltStore) defineBleveMapping() *mapping.IndexMappingImpl {
	indexMapping := bleve.NewIndexMapping()
	docMapping := bleve.NewDocumentMapping()

	// To use queries, I found it was necessary to use both a TextField and a KeywordField
	docMapping.AddFieldMappingsAt("slug", bleve.NewTextFieldMapping())
	docMapping.AddFieldMappingsAt("postType", bleve.NewTextFieldMapping())
	docMapping.AddFieldMappingsAt("content", bleve.NewTextFieldMapping())
	docMapping.AddFieldMappingsAt("name", bleve.NewTextFieldMapping())
	docMapping.AddFieldMappingsAt("subtitle", bleve.NewTextFieldMapping())
	docMapping.AddFieldMappingsAt("summary", bleve.NewTextFieldMapping())
	docMapping.AddFieldMappingsAt("featured", bleve.NewBooleanFieldMapping())
	docMapping.AddFieldMappingsAt("status", bleve.NewTextFieldMapping())
	docMapping.AddFieldMappingsAt("published", bleve.NewDateTimeFieldMapping())
	docMapping.AddFieldMappingsAt("updated", bleve.NewDateTimeFieldMapping())
	docMapping.AddFieldMappingsAt("authors", bleve.NewTextFieldMapping())

	// Create a sub-mapping for taxonomies
	taxonomyMapping := bleve.NewDocumentMapping()
	for _, taxonomy := range bbs.taxonomies {
		taxonomyMapping.AddFieldMappingsAt(taxonomy, bleve.NewTextFieldMapping())
	}

	docMapping.AddSubDocumentMapping("taxonomies", taxonomyMapping)
	indexMapping.AddDocumentMapping("post", docMapping)

	return indexMapping
}

func defaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr,
		&slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelDebug,
		}))
}

func (bbs *BBoltStore) updateTaxonomyCount(tx *bbolt.Tx, taxonomy, term string, delta int) error {
	b := tx.Bucket([]byte(bucketTaxonomies))
	if b == nil {
		return fmt.Errorf("bucket not found")
	}

	count := 0
	key := []byte(fmt.Sprintf("%s:%s", taxonomy, term))
	countBytes := b.Get(key)
	if countBytes != nil {
		count = int(binary.BigEndian.Uint64(countBytes))
	}

	count += delta
	if count < 0 {
		count = 0
	}

	newCount := make([]byte, 8)
	binary.BigEndian.PutUint64(newCount, uint64(count))
	if count == 0 {
		return b.Delete(key)
	}

	return b.Put(key, newCount)
}

func (bbs *BBoltStore) postsFromSearchRequest(request *bleve.SearchRequest, checkField, checkValue string) (*bleve.SearchResult, []*downcache.Post, error) {
	result, err := bbs.bleveIndex.Search(request)
	if err != nil {
		return nil, nil, fmt.Errorf("error searching for posts: %w", err)
	}

	docs := make([]*downcache.Post, 0, result.Total)
	for _, hit := range result.Hits {
		if checkField == "" || slices.Contains(anyToStringSlice(hit.Fields[checkField]), checkValue) {
			doc, err := bbs.GetBySlug(hit.ID)
			if err != nil {
				return nil, nil, fmt.Errorf("error getting post %s: %w", hit.ID, err)
			}
			docs = append(docs, doc)
		}
	}

	return result, docs, nil
}

func (bbs *BBoltStore) searchRequest(query query.Query, pageNum, pageSize int, sortBy []string, fields ...string) *bleve.SearchRequest {
	offset := (pageNum - 1) * pageSize
	request := bleve.NewSearchRequestOptions(query, pageSize, offset, true)

	if len(sortBy) > 0 {
		request.SortBy(sortBy)
	} else {
		request.SortBy([]string{
			"-featured",
			"-published",
			"name",
		})
	}

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

func (bbs *BBoltStore) searchQuery(postType downcache.PostType, matches ...matchOptions) *query.ConjunctionQuery {
	queries := make([]query.Query, 0, len(matches)+1)

	if postType != downcache.PostTypeKeyAny && postType != "" {
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

// convertToStringSlice converts a []byte to a []string
func anyToStringSlice(value any) []string {
	if val, ok := value.(string); ok {
		return []string{val}
	} else if val, ok := value.([]any); ok {
		var result []string
		for _, v := range val {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	return []string{}
}
