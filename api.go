package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/morales0/chirpy-demo/internal/database"
)

// This file defined the api config and associated methods

type apiConfig struct {
	fileServerHits atomic.Int32
	db             *database.Queries
	platform       string
}

/*
 * Middleware to count file server hits
 */
func (cfg *apiConfig) middlewareIncrement(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, req)
	})
}

/*
 * Handler that displays visited metrics
 */
func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	page := fmt.Sprintf(`
	<html>
	  <body>
	    <h1>Welcome, Chirpy Admin</h1>
	    <p>Chirpy has been visited %d times!</p>
	  </body>
	</html>`,
		cfg.fileServerHits.Load())

	w.Write([]byte(page))
}

/*
 * Handler that resets the system. Only accessible in dev environments.
 */
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, req *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Restore server hit count
	cfg.fileServerHits.Store(0)
	// Delete all users
	cfg.db.DeleteAllUsers(req.Context())

	w.WriteHeader(http.StatusOK)
}

/*
 * Handler for createing a user.
 */
func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, req *http.Request) {
	// Load the email from the body of the request
	type ReqPayload struct {
		Email string `json:"email"`
	}
	var reqPayload ReqPayload
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&reqPayload)
	if err != nil {
		respondWithError(w, "Unable to decode payload", err, http.StatusBadRequest)
		return
	}
	email := reqPayload.Email

	// Call the CreateUser function
	newUser, err := cfg.db.CreateUser(req.Context(), email)
	if err != nil {
		respondWithError(w, "Error trying to create the user", err, http.StatusInternalServerError)
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
		respondWithError(w, "Error creating response payload", err, http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	w.Write(data)
}

/*
 * Handler for creating a chirp
 */
func (cfg *apiConfig) chirpHandler(w http.ResponseWriter, req *http.Request) {
	// Read body and user_id from request
	type ReqPayload struct {
		Body   string `json:"body"`
		UserId string `json:"user_id"`
	}
	var reqPayload ReqPayload
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&reqPayload)
	if err != nil {
		respondWithError(w, "Unable to decode payload", err, http.StatusBadRequest)
		return
	}

	// Validate the chirp
	chirp := string(reqPayload.Body)
	if len(chirp) > 140 {
		respondWithError(w, "Chirp is too long", err, http.StatusBadRequest)
		return
	}
	chirp = censorProfanity(chirp)
	// If valid, save the chirp in the database
	userId, err := uuid.Parse(reqPayload.UserId)
	if err != nil {
		respondWithError(w, "Invalid user id", err, http.StatusBadRequest)
		return
	}
	params := database.CreateChirpParams{
		UserID: userId,
		Body:   chirp,
	}
	savedChirp, err := cfg.db.CreateChirp(req.Context(), params)
	if err != nil {
		respondWithError(w, "Error trying to save chirp", err, http.StatusInternalServerError)
		return
	}
	// Create response data
	type RespPayload struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt string    `json:"created_at"`
		UpdatedAt string    `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	response := RespPayload{
		ID:        savedChirp.ID,
		CreatedAt: savedChirp.CreatedAt.String(),
		UpdatedAt: savedChirp.UpdatedAt.String(),
		Body:      savedChirp.Body,
		UserID:    savedChirp.UserID,
	}
	data, err := json.Marshal(response)
	if err != nil {
		respondWithError(w, "Error creating response", err, http.StatusInternalServerError)
		return
	}
	// Return
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	w.Write(data)
}

func (cfg *apiConfig) getChirpsHandler(w http.ResponseWriter, req *http.Request) {
	// Fetch them from the database
	chirps, err := cfg.db.FetchAllChirpsOrderedByCreatedAsc(req.Context())
	if err != nil {
		respondWithError(w, "Error fetching chirps", err, http.StatusInternalServerError)
	}

	type RespPayload struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt string    `json:"created_at"`
		UpdatedAt string    `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	var jsonChirps []RespPayload
	for _, chirp := range chirps {
		jsonChirp := RespPayload{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt.String(),
			UpdatedAt: chirp.UpdatedAt.String(),
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		}

		jsonChirps = append(jsonChirps, jsonChirp)
	}
	data, err := json.Marshal(jsonChirps)

	// Return the array
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (cfg *apiConfig) getChirpHandler(w http.ResponseWriter, req *http.Request) {
	// Get the id from the path
	pathId := req.PathValue("chirpId")
	id, err := uuid.Parse(pathId)
	if err != nil {
		respondWithError(w, "Invalid chirp id", err, http.StatusBadRequest)
		return
	}

	// Fetch the chirp from the database
	chirp, err := cfg.db.GetChirp(req.Context(), id)
	if err != nil {
		respondWithError(w, "Failed to fetch chirp", err, http.StatusNotFound)
		return
	}

	// Return the data
	type RespPayload struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt string    `json:"created_at"`
		UpdatedAt string    `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	chirpData := RespPayload{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt.String(),
		UpdatedAt: chirp.UpdatedAt.String(),
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}
	data, err := json.Marshal(chirpData)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
