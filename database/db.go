package database

import "github.com/go-kiss/sqlx"

type IDriver interface {
	Open() (*sqlx.DB, error)
}

type Driver struct {
	Name string
	URI  string
}
