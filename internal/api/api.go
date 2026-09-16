package api

import (
	"encoding/json"
	"log"
	"net/http"

	"vectrixipam/internal/db"
)

type API struct {
	DB *db.DB
}

func New(d *db.DB) *API {
	return &API{DB: d}
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/subnets", a.listSubnets)
	mux.HandleFunc("POST /api/subnets", a.createSubnet)
	mux.HandleFunc("DELETE /api/subnets/{id}", a.deleteSubnet)

	mux.HandleFunc("GET /api/subnets/{id}/addresses", a.listAddresses)
	mux.HandleFunc("POST /api/subnets/{id}/addresses", a.createAddress)
	mux.HandleFunc("PUT /api/addresses/{id}", a.updateAddress)
	mux.HandleFunc("DELETE /api/addresses/{id}", a.deleteAddress)

	mux.HandleFunc("POST /api/subnets/{id}/discover", a.discover)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			log.Printf("write json: %v", err)
		}
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
