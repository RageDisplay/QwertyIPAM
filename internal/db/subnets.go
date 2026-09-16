package db

import (
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

func (d *DB) ListSubnets() ([]Subnet, error) {
	rows, err := d.Query(`SELECT id, cidr, description, vlan, created_at FROM subnets ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Subnet
	for rows.Next() {
		var s Subnet
		if err := rows.Scan(&s.ID, &s.CIDR, &s.Description, &s.VLAN, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (d *DB) GetSubnet(id int64) (Subnet, error) {
	var s Subnet
	err := d.QueryRow(`SELECT id, cidr, description, vlan, created_at FROM subnets WHERE id = ?`, id).
		Scan(&s.ID, &s.CIDR, &s.Description, &s.VLAN, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

func (d *DB) CreateSubnet(cidr, description, vlan string) (int64, error) {
	res, err := d.Exec(`INSERT INTO subnets (cidr, description, vlan) VALUES (?, ?, ?)`, cidr, description, vlan)
	if err != nil {
		return 0, fmt.Errorf("insert subnet: %w", err)
	}
	return res.LastInsertId()
}

func (d *DB) DeleteSubnet(id int64) error {
	_, err := d.Exec(`DELETE FROM subnets WHERE id = ?`, id)
	return err
}
