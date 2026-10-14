package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/morales0/chirpy-demo/internal/database"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
}

func (cfg *apiConfig) middlewareIncrement(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, req)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	// hits := fmt.Sprintf("Hits: %v", cfg.fileServerHits.Load())
	page := fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>
`, cfg.fileServerHits.Load())

	w.Write([]byte(page))
}
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, req *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	cfg.fileServerHits.Store(0)
	// delete all users in the database
	cfg.dbQueries.DeleteAllUsers(req.Context())
	w.WriteHeader(http.StatusOK)
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, req *http.Request) {
	// Load the email from the body of the request
	type ReqPayload struct {
		Email string `json:"email"`
	}
	var reqPayload ReqPayload
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&reqPayload)
	if err != nil {
		// TODO: Move this to a func?
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{ \"error\": \"Invalid request\" }"))
		return
	}
	email := reqPayload.Email

	// Call the CreateUser function
	newUser, err := cfg.dbQueries.CreateUser(req.Context(), email)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("{ \"error\": \"%v\" }", err)))
		return
	}

	// If successful, return the response payload
	type RespPayload struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt string    `json:"created_at"`
		UpdatedAt string    `json:"updated_at"`
		Email     string    `json:"email"`
	}
	resp := RespPayload{
		ID:        newUser.ID,
		CreatedAt: newUser.CreatedAt.String(),
		UpdatedAt: newUser.UpdatedAt.String(),
		Email:     newUser.Email,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{ \"error\": \"Failed to construct response\" }"))
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	w.Write(data)
}

type apiHandler struct{}

func (apiHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func errorResponse(msg string, w http.ResponseWriter) {
	// Write the headers
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(fmt.Sprintf("{ \"error\": \"%v\" }", msg)))
}

// Turns a string into a profanity censored string
func censorProfanity(input string) string {
	badWords := []string{"kerfuffle", "sharbert", "fornax"}

	var cleanedString strings.Builder
	for word := range strings.SplitSeq(input, " ") {
		normWord := strings.ToLower(word)
		if slices.Contains(badWords, normWord) {
			cleanedString.WriteString("****")
		} else {
			cleanedString.WriteString(word)
		}

		cleanedString.WriteString(" ")
	}

	return strings.TrimSpace(cleanedString.String())
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("Error connecting to database")
		return
	}
	const port = "8080"
	var cfg apiConfig
	cfg.dbQueries = database.New(db)
	cfg.platform = os.Getenv("PLATFORM")

	mux := http.NewServeMux()
	mux.Handle("/app/", cfg.middlewareIncrement(http.StripPrefix("/app", http.FileServer(http.Dir("./")))))

	mux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, req *http.Request) {
		type ReqPayload struct {
			Body string `json:"body"`
		}
		// Read the body and decode into a struct
		var reqPayload ReqPayload
		decoder := json.NewDecoder(req.Body)
		err := decoder.Decode(&reqPayload)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("{ \"error\": \"Invalid request\" }"))
			return
		}

		chirp := string(reqPayload.Body)

		// Validate the data
		if len(chirp) > 140 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("{ \"error\": \"Chirp is too long\" }"))
			return
		}

		chirp = censorProfanity(chirp)

		// Otherwise construct succcess message
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("{ \"cleaned_body\": \"%s\" }", chirp)))
	})
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /admin/metrics", cfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)
	mux.HandleFunc("POST /api/users", cfg.createUserHandler)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	server.ListenAndServe()
}
