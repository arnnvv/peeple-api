package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	testServer  *httptest.Server
	testDBPool  *pgxpool.Pool
	testQueries *migrations.Queries
	testApp     *http.ServeMux
	testDbURL   string
)

const seedFilePath = "seed.sql"

func testMainSetup() error {
	log.Println("Setting up E2E tests...")

	testDbURL = os.Getenv("TEST_DATABASE_URL")
	if testDbURL == "" {
		return fmt.Errorf("TEST_DATABASE_URL environment variable not set. Please set it before running tests")
	}
	log.Printf("Using Test Database URL: %s", strings.Split(testDbURL, "@")[1])

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("CRITICAL WARNING: JWT_SECRET environment variable not set for tests. Token validation will fail.")
		return fmt.Errorf("JWT_SECRET environment variable not set. Please set it before running tests")
	}

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID_ANDROID")
	if googleClientID == "" {
		log.Println("WARNING: GOOGLE_CLIENT_ID_ANDROID environment variable not set. Google Auth tests may fail.")
	}

	var err error
	for range 5 {
		testDBPool, err = pgxpool.New(context.Background(), testDbURL)
		if err == nil {
			err = testDBPool.Ping(context.Background())
			if err == nil {
				log.Println("Successfully connected to test database.")
				break
			}
		}
		log.Printf("Waiting for test database... (%v)", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to test database after retries: %w", err)
	}

	testQueries = migrations.New(testDBPool)

	log.Println("Cleaning test database (dropping/recreating public schema)...")
	_, err = testDBPool.Exec(context.Background(), `
		DROP SCHEMA public CASCADE;
		CREATE SCHEMA public;
		GRANT ALL ON SCHEMA public TO public;
	`)
	if err != nil {
		return fmt.Errorf("failed to drop/recreate public schema: %w", err)
	}

	log.Println("Applying database schema...")
	schemaBytes, err := os.ReadFile("db/schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema.sql: %w", err)
	}
	_, err = testDBPool.Exec(context.Background(), string(schemaBytes))
	if err != nil {
		log.Printf("Schema applying error near: %s...", string(schemaBytes[:min(500, len(schemaBytes))]))
		return fmt.Errorf("failed to apply database schema: %w", err)
	}
	log.Println("Database schema applied.")

	log.Println("Seeding test database...")
	cmd := exec.Command("psql", testDbURL, "-f", seedFilePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Printf("psql seeding failed: %v. Trying direct execution.", err)
		seedBytes, readErr := os.ReadFile(seedFilePath)
		if readErr != nil {
			return fmt.Errorf("failed to read seed file '%s' for fallback: %w", seedFilePath, readErr)
		}
		_, execErr := testDBPool.Exec(context.Background(), string(seedBytes))
		if execErr != nil {
			log.Printf("Direct execution also failed: %v", execErr)
			return fmt.Errorf("failed to seed database using psql or direct exec: %w. Check seed.sql format (no COPY FROM STDIN for direct exec)", err)
		}
	}
	log.Println("Test database seeded.")

	if err := db.InitDB(testDbURL); err != nil {
		return fmt.Errorf("failed to initialize main app DB connection to test DB: %w", err)
	}

	log.Println("Setting up application router...")
	testApp = setupRoutes()

	log.Println("Starting test HTTP server...")
	testServer = httptest.NewServer(testApp)
	log.Printf("Test server running at: %s", testServer.URL)

	return nil
}

func testMainTeardown() {
	log.Println("Tearing down E2E tests...")
	if testServer != nil {
		log.Println("Closing test HTTP server.")
		testServer.Close()
	}
	if testDBPool != nil {
		log.Println("Closing test database connection pool.")
		testDBPool.Close()
	}
	db.CloseDB()
	log.Println("Teardown complete.")
}

func TestMain(m *testing.M) {
	if err := testMainSetup(); err != nil {
		log.Fatalf("FATAL: Test setup failed: %v", err)
	}

	exitCode := m.Run()
	testMainTeardown()
	os.Exit(exitCode)
}

func getTestAuthToken(t *testing.T, email string) string {
	t.Helper()

	if os.Getenv("JWT_SECRET") == "" {
		t.Fatal("JWT_SECRET environment variable not set for test token generation")
	}

	reqURL := fmt.Sprintf("%s/token?email=%s", testServer.URL, email)
	resp, err := http.Get(reqURL)
	if err != nil {
		t.Fatalf("Failed to make request to get token for %s: %v", email, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK for token request, got %d", resp.StatusCode)
	}

	var tokenResp token.TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		t.Fatalf("Failed to decode token response: %v", err)
	}

	if !tokenResp.Success || tokenResp.Token == "" {
		t.Fatalf("Token request was not successful or token was empty for %s", email)
	}

	return tokenResp.Token
}

func getTestUserByEmail(t *testing.T, email string) *migrations.User {
	t.Helper()
	user, err := testQueries.GetUserByEmail(context.Background(), email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("Failed to find seed user with email %s in test DB: %v", email, err)
		}
		t.Fatalf("Error fetching user %s from test DB: %v", email, err)
	}
	return &user
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
