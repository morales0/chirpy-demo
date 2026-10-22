package main

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
)

/*
 * Helper function to respond with an error message in JSON format
 */
func respondWithError(w http.ResponseWriter, message string, err error, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	errMsg := fmt.Sprintf("{ \"error\": \"%v\", \"message\": \"%v\" }", err, message)
	w.Write([]byte(errMsg))
}

/*
 * Turns a string into a profanity censored string
 */
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
