package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var db *sql.DB
var mu sync.Mutex

// MemeResponse represents the response structure for the meme API
type MemeResponse struct {
	URL      string `json:"url"`
	Location string `json:"location"`
	Query    string `json:"query"`
}

func main() {
	initDB()

	http.Handle("/memes", authMiddleware(http.HandlerFunc(handleMemes)))
	http.HandleFunc("/update_tokens", updateTokens)
	http.HandleFunc("/generate_token", generateTokenHandler)

	fmt.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
	}
}

func handleMemes(w http.ResponseWriter, r *http.Request) {
	lat, lon, query := r.URL.Query().Get("lat"), r.URL.Query().Get("lon"), r.URL.Query().Get("query")

	locationName, err := reverseGeocode(lat, lon)
	if err != nil {
		http.Error(w, "Failed to get location name", http.StatusInternalServerError)
		return
	}

	combinedQuery := fmt.Sprintf("%s %s", query, locationName)

	memeURL, err := fetchMeme(combinedQuery)
	if err != nil {
		http.Error(w, "Failed to fetch meme", http.StatusInternalServerError)
		return
	}

	response := MemeResponse{
		URL:      memeURL,
		Location: locationName,
		Query:    query,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func reverseGeocode(lat, lon string) (string, error) {
	apiKey := "95a8208a6c3a47188b9dd50d05c86b2e"
	reqURL := fmt.Sprintf("https://api.opencagedata.com/geocode/v1/json?q=%s,%s&key=%s", lat, lon, apiKey)

	resp, err := http.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenCage API request failed with status %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if results, ok := result["results"].([]interface{}); ok && len(results) > 0 {
		if components, ok := results[0].(map[string]interface{})["components"].(map[string]interface{}); ok {
			if city, ok := components["city"].(string); ok {
				return city, nil
			} else if town, ok := components["town"].(string); ok {
				return town, nil
			} else if state, ok := components["state"].(string); ok {
				return state, nil
			}
		}
	}

	return "", fmt.Errorf("no valid location found for the provided coordinates")
}

func fetchMeme(query string) (string, error) {
	apiKey := "yV6ML6lgRWF7mO7RZiIUHjFgTBGPr1dI"
	reqURL := fmt.Sprintf("https://api.giphy.com/v1/gifs/search?api_key=%s&q=%s&limit=10", apiKey, url.QueryEscape(query))

	resp, err := http.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Giphy API request failed with status %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	data, ok := result["data"].([]interface{})
	if !ok || len(data) == 0 {
		return "", fmt.Errorf("no memes found for query: %s", query)
	}

	rand.Seed(time.Now().UnixNano())
	memeURL := data[rand.Intn(len(data))].(map[string]interface{})["images"].(map[string]interface{})["original"].(map[string]interface{})["url"].(string)

	return memeURL, nil
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./client_tokens.db")
	if err != nil {
		panic(err)
	}

	createTableQueries := `
	CREATE TABLE IF NOT EXISTS clients (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		auth_token TEXT UNIQUE NOT NULL,
		token_balance INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS api_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		client_id INTEGER,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(client_id) REFERENCES clients(id)
	);
	`

	_, err = db.Exec(createTableQueries)
	if err != nil {
		panic(err)
	}

	fmt.Println("Database initialized")
}

func generateTokenHandler(w http.ResponseWriter, r *http.Request) {
	authToken := uuid.New().String()

	initialBalance := 100
	if err := insertAuthToken(authToken, initialBalance); err != nil {
		http.Error(w, "Failed to generate auth token", http.StatusInternalServerError)
		return
	}

	response := map[string]string{"auth_token": authToken}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func insertAuthToken(authToken string, initialBalance int) error {
	mu.Lock()
	defer mu.Unlock()

	_, err := db.Exec("INSERT INTO clients (auth_token, token_balance) VALUES (?, ?)", authToken, initialBalance)
	return err
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authToken := r.Header.Get("Authorization")
		if authToken == "" {
			http.Error(w, "Missing auth token", http.StatusUnauthorized)
			return
		}

		clientID, tokenBalance, err := getClientTokenBalance(authToken)
		if err != nil {
			http.Error(w, "Invalid auth token", http.StatusUnauthorized)
			return
		}

		if tokenBalance <= 0 {
			http.Error(w, "Insufficient tokens", http.StatusPaymentRequired)
			return
		}

		if err := logApiCallAndDecrementToken(clientID); err != nil {
			http.Error(w, "Failed to log API call", http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getClientTokenBalance(authToken string) (int, int, error) {
	var clientID, tokenBalance int
	err := db.QueryRow("SELECT id, token_balance FROM clients WHERE auth_token=$1", authToken).Scan(&clientID, &tokenBalance)
	return clientID, tokenBalance, err
}

func logApiCallAndDecrementToken(clientID int) error {
	mu.Lock()
	defer mu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("INSERT INTO api_calls (client_id) VALUES ($1)", clientID)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("UPDATE clients SET token_balance = token_balance - 1 WHERE id = $1", clientID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func updateTokens(w http.ResponseWriter, r *http.Request) {
	authToken := r.URL.Query().Get("auth_token")
	additionalTokens := r.URL.Query().Get("tokens")
	_, err := db.Exec("UPDATE clients SET token_balance = token_balance + $1 WHERE auth_token = $2", additionalTokens, authToken)
	if err != nil {
		http.Error(w, "Failed to update tokens", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Tokens updated successfully")
}
