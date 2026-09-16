package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"vectrixipam/internal/api"
	"vectrixipam/internal/db"
)

//go:embed web/static web/templates
var webFS embed.FS

func main() {
	dbPath := getenv("VECTRIXIPAM_DB_PATH", "/data/vectrixipam.db")
	addr := getenv("VECTRIXIPAM_LISTEN_ADDR", ":8080")

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	mux := http.NewServeMux()
	a := api.New(database)
	a.Routes(mux)

	static, err := fs.Sub(webFS, "web/static")
	if err != nil {
		log.Fatalf("mount static assets: %v", err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))

	indexHTML, err := webFS.ReadFile("web/templates/index.html")
	if err != nil {
		log.Fatalf("read index.html: %v", err)
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	log.Printf("VectrixIPAM listening on %s (db: %s)", addr, dbPath)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
