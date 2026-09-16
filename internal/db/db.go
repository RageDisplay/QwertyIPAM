package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1) // avoid sqlite "database is locked" under concurrent writes

	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &DB{conn}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS subnets (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	cidr        TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	vlan        TEXT NOT NULL DEFAULT '',
	created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS addresses (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	subnet_id   INTEGER NOT NULL REFERENCES subnets(id) ON DELETE CASCADE,
	ip          TEXT NOT NULL,
	hostname    TEXT NOT NULL DEFAULT '',
	mac         TEXT NOT NULL DEFAULT '',
	status      TEXT NOT NULL DEFAULT 'used',
	owner       TEXT NOT NULL DEFAULT '',
	notes       TEXT NOT NULL DEFAULT '',
	last_seen   DATETIME,
	updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(subnet_id, ip)
);

CREATE INDEX IF NOT EXISTS idx_addresses_subnet ON addresses(subnet_id);
`
