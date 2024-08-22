package database

import "database/sql"

type ProxyStorage struct {
	db *sql.DB
}
