package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go"
)

// MySQLMemoryStore implements agentsdk.MemoryStore using MySQL.
//
// It uses two tables (auto-created if AutoMigrate is true):
//   - {prefix}_kv:   (namespace, key, value) for KV operations
//   - {prefix}_list: (namespace, key, idx, value) for ordered lists
type MySQLMemoryStore struct {
	db     *sql.DB
	prefix string
}

// MySQLStoreConfig configures the MySQL store.
type MySQLStoreConfig struct {
	Prefix      string // table prefix, default "memory_store"
	AutoMigrate bool   // create tables if not exist, default true
}

// NewMySQLMemoryStore creates a MemoryStore backed by MySQL.
// The sql.DB must be already opened with a MySQL driver.
func NewMySQLMemoryStore(db *sql.DB, config ...MySQLStoreConfig) (*MySQLMemoryStore, error) {
	cfg := MySQLStoreConfig{Prefix: "memory_store", AutoMigrate: true}
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "memory_store"
	}

	s := &MySQLMemoryStore{db: db, prefix: cfg.Prefix}
	if cfg.AutoMigrate {
		if err := s.migrate(); err != nil {
			return nil, fmt.Errorf("auto-migrate failed: %w", err)
		}
	}
	return s, nil
}

func (s *MySQLMemoryStore) kvTable() string   { return s.prefix + "_kv" }
func (s *MySQLMemoryStore) listTable() string { return s.prefix + "_list" }

func (s *MySQLMemoryStore) migrate() error {
	kvDDL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		namespace VARCHAR(255) NOT NULL,
		k         VARCHAR(255) NOT NULL,
		v         LONGTEXT     NOT NULL,
		PRIMARY KEY (namespace, k)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, s.kvTable())

	listDDL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id        BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		namespace VARCHAR(255) NOT NULL,
		k         VARCHAR(255) NOT NULL,
		v         LONGTEXT     NOT NULL,
		PRIMARY KEY (id),
		KEY idx_ns_key (namespace, k)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, s.listTable())

	if _, err := s.db.Exec(kvDDL); err != nil {
		return err
	}
	_, err := s.db.Exec(listDDL)
	return err
}

func (s *MySQLMemoryStore) Get(namespace, key string) (string, error) {
	var val string
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT v FROM %s WHERE namespace=? AND k=?", s.kvTable()),
		namespace, key,
	).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (s *MySQLMemoryStore) Set(namespace, key, value string) error {
	q := fmt.Sprintf(
		"INSERT INTO %s (namespace, k, v) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE v=VALUES(v)",
		s.kvTable(),
	)
	_, err := s.db.Exec(q, namespace, key, value)
	return err
}

func (s *MySQLMemoryStore) Delete(namespace, key string) error {
	_, err := s.db.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE namespace=? AND k=?", s.kvTable()),
		namespace, key,
	)
	return err
}

func (s *MySQLMemoryStore) ListKeys(namespace string) ([]string, error) {
	rows, err := s.db.Query(
		fmt.Sprintf("SELECT DISTINCT k FROM %s WHERE namespace=? UNION SELECT DISTINCT k FROM %s WHERE namespace=?",
			s.kvTable(), s.listTable()),
		namespace, namespace,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *MySQLMemoryStore) Append(namespace, key, value string) error {
	_, err := s.db.Exec(
		fmt.Sprintf("INSERT INTO %s (namespace, k, v) VALUES (?, ?, ?)", s.listTable()),
		namespace, key, value,
	)
	return err
}

func (s *MySQLMemoryStore) GetList(namespace, key string, limit, offset int) ([]string, error) {
	q := fmt.Sprintf("SELECT v FROM %s WHERE namespace=? AND k=? ORDER BY id ASC", s.listTable())
	var args []interface{}
	args = append(args, namespace, key)

	if limit > 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	} else if offset > 0 {
		q += " LIMIT 18446744073709551615 OFFSET ?"
		args = append(args, offset)
	}

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	if items == nil {
		items = []string{}
	}
	return items, rows.Err()
}

func (s *MySQLMemoryStore) TrimList(namespace, key string, maxSize int) error {
	var count int
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE namespace=? AND k=?", s.listTable()),
		namespace, key,
	).Scan(&count)
	if err != nil || count <= maxSize {
		return err
	}

	toDelete := count - maxSize
	_, err = s.db.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE id IN (SELECT id FROM (SELECT id FROM %s WHERE namespace=? AND k=? ORDER BY id ASC LIMIT ?) AS tmp)",
			s.listTable(), s.listTable()),
		namespace, key, toDelete,
	)
	return err
}

func (s *MySQLMemoryStore) ClearList(namespace, key string) error {
	_, err := s.db.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE namespace=? AND k=?", s.listTable()),
		namespace, key,
	)
	return err
}

func (s *MySQLMemoryStore) ListLength(namespace, key string) (int, error) {
	var count int
	err := s.db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE namespace=? AND k=?", s.listTable()),
		namespace, key,
	).Scan(&count)
	return count, err
}

func (s *MySQLMemoryStore) Close() error {
	return s.db.Close()
}

// BatchSet writes multiple KV pairs in a single transaction.
func (s *MySQLMemoryStore) BatchSet(namespace string, kvs map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		fmt.Sprintf("INSERT INTO %s (namespace, k, v) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE v=VALUES(v)", s.kvTable()),
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for k, v := range kvs {
		if _, err := stmt.Exec(namespace, k, v); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// DeleteNamespace removes all data for a namespace (KV + lists).
// Useful for GDPR right-to-forget compliance.
func (s *MySQLMemoryStore) DeleteNamespace(namespace string) error {
	if _, err := s.db.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE namespace=?", s.kvTable()), namespace,
	); err != nil {
		return err
	}
	_, err := s.db.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE namespace=?", s.listTable()), namespace,
	)
	return err
}

// Export returns all KV data for a namespace as a JSON-serializable map.
func (s *MySQLMemoryStore) Export(namespace string) (map[string]string, error) {
	rows, err := s.db.Query(
		fmt.Sprintf("SELECT k, v FROM %s WHERE namespace=?", s.kvTable()), namespace,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

// Import bulk-loads KV data for a namespace from a map.
func (s *MySQLMemoryStore) Import(namespace string, data map[string]string) error {
	return s.BatchSet(namespace, data)
}

// ExportJSON exports all KV data as a JSON string.
func (s *MySQLMemoryStore) ExportJSON(namespace string) (string, error) {
	data, err := s.Export(namespace)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(data)
	return string(b), err
}

// Compile-time interface check.
var _ agentsdk.MemoryStore = (*MySQLMemoryStore)(nil)
