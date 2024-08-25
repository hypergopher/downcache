package downcache

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"go.etcd.io/bbolt"
)

const (
	bboltFile        = "downcache.db"
	bleveFile        = "downcache.bleve"
	bucketPosts      = "posts"
	bucketTaxonomies = "taxonomies"
)

type FrontmatterFormat string

const (
	FrontmatterTOML FrontmatterFormat = "toml"
	FrontmatterYAML FrontmatterFormat = "yaml"
)

// DownCache is the main struct for interacting with the DownCache library.
// It provides methods for indexing, querying, and managing markdown posts.
type DownCache struct {
	authors           map[string]Author
	bleveIndex        bleve.Index
	boltIndex         *bbolt.DB
	dataDir           string
	frontmatterFormat FrontmatterFormat
	logger            *slog.Logger
	markDir           string
	mdParser          MarkdownParserFunc
	mu                sync.Mutex
	taxonomies        map[string]string
	postTypes         PostTypesMap
}

// Options is a struct for configuring a new DownCache instance.
type Options struct {
	Authors           map[string]Author  // Authors is a map of author IDs to Author structs.
	ClearIndexes      bool               // ClearIndexes will remove existing indexes before reindexing.
	DataDir           string             // DataDir is the directory where DownCache will store its indexes.
	FrontMatterFormat FrontmatterFormat  // FrontMatterFormat is the format used for frontmatter in markdown files. Default is TOML.
	Logger            *slog.Logger       // Logger is the logger used by DownCache. Default is a debug logger to stderr.
	MarkDir           string             // MarkDir is the directory where markdown files are stored.
	MarkdownParser    MarkdownParserFunc // MarkdownParser is the function used to parse markdown files. A default parser is used if not provided.
	Reindex           bool               // Reindex will reindex all markdown files when DownCache is initialized.
	Taxonomies        map[string]string  // Taxonomies is a map of taxonomy plural names to their singular names.
}

// NewDownCache creates a new DownCache instance with the provided options.
func NewDownCache(opts Options) (*DownCache, error) {
	if opts.MarkDir == "" || opts.DataDir == "" {
		return nil, errors.New("MarkDir and DataDir are required")
	}

	if opts.MarkdownParser == nil {
		opts.MarkdownParser = DefaultMarkdownParser()
	}

	if opts.Logger == nil {
		opts.Logger = defaultLogger()
	}

	if opts.FrontMatterFormat == "" {
		opts.FrontMatterFormat = FrontmatterTOML
	}

	if _, err := os.Stat(opts.DataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(opts.DataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	dg := &DownCache{
		authors:           opts.Authors,
		markDir:           opts.MarkDir,
		dataDir:           opts.DataDir,
		frontmatterFormat: opts.FrontMatterFormat,
		mdParser:          opts.MarkdownParser,
		taxonomies:        opts.Taxonomies,
		postTypes:         DefaultPostTypes(),
		logger:            opts.Logger,
	}

	// Before we can initialize the indexes, we need to ensure the post type directories exist
	for _, postType := range dg.postTypes {
		if postType.TypeKey == PostTypeKeyAny {
			continue
		}

		if _, err := os.Stat(filepath.Join(dg.markDir, postType.DirPattern)); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Join(dg.markDir, postType.DirPattern), 0755); err != nil {
				return nil, fmt.Errorf("failed to create post type directory: %w", err)
			}
		}
	}

	boltIndex, err := dg.initBolt()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize bbolt: %w", err)
	}
	dg.boltIndex = boltIndex

	bleveIndex, err := dg.initBleve()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize bleve: %w", err)
	}
	dg.bleveIndex = bleveIndex

	if opts.ClearIndexes {
		if err := dg.clearIndexes(); err != nil {
			return nil, fmt.Errorf("failed to clear index: %w", err)
		}
	}

	if opts.Reindex {
		_, err := dg.Reindex()
		if err != nil {
			return nil, fmt.Errorf("failed to index markdown files: %w", err)
		}
	}

	return dg, nil
}

// SetFrontmatterFormat sets the format used for frontmatter in markdown files.
func (dg *DownCache) SetFrontmatterFormat(format FrontmatterFormat) {
	dg.frontmatterFormat = format
}

// SetPostType sets a PostType for a given PostTypeKey.
func (dg *DownCache) SetPostType(typeKey PostTypeKey, postType PostType) error {
	if postType.TypeKey == PostTypeKeyAny {
		return errors.New("cannot set PostType for PostTypeKeyAny")
	}

	postType.TypeKey = typeKey
	dg.postTypes[typeKey] = postType

	// Ensure the directory exists
	if _, err := os.Stat(filepath.Join(dg.markDir, postType.DirPattern)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(dg.markDir, postType.DirPattern), 0755); err != nil {
			// remove the post type if the directory cannot be created
			delete(dg.postTypes, typeKey)
			return fmt.Errorf("failed to create post type directory: %w", err)
		}
	}

	return nil
}

func (dg *DownCache) initBolt() (*bbolt.DB, error) {
	var err error
	boltPath := filepath.Join(dg.dataDir, bboltFile)
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

func (dg *DownCache) initBleve() (bleve.Index, error) {
	index, err := bleve.Open(filepath.Join(dg.dataDir, bleveFile))
	if errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		dg.logger.Debug("Creating new bleve index")
		indexMapping := dg.defineBleveMapping()
		//index, err = bleve.NewUsing(filepath.Join(dg.dataDir, bleveFile), indexMapping, "scorch", "scorch", nil)
		index, err = bleve.NewUsing(filepath.Join(dg.dataDir, bleveFile), indexMapping, bleve.Config.DefaultIndexType, bleve.Config.DefaultKVStore, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create bleve index: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to open bleve index: %w", err)
	}

	return index, nil
}

func defaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr,
		&slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelDebug,
		}))
}

// Close closes the DownCache instance by closing the bolt and bleve indexes.
func (dg *DownCache) Close() error {
	if dg.boltIndex != nil {
		if err := dg.boltIndex.Close(); err != nil {
			return err
		}
	}

	if dg.bleveIndex != nil {
		return dg.bleveIndex.Close()
	}

	return nil
}

func (dg *DownCache) clearIndexes() error {
	if err := dg.Close(); err != nil {
		return fmt.Errorf("failed to close indexes: %w", err)
	}

	// Remove the bolt and bleve files
	boltPath := filepath.Join(dg.dataDir, bboltFile)
	blevePath := filepath.Join(dg.dataDir, bleveFile)

	if err := os.Remove(boltPath); err != nil {
		return fmt.Errorf("failed to remove bolt file: %w", err)
	}

	if err := os.RemoveAll(blevePath); err != nil {
		return fmt.Errorf("failed to remove bleve file: %w", err)
	}

	// Reinitialize the indexes
	boltIndex, err := dg.initBolt()
	if err != nil {
		return fmt.Errorf("failed to reinitialize bolt: %w", err)
	}

	bleveIndex, err := dg.initBleve()
	if err != nil {
		return fmt.Errorf("failed to reinitialize bleve: %w", err)
	}

	dg.boltIndex = boltIndex
	dg.bleveIndex = bleveIndex

	return nil
}

// Reindex re-indexes all markdown files in the MarkDir directory.
func (dg *DownCache) Reindex() (map[string]int, error) {
	indexCounts := make(map[string]int)

	// Clear existing indexes
	if err := dg.clearIndexes(); err != nil {
		return indexCounts, fmt.Errorf("failed to clear indexes: %w", err)
	}

	for _, postType := range dg.postTypes {
		count, err := dg.IndexPostType(postType)
		if err != nil {
			return indexCounts, fmt.Errorf("failed to reindex post type %s: %w", postType.TypeKey, err)
		}

		indexCounts[string(postType.TypeKey)] = count
	}

	dg.logger.Info("Reindexing complete")
	return indexCounts, nil
}

// IndexPostType indexes all posts of a given type.
func (dg *DownCache) IndexPostType(postType PostType) (int, error) {
	indexCount := 0
	currentPath := ""
	markdownPath := filepath.Join(dg.markDir, postType.DirPattern)

	err := filepath.WalkDir(markdownPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		currentPath = path

		if !d.IsDir() && filepath.Ext(path) == ".md" {
			doc, err := ReadFile(dg.mdParser, dg.markDir, path, postType)
			if err != nil || doc == nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			if err := dg.IndexPost(doc); err != nil {
				return fmt.Errorf("failed to index post: %w", err)
			}

			indexCount++
		}

		return nil
	})

	if err != nil {
		return indexCount, fmt.Errorf("failed to walk directory %s: %w", currentPath, err)
	}

	return indexCount, nil
}

// DeIndexPost removes a post from the indexes.
func (dg *DownCache) DeIndexPost(pathID string) error {
	doc, err := dg.GetPost(pathID)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}

	if doc == nil {
		return nil
	}

	if err := dg.boltIndex.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketPosts))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		if err := b.Delete([]byte(pathID)); err != nil {
			return fmt.Errorf("failed to delete post: %w", err)
		}

		// Remove the taxonomies
		for taxonomy, terms := range doc.Taxonomies {
			for _, term := range terms {
				if err := dg.updateTaxonomyCount(tx, taxonomy, term, -1); err != nil {
					dg.logger.Error("failed to update taxonomy count",
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

	if err := dg.bleveIndex.Delete(pathID); err != nil {
		return fmt.Errorf("failed to delete post from bleve: %w", err)
	}

	return nil
}

// IndexPost indexes a post in the bolt and bleve indexes.
func (dg *DownCache) IndexPost(doc *Post) error {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	// Recover from panic
	defer func() {
		if r := recover(); r != nil {
			dg.logger.Error("panic while indexing post", slog.String("error", fmt.Sprintf("%v", r)))
		}
	}()

	currentPage, _ := dg.GetPost(doc.ID())

	err := dg.boltIndex.Update(func(tx *bbolt.Tx) error {
		if currentPage != nil {
			// Remove the existing taxonomies
			for taxonomy, terms := range currentPage.Taxonomies {
				for _, term := range terms {
					if !slices.Contains(doc.Taxonomies[taxonomy], term) {
						if err := dg.updateTaxonomyCount(tx, taxonomy, term, -1); err != nil {
							dg.logger.Error("failed to update taxonomy count",
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

		docBytes, err := doc.Serialize()
		if err != nil {
			return fmt.Errorf("failed to serialize post: %w", err)
		}

		if err := b.Put([]byte(doc.ID()), docBytes); err != nil {
			return fmt.Errorf("failed to put post in bucket: %w", err)
		}

		// Update the taxonomies
		for taxonomy, terms := range doc.Taxonomies {
			for _, term := range terms {
				if err := dg.updateTaxonomyCount(tx, taxonomy, term, 1); err != nil {
					dg.logger.Error("failed to update taxonomy count",
						slog.String("taxonomy", taxonomy),
						slog.String("error", err.Error()))
					continue
				}
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to update post in bolt: %w", err)
	}

	// Index in Bleve
	if err := dg.bleveIndex.Index(doc.ID(), doc); err != nil {
		return fmt.Errorf("failed to index post in bleve: %w", err)
	}

	return nil
}

func (dg *DownCache) updateTaxonomyCount(tx *bbolt.Tx, taxonomy, term string, delta int) error {
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

func (dg *DownCache) defineBleveMapping() *mapping.IndexMappingImpl {
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
	for _, taxonomy := range dg.taxonomies {
		taxonomyMapping.AddFieldMappingsAt(taxonomy, bleve.NewTextFieldMapping())
	}

	docMapping.AddSubDocumentMapping("taxonomies", taxonomyMapping)
	indexMapping.AddDocumentMapping("post", docMapping)

	return indexMapping
}
