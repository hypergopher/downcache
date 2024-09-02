package sqlitestore

import (
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hypergopher/downcache"
)

var ErrPostNotFound = errors.New("post not found")

type SQLiteStore struct {
	db        *sql.DB
	dbPath    string
	tableName string
}

func NewSQLiteStore(db *sql.DB, dbPath, tableName string) *SQLiteStore {
	return &SQLiteStore{db: db, dbPath: dbPath, tableName: tableName}
}

func (s *SQLiteStore) DBPath() string {
	return s.dbPath
}

// Init initializes the SQLiteStore, creating the necessary tables or indexes if they do not exist.
func (s *SQLiteStore) Init() error {
	query := `
		-- Table for holding posts
		CREATE TABLE IF NOT EXISTS ` + s.tableName + ` (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id TEXT,
			slug TEXT,
			post_type TEXT,
			author TEXT,
			content_body TEXT,
			etag TEXT,
			estimated_read_time TEXT,
			pinned INTEGER,
			photo TEXT,
			file_time_path TEXT,
			name TEXT,
			published TEXT,
			status TEXT,
			subtitle TEXT,
			summary TEXT,
			visibility TEXT,
			created TEXT DEFAULT CURRENT_TIMESTAMP,
			updated TEXT DEFAULT CURRENT_TIMESTAMP
		);

		-- Index on post_id
		CREATE UNIQUE INDEX IF NOT EXISTS ` + s.tableName + `_post_id_idx ON ` + s.tableName + `(post_id);

		-- Index on post_type and slug 
		CREATE UNIQUE INDEX IF NOT EXISTS ` + s.tableName + `_post_type_slug_idx ON ` + s.tableName + `(post_type, slug);
		
		-- Index on visibility
		CREATE INDEX IF NOT EXISTS ` + s.tableName + `_visibility_idx ON ` + s.tableName + `(visibility);

		-- Index on status
		CREATE INDEX IF NOT EXISTS ` + s.tableName + `_status_idx ON ` + s.tableName + `(status);

		-- Index on published date
		CREATE INDEX IF NOT EXISTS ` + s.tableName + `_published_idx ON ` + s.tableName + `(published);

		-- Table for properties 
		CREATE TABLE IF NOT EXISTS ` + s.tableName + `_properties (
			post_id TEXT,
			key TEXT,
			value TEXT,
			PRIMARY KEY(post_id, key),
			FOREIGN KEY(post_id) REFERENCES ` + s.tableName + `(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS ` + s.tableName + `_properties_post_id_idx ON ` + s.tableName + `_properties(post_id);
		CREATE INDEX IF NOT EXISTS ` + s.tableName + `_properties_key_idx ON ` + s.tableName + `_properties(key);

		-- Table for taxonomies
		CREATE TABLE IF NOT EXISTS ` + s.tableName + `_taxonomies (
			post_id TEXT,
			taxonomy TEXT,
			term TEXT,
			PRIMARY KEY(post_id, taxonomy, term),
			FOREIGN KEY(post_id) REFERENCES ` + s.tableName + `(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS ` + s.tableName + `_taxonomies_post_id_idx ON ` + s.tableName + `_taxonomies(post_id);
		CREATE INDEX IF NOT EXISTS ` + s.tableName + `_taxonomies_taxonomy_idx ON ` + s.tableName + `_taxonomies(taxonomy);

		-- Create virtual table for full-text search
		CREATE VIRTUAL TABLE IF NOT EXISTS ` + s.tableName + `_search USING fts5(
			name,
			subtitle,	
			content_body,
			summary,
			content='` + s.tableName + `',
			content_rowid='id'
		);

		-- Trigger to update the full-text search table	
		CREATE TRIGGER IF NOT EXISTS ` + s.tableName + `_search_ai AFTER INSERT ON ` + s.tableName + `
		BEGIN
			INSERT INTO ` + s.tableName + `_search(rowid, name, subtitle, content_body, summary)
			VALUES(new.id, new.name, new.subtitle, new.content_body, new.summary);
		END;

		CREATE TRIGGER IF NOT EXISTS ` + s.tableName + `_search_ad AFTER DELETE ON ` + s.tableName + `
		BEGIN
			INSERT INTO ` + s.tableName + `_search(` + s.tableName + `_search, rowid, name, subtitle, content_body, summary)
			VALUES('delete', old.id, old.name, old.subtitle, old.content_body, old.summary);
		END;

		CREATE TRIGGER IF NOT EXISTS ` + s.tableName + `_search_au AFTER UPDATE ON ` + s.tableName + `
		BEGIN
			INSERT INTO ` + s.tableName + `_search(` + s.tableName + `_search, rowid, name, subtitle, content_body, summary)
			VALUES('delete', old.id, old.name, old.subtitle, old.content_body, old.summary);

			INSERT INTO ` + s.tableName + `_search(rowid, name, subtitle, content_body, summary)
			VALUES(new.id, new.name, new.subtitle, new.content_body, new.summary);

			UPDATE ` + s.tableName + ` SET updated = CURRENT_TIMESTAMP WHERE id = new.id;
		END;
	`
	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) Clear() error {
	// delete all rows from tableName
	query := `DELETE FROM ` + s.tableName + `;`

	_, err := s.db.Exec(query)
	return err
}

// Create creates a new post in the database
func (s *SQLiteStore) Create(post *downcache.Post) (*downcache.Post, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	postID := downcache.PostPathID(post.PostType, post.Slug)

	query := `
		REPLACE INTO ` + s.tableName + ` (
			post_id, name, slug, post_type, 
			author, content_body, etag, estimated_read_time, 
			pinned, photo, file_time_path, published, 
			status, subtitle, summary, visibility) 
		VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, $12,
			$13, $14, $15, $16)
	`
	result, err := tx.Exec(query,
		postID, post.Name, post.Slug, post.PostType,
		post.Author, post.Content, post.ETag, post.EstimatedReadTime,
		post.Pinned, post.Photo, post.FileTimePath, post.Published,
		post.Status, post.Subtitle, post.Summary, post.Visibility)

	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	post.ID = id
	post.PostID = postID

	// Insert properties
	if err := s.insertProperties(tx, post); err != nil {
		return nil, err
	}

	// Insert taxonomies
	if err := s.insertTaxonomies(tx, post); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return post, nil
}

func (s *SQLiteStore) Update(post *downcache.Post) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	query := `
		UPDATE ` + s.tableName + ` SET
			name = $1, slug = $2, post_type = $3,
			author = $4, content_body = $5, etag = $6, estimated_read_time = $7,
			pinned = $8, photo = $9, file_time_path = $10, published = $11,
			status = $12, subtitle = $13, summary = $14, visibility = $15
		WHERE post_id = $16 
	`
	if _, err = tx.Exec(query,
		post.Name, post.Slug, post.PostType,
		post.Author, post.Content, post.ETag, post.EstimatedReadTime,
		post.Pinned, post.Photo, post.FileTimePath, post.Published,
		post.Status, post.Subtitle, post.Summary, post.Visibility,
		post.PostID); err != nil {
		return err
	}

	// Delete existing properties
	query = `DELETE FROM ` + s.tableName + `_properties WHERE post_id = ?`
	if _, err := tx.Exec(query, post.PostID); err != nil {
		return err
	}

	// Insert properties
	if err := s.insertProperties(tx, post); err != nil {
		return err
	}

	// Delete existing taxonomies
	query = `DELETE FROM ` + s.tableName + `_taxonomies WHERE post_id = ?`
	if _, err := tx.Exec(query, post.ID); err != nil {
		return err
	}

	// Insert taxonomies
	if err := s.insertTaxonomies(tx, post); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteStore) Delete(postID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	query := `DELETE FROM ` + s.tableName + ` WHERE post_id = ?`
	if _, err := tx.Exec(query, postID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteStore) GetPostByPath(slug string) (*downcache.Post, error) {
	query := `
		SELECT
		    p.id, p.post_id, p.name, p.slug, p.post_type,
		    p.author, p.content_body, p.etag, p.estimated_read_time,
		    p.pinned, p.photo, p.file_time_path, p.published, p.status,
		    p.subtitle, p.summary, p.visibility, p.created, p.updated
		FROM ` + s.tableName + ` p
		WHERE p.post_id = ?
	`

	row := s.db.QueryRow(query, slug)
	post, err := s.scanPost(row)
	if err != nil {
		return nil, err
	}

	// Get properties for the post
	query = `SELECT KEY, VALUE FROM ` + s.tableName + `_properties WHERE post_id = ?`
	rows, err := s.db.Query(query, post.ID)
	if err != nil {
		return nil, err
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	post.Properties = make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		post.Properties[key] = value
	}

	// Get taxonomies for the post
	query = `SELECT taxonomy, term FROM ` + s.tableName + `_taxonomies WHERE post_id = ?`
	rows, err = s.db.Query(query, post.ID)
	if err != nil {
		return nil, err
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	post.Taxonomies = make(map[string][]string)
	for rows.Next() {
		var taxonomy, term string
		if err := rows.Scan(&taxonomy, &term); err != nil {
			return nil, err
		}
		post.Taxonomies[taxonomy] = append(post.Taxonomies[taxonomy], term)
	}

	return post, nil
}

func (s *SQLiteStore) GetTaxonomies() ([]string, error) {
	query := `SELECT DISTINCT taxonomy FROM ` + s.tableName + `_taxonomies`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var taxonomies []string
	for rows.Next() {
		var taxonomy string
		if err := rows.Scan(&taxonomy); err != nil {
			return nil, err
		}
		taxonomies = append(taxonomies, taxonomy)
	}

	return taxonomies, nil
}

func (s *SQLiteStore) GetTaxonomyTerms(taxonomy string) ([]string, error) {
	query := `SELECT DISTINCT term FROM ` + s.tableName + `_taxonomies WHERE taxonomy = ?`
	rows, err := s.db.Query(query, taxonomy)
	if err != nil {
		return nil, err
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var terms []string
	for rows.Next() {
		var term string
		if err := rows.Scan(&term); err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}

	return terms, nil
}

func (s *SQLiteStore) Search(opts downcache.FilterOptions) ([]*downcache.Post, error) {
	query := `
		SELECT DISTINCT
		    p.id, p.post_id, p.name, p.slug, p.post_type,
		    p.author, p.content_body, p.etag, p.estimated_read_time,
		    p.pinned, p.photo, p.file_time_path, p.published, p.status,
		    p.subtitle, p.summary, p.visibility, p.created, p.updated
		FROM ` + s.tableName + ` p
		JOIN ` + s.tableName + `_search ON p.id = ` + s.tableName + `_search.rowid
		LEFT JOIN ` + s.tableName + `_properties prop ON p.id = prop.post_id
		LEFT JOIN ` + s.tableName + `_taxonomies tax ON p.id = tax.post_id
	`

	var conditions []string
	var args []interface{}

	orderBy := "p.created DESC"

	if opts.FilterPostType != "" {
		conditions = append(conditions, "p.post_type = ?")
		args = append(args, opts.FilterPostType)
	}

	if opts.FilterStatus != "" {
		conditions = append(conditions, "p.status = ?")
		args = append(args, opts.FilterStatus)
	}

	if opts.FilterVisibility != "" {
		conditions = append(conditions, "p.visibility = ?")
		args = append(args, opts.FilterVisibility)
	}

	if opts.FilterAuthor != "" {
		conditions = append(conditions, "p.author = ?")
		args = append(args, opts.FilterAuthor)
	}

	if opts.FilterSearch != "" {
		conditions = append(conditions, s.tableName+"_search MATCH ?")
		args = append(args, opts.FilterSearch)
		orderBy = s.tableName + "_search.rank"
	}

	if opts.FilterTaxonomies != nil {
		for _, tax := range opts.FilterTaxonomies {
			conditions = append(conditions, "tax.taxonomy = ? AND tax.term = ?")
			args = append(args, tax.Key, tax.Value)
		}
	}

	if opts.FilterProperties != nil {
		for _, prop := range opts.FilterProperties {
			conditions = append(conditions, "prop.key = ? AND prop.value = ?")
			args = append(args, prop.Key, prop.Value)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " GROUP BY p.id"
	query += " ORDER BY " + orderBy

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	//var posts []*downcache.Post
	postsMap := make(map[int64]*downcache.Post)
	postIDs := make([]any, 0)
	for rows.Next() {
		post, err := s.scanPost(rows)
		if err != nil {
			return nil, err
		}
		postsMap[post.ID] = post
		//posts = append(posts, post)
		postIDs = append(postIDs, post.ID)
	}

	placeholders := strings.Trim(strings.Join(strings.Fields(strings.Repeat("?,", len(postIDs))), ","), ",")

	// Get taxonomies for the posts
	termsQuery := fmt.Sprintf(`SELECT post_id, taxonomy, term FROM `+s.tableName+`_taxonomies WHERE post_id IN (%s)`, placeholders)
	termRows, err := s.db.Query(termsQuery, postIDs...)
	if err != nil {
		return nil, err
	}

	defer func(termRows *sql.Rows) {
		_ = termRows.Close()
	}(termRows)

	for termRows.Next() {
		var postID int64
		var taxonomy, term string
		if err := termRows.Scan(&postID, &taxonomy, &term); err != nil {
			return nil, err
		}
		postsMap[postID].Taxonomies[taxonomy] = append(postsMap[postID].Taxonomies[taxonomy], term)
	}

	// Get properties for the posts
	propsQuery := fmt.Sprintf(`SELECT post_id, KEY, VALUE FROM `+s.tableName+`_properties WHERE post_id IN (%s)`, placeholders)
	propsRows, err := s.db.Query(propsQuery, postIDs...)
	if err != nil {
		return nil, err
	}

	defer func(propsRows *sql.Rows) {
		_ = propsRows.Close()
	}(propsRows)

	for propsRows.Next() {
		var postID int64
		var key, value string
		if err := propsRows.Scan(&postID, &key, &value); err != nil {
			return nil, err
		}
		postsMap[postID].Properties[key] = value
	}

	posts := slices.Collect(maps.Values(postsMap))
	return posts, nil
}

func (s *SQLiteStore) scanPost(scanner interface {
	Scan(dest ...interface{}) error
}) (*downcache.Post, error) {
	var p downcache.Post
	var properties, taxonomies string
	if err := scanner.Scan(
		&p.ID, &p.PostID, &p.Name, &p.Slug, &p.PostType,
		&p.Author, &p.Content, &p.ETag, &p.EstimatedReadTime,
		&p.Pinned, &p.Photo, &p.FileTimePath, &p.Published, &p.Status,
		&p.Subtitle, &p.Summary, &p.Visibility, &p.Created, &p.Updated,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	p.Properties = make(map[string]string)
	for _, prop := range strings.Fields(properties) {
		if prop == "" {
			continue
		}
		parts := strings.SplitN(prop, ":", 2)
		p.Properties[parts[0]] = parts[1]
	}

	p.Taxonomies = make(map[string][]string)
	for _, tax := range strings.Fields(taxonomies) {
		if tax == "" {
			continue
		}
		parts := strings.SplitN(tax, ":", 2)
		p.Taxonomies[parts[0]] = append(p.Taxonomies[parts[0]], parts[1])
	}

	return &p, nil
}

func (s *SQLiteStore) insertProperties(tx *sql.Tx, post *downcache.Post) error {
	for key, value := range post.Properties {
		query := `REPLACE INTO ` + s.tableName + `_properties (post_id, key, value) VALUES (?, ?, ?)`
		_, err := tx.Exec(query, post.ID, key, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) insertTaxonomies(tx *sql.Tx, post *downcache.Post) error {
	for taxonomy, terms := range post.Taxonomies {
		for _, term := range terms {
			query := `REPLACE INTO ` + s.tableName + `_taxonomies (post_id, taxonomy, term) VALUES (?, ?, ?)`
			_, err := tx.Exec(query, post.ID, taxonomy, term)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
