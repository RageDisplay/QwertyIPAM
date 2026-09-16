package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"

	"vectrixipam/internal/db"
)

func (a *API) listAddresses(w http.ResponseWriter, r *http.Request) {
	subnet, ok := a.requireSubnet(w, r)
	if !ok {
		return
	}
	addrs, err := a.DB.ListAddresses(subnet.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, addrs)
}

type addressRequest struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	MAC      string `json:"mac"`
	Status   string `json:"status"`
	Owner    string `json:"owner"`
	Notes    string `json:"notes"`
}

func (a *API) createAddress(w http.ResponseWriter, r *http.Request) {
	subnet, ok := a.requireSubnet(w, r)
	if !ok {
		return
	}

	var req addressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if net.ParseIP(req.IP) == nil {
		writeError(w, http.StatusBadRequest, "invalid IP address")
		return
	}
	if _, ipnet, _ := net.ParseCIDR(subnet.CIDR); ipnet != nil && !ipnet.Contains(net.ParseIP(req.IP)) {
		writeError(w, http.StatusBadRequest, "IP does not belong to this subnet")
		return
	}
	if req.Status == "" {
		req.Status = "used"
	}

	id, err := a.DB.CreateAddress(db.Address{
		SubnetID: subnet.ID,
		IP:       req.IP,
		Hostname: req.Hostname,
		MAC:      req.MAC,
		Status:   req.Status,
		Owner:    req.Owner,
		Notes:    req.Notes,
	})
	if err != nil {
		writeError(w, http.StatusConflict, "could not create address (maybe it already exists): "+err.Error())
		return
	}

	addr, err := a.DB.GetAddress(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, addr)
}

func (a *API) updateAddress(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	existing, err := a.DB.GetAddress(id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "address not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req addressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	existing.Hostname = req.Hostname
	existing.MAC = req.MAC
	existing.Status = req.Status
	existing.Owner = req.Owner
	existing.Notes = req.Notes

	if err := a.DB.UpdateAddress(existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, err := a.DB.GetAddress(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) deleteAddress(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.DB.DeleteAddress(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
