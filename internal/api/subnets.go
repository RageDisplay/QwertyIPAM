package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"

	"vectrixipam/internal/db"
)

func (a *API) listSubnets(w http.ResponseWriter, r *http.Request) {
	subnets, err := a.DB.ListSubnets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, subnets)
}

type createSubnetRequest struct {
	CIDR        string `json:"cidr"`
	Description string `json:"description"`
	VLAN        string `json:"vlan"`
}

func (a *API) createSubnet(w http.ResponseWriter, r *http.Request) {
	var req createSubnetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if _, _, err := net.ParseCIDR(req.CIDR); err != nil {
		writeError(w, http.StatusBadRequest, "invalid CIDR: "+err.Error())
		return
	}

	id, err := a.DB.CreateSubnet(req.CIDR, req.Description, req.VLAN)
	if err != nil {
		writeError(w, http.StatusConflict, "could not create subnet (maybe it already exists): "+err.Error())
		return
	}

	subnet, err := a.DB.GetSubnet(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, subnet)
}

func (a *API) deleteSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.DB.DeleteSubnet(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseSubnetID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

var errBadID = errors.New("invalid id")

func (a *API) requireSubnet(w http.ResponseWriter, r *http.Request) (db.Subnet, bool) {
	id, err := parseSubnetID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, errBadID.Error())
		return db.Subnet{}, false
	}
	s, err := a.DB.GetSubnet(id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "subnet not found")
		return db.Subnet{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return db.Subnet{}, false
	}
	return s, true
}
