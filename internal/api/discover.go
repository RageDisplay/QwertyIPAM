package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"vectrixipam/internal/scan"
)

type discoveredHost struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	MAC      string `json:"mac"`
	New      bool   `json:"new"`
}

// discover scans the subnet for live hosts via ICMP/TCP probing and upserts
// every host that responds into the addresses table.
func (a *API) discover(w http.ResponseWriter, r *http.Request) {
	subnet, ok := a.requireSubnet(w, r)
	if !ok {
		return
	}

	existing, err := a.DB.ListAddresses(subnet.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	known := make(map[string]bool, len(existing))
	for _, addr := range existing {
		known[addr.IP] = true
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	hosts, scanned, err := scan.Subnet(ctx, subnet.CIDR)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result := make([]discoveredHost, 0, len(hosts))
	for _, h := range hosts {
		isNew := !known[h.IP]
		if err := a.DB.UpsertDiscovered(subnet.ID, h.IP, h.Hostname, h.MAC); err != nil {
			log.Printf("upsert discovered host %s: %v", h.IP, err)
			continue
		}
		result = append(result, discoveredHost{IP: h.IP, Hostname: h.Hostname, MAC: h.MAC, New: isNew})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"scanned": scanned,
		"hosts":   result,
	})
}
