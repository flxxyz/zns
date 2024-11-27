package database

import "github.com/go-kiss/sqlx"

type Sqlite struct {
	Driver
}

func (s Sqlite) Open() (*sqlx.DB, error) {
	db, err := sqlx.Connect(s.Name, s.URI+"?cache=shared&mode=rwc&_journal_mode=WAL")
	return db, err
}
