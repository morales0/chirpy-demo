package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/morales0/chirpy-demo/internal/database"
)

type apiHandler struct{}

func (apiHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func main() {
	godotenv.Load()
	const port = "8080"

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("Error connecting to database")
		return
	}
	var cfg apiConfig
	cfg.db = database.New(db)
	cfg.platform = os.Getenv("PLATFORM")

	mux := http.NewServeMux()

	mux.Handle("/app/", cfg.middlewareIncrement(http.StripPrefix("/app", http.FileServer(http.Dir("./")))))
	mux.HandleFunc("POST /api/chirps", cfg.chirpHandler)
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /admin/metrics", cfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)
	mux.HandleFunc("POST /api/users", cfg.createUserHandler)
	mux.HandleFunc("GET /api/chirps/{chirpId}", cfg.getChirpHandler)
	mux.HandleFunc("GET /api/chirps", cfg.getChirpsHandler)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	server.ListenAndServe()
}
