package downcache

import (
	"fmt"
	"strings"

	"go.etcd.io/bbolt"
)

// GetTaxonomyTerms returns a list of terms for a given taxonomy.
func (dg *DownCache) GetTaxonomyTerms(taxonomy string) ([]string, error) {
	var terms []string
	err := dg.boltIndex.View(func(tx *bbolt.Tx) error {
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
