package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

var testDB *sql.DB

func setupTestDB() {
	var err error
	testDB, err = sql.Open("sqlite3", ":memory:") // Use in-memory DB for tests
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

	_, err = testDB.Exec(createTableQueries)
	if err != nil {
		panic(err)
	}

	db = testDB
}

func TestGenerateTokenHandler(t *testing.T) {
	setupTestDB()

	req, err := http.NewRequest("GET", "/generate_token", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(generateTokenHandler)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Expected status OK")

	// Check if the response contains the auth_token
	responseData := rr.Body.String()
	assert.Contains(t, responseData, "auth_token", "Response should contain an auth_token")

	// Verify that the token was inserted into the database
	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM clients").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, count, "Expected one token in the database")
}

func TestMemesEndpointValidToken(t *testing.T) {
	setupTestDB()

	authToken := "test-valid-token"
	_, err := testDB.Exec("INSERT INTO clients (auth_token, token_balance) VALUES (?, ?)", authToken, 10)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("GET", "/memes?lat=40.730610&lon=-73.935242&query=food", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", authToken)

	rr := httptest.NewRecorder()
	handler := authMiddleware(http.HandlerFunc(handleMemes))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Expected status OK")
	assert.Contains(t, rr.Body.String(), "url", "Response should contain a meme URL")

	// Verify that the token balance was decremented
	var tokenBalance int
	err = testDB.QueryRow("SELECT token_balance FROM clients WHERE auth_token = ?", authToken).Scan(&tokenBalance)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 9, tokenBalance, "Expected token balance to be decremented")
}

func TestMemesEndpointInvalidToken(t *testing.T) {
	setupTestDB()

	req, err := http.NewRequest("GET", "/memes?lat=40.730610&lon=-73.935242&query=food", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "invalid-token")

	rr := httptest.NewRecorder()
	handler := authMiddleware(http.HandlerFunc(handleMemes))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected status Unauthorized")
}

func TestMemesEndpointInsufficientTokens(t *testing.T) {
	setupTestDB()

	authToken := "test-insufficient-token"
	_, err := testDB.Exec("INSERT INTO clients (auth_token, token_balance) VALUES (?, ?)", authToken, 0)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("GET", "/memes?lat=40.730610&lon=-73.935242&query=food", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", authToken)

	rr := httptest.NewRecorder()
	handler := authMiddleware(http.HandlerFunc(handleMemes))

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusPaymentRequired, rr.Code, "Expected status Payment Required")
}
