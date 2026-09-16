package db

import "time"

type Subnet struct {
	ID          int64     `json:"id"`
	CIDR        string    `json:"cidr"`
	Description string    `json:"description"`
	VLAN        string    `json:"vlan"`
	CreatedAt   time.Time `json:"created_at"`
}

type Address struct {
	ID         int64      `json:"id"`
	SubnetID   int64      `json:"subnet_id"`
	IP         string     `json:"ip"`
	Hostname   string     `json:"hostname"`
	MAC        string     `json:"mac"`
	Status     string     `json:"status"` // free, used, reserved
	Owner      string     `json:"owner"`
	Notes      string     `json:"notes"`
	LastSeen   *time.Time `json:"last_seen,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at"`
	CreatedAt  time.Time  `json:"created_at"`
}
