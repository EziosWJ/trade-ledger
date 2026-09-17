package store

import (
	"database/sql"
	"embed"
	"io/fs"
	"sort"
)

func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// V1 单机低并发：单连接避免 :memory: 多连接隔离与嵌套查询死锁。
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		return nil, err
	}
	return db, nil
}

// Migrate applies *.sql files from the given embedded FS in lexical order.
// Schema files are idempotent (CREATE IF NOT EXISTS), so re-apply is safe.
func Migrate(db *sql.DB, mig embed.FS) error {
	var files []string
	if err := fs.WalkDir(mig, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := mig.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(b)); err != nil {
			return err
		}
	}
	return nil
}
