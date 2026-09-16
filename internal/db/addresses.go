package db

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"sort"
	"time"
)

func (d *DB) ListAddresses(subnetID int64) ([]Address, error) {
	rows, err := d.Query(`
		SELECT id, subnet_id, ip, hostname, mac, status, owner, notes, last_seen, updated_at, created_at
		FROM addresses WHERE subnet_id = ?`, subnetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Address
	for rows.Next() {
		var a Address
		if err := rows.Scan(&a.ID, &a.SubnetID, &a.IP, &a.Hostname, &a.MAC, &a.Status, &a.Owner, &a.Notes, &a.LastSeen, &a.UpdatedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort numerically by IP (e.g. .16 before .95 before .230) rather than
	// lexicographically, which ORDER BY ip in SQL would do on the raw string.
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(ipSortKey(out[i].IP), ipSortKey(out[j].IP)) < 0
	})

	return out, nil
}

// ipSortKey returns a comparable byte representation of an IP address,
// falling back to the raw string if it doesn't parse as one.
func ipSortKey(ip string) []byte {
	if parsed := net.ParseIP(ip); parsed != nil {
		if v4 := parsed.To4(); v4 != nil {
			return v4
		}
		return parsed
	}
	return []byte(ip)
}

func (d *DB) CreateAddress(a Address) (int64, error) {
	res, err := d.Exec(`
		INSERT INTO addresses (subnet_id, ip, hostname, mac, status, owner, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.SubnetID, a.IP, a.Hostname, a.MAC, a.Status, a.Owner, a.Notes)
	if err != nil {
		return 0, fmt.Errorf("insert address: %w", err)
	}
	return res.LastInsertId()
}

func (d *DB) UpdateAddress(a Address) error {
	res, err := d.Exec(`
		UPDATE addresses SET hostname = ?, mac = ?, status = ?, owner = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		a.Hostname, a.MAC, a.Status, a.Owner, a.Notes, a.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) DeleteAddress(id int64) error {
	_, err := d.Exec(`DELETE FROM addresses WHERE id = ?`, id)
	return err
}

// UpsertDiscovered inserts a discovered host, or updates last_seen/hostname/mac if it already exists.
func (d *DB) UpsertDiscovered(subnetID int64, ip, hostname, mac string) error {
	now := time.Now()
	_, err := d.Exec(`
		INSERT INTO addresses (subnet_id, ip, hostname, mac, status, last_seen)
		VALUES (?, ?, ?, ?, 'used', ?)
		ON CONFLICT(subnet_id, ip) DO UPDATE SET
			hostname = CASE WHEN excluded.hostname != '' THEN excluded.hostname ELSE addresses.hostname END,
			mac = CASE WHEN excluded.mac != '' THEN excluded.mac ELSE addresses.mac END,
			last_seen = excluded.last_seen,
			updated_at = CURRENT_TIMESTAMP`,
		subnetID, ip, hostname, mac, now)
	return err
}

func (d *DB) GetAddress(id int64) (Address, error) {
	var a Address
	err := d.QueryRow(`
		SELECT id, subnet_id, ip, hostname, mac, status, owner, notes, last_seen, updated_at, created_at
		FROM addresses WHERE id = ?`, id).
		Scan(&a.ID, &a.SubnetID, &a.IP, &a.Hostname, &a.MAC, &a.Status, &a.Owner, &a.Notes, &a.LastSeen, &a.UpdatedAt, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}
