package sqlitestore

import (
	"database/sql"

	"github.com/hypergopher/downcache"
)

type SQLiteStore struct {
	db        *sql.DB
	tableName string
}

func NewSQLiteStore(db *sql.DB, tableName string) *SQLiteStore {
	return &SQLiteStore{db: db, tableName: tableName}
}

// Init initializes the SQLiteStore, creating the necessary tables or indexes if they do not exist.
func (s *SQLiteStore) Init() error {
	query := `
		-- Table for holding posts
		CREATE TABLE IF NOT EXISTS ` + s.tableName + ` (
			id TEXT PRIMARY KEY,
			slug TEXT,
			post_type TEXT,
			author TEXT,
			content TEXT,
			etag TEXT,
			estimated_read_time TEXT,
			pinned BOOL,
			photo TEXT,
			file_time_path TEXT,
			name TEXT,
			published DATETIME,
			status TEXT,
			subtitle TEXT,
			summary TEXT,
			visibility TEXT,
			created DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- Index on post_type and slug 
		CREATE UNIQUE INDEX ` + s.tableName + `_post_type_slug_idx ON ` + s.tableName + `(post_type, slug);
		
		-- Index on visibility
		CREATE INDEX ` + s.tableName + `_visibility_idx ON ` + s.tableName + `(visibility);

		-- Index on status
		CREATE INDEX ` + s.tableName + `_status_idx ON ` + s.tableName + `(status);

		-- Index on published date
		CREATE INDEX ` + s.tableName + `_published_idx ON ` + s.tableName + `(published);

		-- Table for properties 
		CREATE TABLE IF NOT EXISTS ` + s.tableName + `_properties (
			post_id TEXT,
			key TEXT,
			value TEXT,
			PRIMARY KEY(post_id, key),
			FOREIGN KEY(post_id) REFERENCES ` + s.tableName + `(id) ON DELETE CASCADE
		);

		CREATE INDEX ` + s.tableName + `_properties_post_id_idx ON ` + s.tableName + `_properties(post_id);
		CREATE INDEX ` + s.tableName + `_properties_key_idx ON ` + s.tableName + `_properties(key);

		-- Table for taxonomies
		CREATE TABLE IF NOT EXISTS ` + s.tableName + `_taxonomies (
			post_id TEXT,
			key TEXT,
			value TEXT,
			PRIMARY KEY(post_id, key, value),
			FOREIGN KEY(post_id) REFERENCES ` + s.tableName + `(id) ON DELETE CASCADE
		);

		CREATE INDEX ` + s.tableName + `_taxonomies_post_id_idx ON ` + s.tableName + `_taxonomies(post_id);
		CREATE INDEX ` + s.tableName + `_taxonomies_key_idx ON ` + s.tableName + `_taxonomies(key);

		-- Create virtual table for full-text search
		CREATE VIRTUAL TABLE IF NOT EXISTS ` + s.tableName + `_search USING fts5(
			name,
			subtitle,	
			content,
			summary,
			author
		);

		-- Trigger to update the full-text search table	
		CREATE TRIGGER IF NOT EXISTS ` + s.tableName + `_search_ai AFTER INSERT ON ` + s.tableName + `
		BEGIN
			INSERT INTO ` + s.tableName + `_search(rowid, name, subtitle, content, summary, author)
			VALUES(new.id, new.name, new.subtitle, new.content, new.summary, new.author);
		END;

		CREATE TRIGGER IF NOT EXISTS ` + s.tableName + `_search_ad AFTER DELETE ON ` + s.tableName + `
		BEGIN
			INSERT INTO ` + s.tableName + `_search(` + s.tableName + `_search, rowid, name, subtitle, content, summary, author)
			VALUES('delete', old.id, old.name, old.subtitle, old.content, old.summary, old.author);
		END;

		CREATE TRIGGER IF NOT EXISTS ` + s.tableName + `_search_au AFTER UPDATE ON ` + s.tableName + `
		BEGIN
			INSERT INTO ` + s.tableName + `_search(` + s.tableName + `_search, rowid, name, subtitle, content, summary, author)
			VALUES('delete', old.id, old.name, old.subtitle, old.content, old.summary, old.author);
			INSERT INTO ` + s.tableName + `_search(rowid, name, subtitle, content, summary, author)
			VALUES(new.id, new.name, new.subtitle, new.content, new.summary, new.author);
		END;

		-- Trigger to update the updated timestamp
		CREATE TRIGGER IF NOT EXISTS ` + s.tableName + `_updated AFTER UPDATE ON ` + s.tableName + `
		BEGIN
			UPDATE ` + s.tableName + ` SET updated = CURRENT_TIMESTAMP WHERE id = old.id;
		END;	
	`
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

	query := `
		INSERT INTO ` + s.tableName + ` (
			id, name, slug, post_type, 
			author, content, etag, estimated_read_time, 
			pinned, photo, file_time_path, published, status, 
			subtitle, summary, visibility) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := tx.Exec(query,
		downcache.PostID(post.PostType, post.Slug), post.Name, post.Slug, post.PostType,
		post.Author, post.Content, post.ETag, post.EstimatedReadTime,
		post.Pinned, post.Photo, post.FileTimePath, post.Published, post.Status,
		post.Subtitle, post.Summary, post.Visibility); err != nil {
		return nil, err
	}

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
			name = ?, slug = ?, post_type = ?,
			author = ?, content = ?, etag = ?, estimated_read_time = ?,
			pinned = ?, photo = ?, file_time_path = ?, published = ?, status = ?,
			subtitle = ?, summary = ?, visibility = ?
		WHERE id = ?
	`
	if _, err = tx.Exec(query,
		post.Name, post.Slug, post.PostType,
		post.Author, post.Content, post.ETag, post.EstimatedReadTime,
		post.Pinned, post.Photo, post.FileTimePath, post.Published, post.Status,
		post.Subtitle, post.Summary, post.Visibility,
		post.ID); err != nil {
		return err
	}

	// Delete existing properties
	query = `DELETE FROM ` + s.tableName + `_properties WHERE post_id = ?`
	if _, err := tx.Exec(query, post.ID); err != nil {
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
}

func (s *SQLiteStore) Delete(post *downcache.Post) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(tx)

	query := `DELETE FROM ` + s.tableName + ` WHERE id = ?`
	if _, err := tx.Exec(query, post.ID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteStore) GetBySlug(slug string) (*downcache.Post, error) {
}

func (s *SQLiteStore) Search(opts downcache.FilterOptions) ([]*downcache.Post, error) {
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) insertProperties(tx *sql.Tx, post *downcache.Post) error {
	for key, value := range post.Properties {
		query := `INSERT INTO ` + s.tableName + `_properties (post_id, key, value) VALUES (?, ?, ?)`
		_, err := tx.Exec(query, post.ID, key, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) insertTaxonomies(tx *sql.Tx, post *downcache.Post) error {
	for key, values := range post.Taxonomies {
		for _, value := range values {
			query := `REPLACE INTO ` + s.tableName + `_taxonomies (post_id, key, value) VALUES (?, ?, ?)`
			_, err := tx.Exec(query, post.ID, key, value)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
