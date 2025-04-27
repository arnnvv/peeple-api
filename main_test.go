package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/handlers"

	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/ws"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testServerURL      string
	testWsURL          string
	testDBPool         *pgxpool.Pool
	testJWTSecret      string
	testUser13Token    string // Ayush
	testUser12Token    string // Shruti
	testUserAdminToken string // Assume user 18 (Mansi) is made admin for tests
	testUser17Token string // Kushal
	testUser18Token string // Mansi (will be made admin)
)

const (
	testPort          = "8089" // Use a different port for tests
	testUserID13      = 13     // Ayush
	testUserID12      = 12     // Shruti
	testUserID17      = 17     // Kushal
	testUserID18      = 18     // Mansi (Admin)
	testWaitServer    = 3 * time.Second
	wsReadWait        = 10 * time.Second // Timeout for reading ws message
	wsWriteWait       = 10 * time.Second
	defaultTestTimeout = 15 * time.Second
)

func TestMain(m *testing.M) {
	var err error
	testDbURL := os.Getenv("TEST_DATABASE_URL")
	testJWTSecret = os.Getenv("JWT_SECRET")

	if testDbURL == "" {
		log.Fatal("FATAL: TEST_DATABASE_URL environment variable not set.")
	}
	if testJWTSecret == "" {
		log.Fatal("FATAL: JWT_SECRET environment variable not set.")
	}
	os.Setenv("PORT", testPort)

	err = setupTestDatabase(testDbURL)
	if err != nil {
		log.Fatalf("FATAL: Failed to setup test database: %v", err)
	}

	err = db.InitDB(testDbURL)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize db package with test DB: %v", err)
	}

	testUser13Token, err = token.GenerateToken(testUserID13)
	require.NoError(&testing.T{}, err, "Failed to generate token for user 13")
	testUser12Token, err = token.GenerateToken(testUserID12)
	require.NoError(&testing.T{}, err, "Failed to generate token for user 12")
	testUser17Token, err = token.GenerateToken(testUserID17)
	require.NoError(&testing.T{}, err, "Failed to generate token for user 17")
	testUser18Token, err = token.GenerateToken(testUserID18)
	require.NoError(&testing.T{}, err, "Failed to generate token for user 18")

	err = makeUserAdminForTest(testUserID18)
	require.NoError(&testing.T{}, err, "Failed to make user 18 admin")
	testUserAdminToken, err = token.GenerateToken(testUserID18)
	require.NoError(&testing.T{}, err, "Failed to generate admin token for user 18")

	serverCtx, serverCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		log.Printf("Starting test server on port %s...", testPort)
		queries, dbErr := db.GetDB()
		if dbErr != nil || queries == nil {
			log.Fatalf("FATAL: Could not get DB queries for test server Hub: %v", dbErr)
		}
		hub := ws.NewHub(queries)
		go hub.Run()

		mux := setupRoutes(hub)
		server := &http.Server{
			Addr:    ":" + testPort,
			Handler: mux,
		}

		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Test server ListenAndServe error: %v", err)
			}
			log.Println("Test server stopped.")
		}()

		<-serverCtx.Done()
		log.Println("Shutting down test server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Test server graceful shutdown error: %v", err)
		} else {
			log.Println("Test server shutdown complete.")
		}
	}()

	testServerURL = "http://localhost:" + testPort
	testWsURL = "ws://localhost:" + testPort + "/chat"
	waitForServer(testServerURL + "/test")

	log.Println("Running tests...")
	exitCode := m.Run()

	log.Println("Signaling test server to shut down...")
	serverCancel()
	log.Println("Waiting for server goroutine to finish...")
	wg.Wait()
	log.Println("Server goroutine finished.")

	log.Println("Cleaning up test database...")
	teardownTestDatabase()
	log.Println("Closing db package connection...")
	db.CloseDB()
	log.Println("Test teardown complete.")

	os.Exit(exitCode)
}


func setupTestDatabase(dbURL string) error {
	log.Printf("Connecting to TEST database: %s\n", strings.Split(dbURL, "@")[1])
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return fmt.Errorf("failed to parse test db config: %w", err)
	}
	config.MaxConns = 10

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testDBPool, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create test db pool: %w", err)
	}

	err = testDBPool.Ping(ctx)
	if err != nil {
		testDBPool.Close()
		return fmt.Errorf("failed to ping test db: %w", err)
	}
	log.Println("Test database connected.")

	err = cleanTestDatabase(ctx)
	if err != nil {
		testDBPool.Close()
		return fmt.Errorf("failed to clean test database: %w", err)
	}

	log.Println("Applying database schema...")
	schemaSQL, err := os.ReadFile("db/schema.sql")
	if err != nil {
		testDBPool.Close()
		return fmt.Errorf("failed to read schema.sql: %w", err)
	}
	_, err = testDBPool.Exec(ctx, string(schemaSQL))
	if err != nil {
		testDBPool.Close()
		return fmt.Errorf("failed to execute schema.sql: %w", err)
	}
	log.Println("Database schema applied.")

	log.Println("Applying seed data...")
	seedSQL, err := os.ReadFile("seed.sql")
	if err != nil {
		testDBPool.Close()
		return fmt.Errorf("failed to read seed.sql: %w", err)
	}
	_, err = testDBPool.Exec(ctx, string(seedSQL))
	if err != nil {
		testDBPool.Close()
		return fmt.Errorf("failed to execute seed.sql: %w", err)
	}
	log.Println("Seed data applied.")

	return nil
}

func cleanTestDatabase(ctx context.Context) error {
	log.Println("Cleaning test database (dropping public schema)...")
	_, err := testDBPool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	if err != nil {
		log.Printf("WARN: Failed to drop public schema (%v), attempting TRUNCATE...", err)
		tables := []string{
			"message_reactions", "reports", "chat_messages", "user_consumables",
			"user_subscriptions", "likes", "dislikes", "filters",
			"date_vibes_prompts", "getting_personal_prompts", "my_type_prompts",
			"story_time_prompts", "users",
		}
		truncateCmd := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE;", strings.Join(tables, ", "))
		_, err = testDBPool.Exec(ctx, truncateCmd)
		if err != nil {
			return fmt.Errorf("failed to truncate tables: %w", err)
		}
		log.Println("Truncated tables instead of dropping schema.")
	} else {
		log.Println("Public schema dropped and recreated.")
	}
	return nil
}

func teardownTestDatabase() {
	if testDBPool != nil {
		log.Println("Closing test database pool...")
		testDBPool.Close()
		log.Println("Test database pool closed.")
	}
}

func makeUserAdminForTest(userID int32) error {
	log.Printf("Attempting to set user %d as admin for tests...", userID)
	queries, err := db.GetDB()
	if err != nil {
		return fmt.Errorf("failed to get DB for making user admin: %w", err)
	}
	_, err = queries.UpdateUserRole(context.Background(), migrations.UpdateUserRoleParams{
		ID:   userID,
		Role: migrations.UserRoleAdmin,
	})
	if err != nil {
		return fmt.Errorf("failed to update role for user %d: %w", userID, err)
	}
	log.Printf("Successfully set user %d as admin.", userID)
	return nil
}

func waitForServer(url string) {
	log.Printf("Waiting for server at %s...", url)
	for i := range 10 {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Println("Server is ready.")
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		log.Printf("Server not ready yet (attempt %d)...", i+1)
		time.Sleep(testWaitServer / 2)
	}
	log.Fatal("FATAL: Server did not become ready in time.")
}

func makeRequest(t *testing.T, method, path string, authToken *string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, testServerURL+path, body)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authToken != nil {
		req.Header.Set("Authorization", "Bearer "+*authToken)
	}
	return req
}

func executeRequest(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	client := &http.Client{Timeout: defaultTestTimeout}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeResponse(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer resp.Body.Close()
	err := json.NewDecoder(resp.Body).Decode(target)
	if err != nil {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Logf("Failed to decode response body: %v. Body was: %s", err, string(bodyBytes))
	}
	require.NoError(t, err, "Failed to decode JSON response")
}

func assertSuccessResponse(t *testing.T, resp *http.Response, expectedCode int) map[string]any {
	t.Helper()
	assert.Equal(t, expectedCode, resp.StatusCode, "Expected status code %d", expectedCode)

	var result map[string]any
	decodeResponse(t, resp, &result)

	success, ok := result["success"].(bool)
	assert.True(t, ok, "Response should have a boolean 'success' field")
	assert.True(t, success, "Expected 'success' field to be true. Message: %v", result["message"])
	return result
}

func assertErrorResponse(t *testing.T, resp *http.Response, expectedCode int, expectedMessageSubstring ...string) {
	t.Helper()
	assert.Equal(t, expectedCode, resp.StatusCode, "Expected error status code %d", expectedCode)

	var result map[string]any
	err := json.NewDecoder(resp.Body).Decode(&result)
	defer resp.Body.Close()

	if err != nil {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Logf("Failed to decode error response body: %v. Status: %d. Body was: %s", err, resp.StatusCode, string(bodyBytes))
		require.NoError(t, err, "Failed to decode JSON error response")
		return
	}


	success, ok := result["success"].(bool)
	assert.True(t, ok, "Error response should have a boolean 'success' field")
	assert.False(t, success, "Expected 'success' field to be false in error response")

	message, ok := result["message"].(string)
	assert.True(t, ok, "Error response should have a string 'message' field")
	if len(expectedMessageSubstring) > 0 {
		assert.Contains(t, message, expectedMessageSubstring[0], "Error message mismatch")
	} else {
		assert.NotEmpty(t, message, "Error message should not be empty")
	}
}

func connectWebSocket(t *testing.T, token string) (*websocket.Conn, *http.Response) {
	t.Helper()
	header := http.Header{}
	header.Add("Authorization", "Bearer "+token)

	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(testWsURL, header)
	require.NoError(t, err, "WebSocket dial error")
	require.NotNil(t, conn, "WebSocket connection should not be nil")
	// Don't assert status code here, dialer handles it implicitly
	// require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode, "WebSocket upgrade failed")

	_, _, err = readWsMessageWithTimeout(t, conn, wsReadWait)
	require.NoError(t, err, "Failed to read initial WS info message")

	return conn, resp
}

func readWsMessageWithTimeout(t *testing.T, conn *websocket.Conn, timeout time.Duration) (int, []byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		messageType int
		p           []byte
		err         error
	}
	resultChan := make(chan result, 1)

	go func() {
		mt, p, err := conn.ReadMessage()
		resultChan <- result{mt, p, err}
	}()

	select {
	case <-ctx.Done():
		return 0, nil, fmt.Errorf("websocket read timed out after %v", timeout)
	case res := <-resultChan:
		return res.messageType, res.p, res.err
	}
}

func writeWsMessage(t *testing.T, conn *websocket.Conn, message any) error {
	t.Helper()
	bytes, err := json.Marshal(message)
	require.NoError(t, err, "Failed to marshal WS message")
	_, cancel := context.WithTimeout(context.Background(), wsWriteWait)
	defer cancel()
	return conn.WriteMessage(websocket.TextMessage, bytes)
}

func assertWsMessageType(t *testing.T, conn *websocket.Conn, expectedType string, timeout time.Duration) ws.WsMessage {
	t.Helper()
	msgType, p, err := readWsMessageWithTimeout(t, conn, timeout)
	require.NoError(t, err, "Failed to read WebSocket message")
	require.Equal(t, websocket.TextMessage, msgType, "Expected text message type")

	var receivedMsg ws.WsMessage
	err = json.Unmarshal(p, &receivedMsg)
	require.NoError(t, err, "Failed to unmarshal WebSocket message JSON")

	require.Equal(t, expectedType, receivedMsg.Type, "Unexpected WebSocket message type received")
	return receivedMsg
}


func TestAuthEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("GetTokenDebug_Success", func(t *testing.T) {
		req := makeRequest(t, "GET", "/token?email=ayush_g@ar.iitr.ac.in", nil, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		assert.NotEmpty(t, body["token"], "Token should not be empty")
		assert.Contains(t, body["message"], "Token generated successfully", "Incorrect success message")
	})

	t.Run("GetTokenDebug_NotFound", func(t *testing.T) {
		req := makeRequest(t, "GET", "/token?email=nonexistent@example.com", nil, nil)
		resp := executeRequest(t, req)
		assertErrorResponse(t, resp, http.StatusNotFound, "User with the provided email not found")
	})

	t.Run("GetTokenDebug_MissingEmail", func(t *testing.T) {
		req := makeRequest(t, "GET", "/token", nil, nil)
		resp := executeRequest(t, req)
		assertErrorResponse(t, resp, http.StatusBadRequest, "Email address query parameter is required")
	})

	t.Run("CheckAuthStatus_ValidToken_Home", func(t *testing.T) {
		req := makeRequest(t, "GET", "/api/auth-status", &testUser13Token, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		assert.Equal(t, "home", body["status"], "Expected status 'home'")
	})

	t.Run("CheckAuthStatus_InvalidToken", func(t *testing.T) {
		invalidToken := "this.is.invalid"
		req := makeRequest(t, "GET", "/api/auth-status", &invalidToken, nil)
		resp := executeRequest(t, req)
		assertErrorResponse(t, resp, http.StatusUnauthorized, "Invalid token")
	})

	t.Run("CheckAuthStatus_NoToken", func(t *testing.T) {
		req := makeRequest(t, "GET", "/api/auth-status", nil, nil)
		resp := executeRequest(t, req)
		assertErrorResponse(t, resp, http.StatusUnauthorized, "Invalid Authorization header format")
	})

	t.Run("GoogleAuth_Placeholder", func(t *testing.T) {
		t.Skip("Skipping Google Auth integration test - requires mocking or real tokens")
	})
}

func TestProfileEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("GetProfile_User13_Success", func(t *testing.T) {
		req := makeRequest(t, "GET", "/get-profile", &testUser13Token, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)

		userMap, ok := body["user"].(map[string]any)
		require.True(t, ok, "Response 'user' field should be an object")

		assert.Equal(t, float64(testUserID13), userMap["id"], "User ID mismatch")
		assert.Equal(t, "ayush_g@ar.iitr.ac.in", userMap["email"], "Email mismatch")

		nameMap, _ := userMap["name"].(map[string]any)
		assert.True(t, nameMap["Valid"].(bool), "Name should be valid")
		assert.Equal(t, "Ayush", nameMap["String"], "Name mismatch")

		genderMap, _ := userMap["gender"].(map[string]any)
		assert.True(t, genderMap["Valid"].(bool), "Gender should be valid")
		assert.Equal(t, string(migrations.GenderEnumMan), genderMap["GenderEnum"], "Gender mismatch")

		heightMap, _ := userMap["height"].(map[string]any)
		assert.True(t, heightMap["Valid"].(bool), "Height should be valid")
		assert.Equal(t, "5' 1\"", heightMap["String"], "Height mismatch (Formatted)")

		prompts, ok := userMap["prompts"].([]any)
		assert.True(t, ok, "Prompts should be an array")
		assert.GreaterOrEqual(t, len(prompts), 1, "User 13 should have prompts")
	})

	t.Run("EditProfile_User13_UpdateHometown", func(t *testing.T) {
		newHometown := "Testville"
		payload := map[string]any{
			"hometown": newHometown,
		}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "PATCH", "/api/profile/edit", &testUser13Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		assertSuccessResponse(t, resp, http.StatusOK)

		reqGet := makeRequest(t, "GET", "/get-profile", &testUser13Token, nil)
		respGet := executeRequest(t, reqGet)
		bodyGet := assertSuccessResponse(t, respGet, http.StatusOK)
		userMap, _ := bodyGet["user"].(map[string]any)
		hometownMap, _ := userMap["hometown"].(map[string]any)
		assert.True(t, hometownMap["Valid"].(bool))
		assert.Equal(t, newHometown, hometownMap["String"])

	})

}

func TestFeedEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("GetFilters_User13_Exists", func(t *testing.T) {
		req := makeRequest(t, "GET", "/api/get-filters", &testUser13Token, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		assert.NotNil(t, body["filters"], "Filters should exist for user 13 based on seed")
		filtersMap, _ := body["filters"].(map[string]any)
		whoMap, _ := filtersMap["WhoYouWantToSee"].(map[string]any)
		assert.Equal(t, string(migrations.GenderEnumWoman), whoMap["GenderEnum"])
	})

	t.Run("ApplyFilters_User13_UpdateRadius", func(t *testing.T) {
		payload := map[string]any{
			"whoYouWantToSee": "woman",
			"radius":          25,
			"activeToday":     false,
			"ageMin":          18,
			"ageMax":          55,
		}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "POST", "/api/filters", &testUser13Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		assert.NotNil(t, body["filters"])
		filtersMap, _ := body["filters"].(map[string]any)
		radiusMap, _ := filtersMap["RadiusKm"].(map[string]any)
		assert.Equal(t, float64(25), radiusMap["Int32"])
	})

	t.Run("GetHomeFeed_User13", func(t *testing.T) {
		req := makeRequest(t, "GET", "/api/homefeed", &testUser13Token, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)

		profiles, ok := body["profiles"].([]any)
		require.True(t, ok, "'profiles' should be an array")
		foundUser18 := false
		for _, p := range profiles {
			profileMap, _ := p.(map[string]any)
			profileID := int32(profileMap["ID"].(float64))
			if profileID == testUserID18 {
				foundUser18 = true
			}
			if profileID == testUserID12 {
				t.Errorf("Home feed for User 13 should NOT contain matched User 12")
			}
			assert.NotEqual(t, testUserID13, profileID, "Feed should not contain self")
			assert.NotEqual(t, testUserID17, profileID, "Feed should not contain User 17 (wrong gender)")

			dist, okDist := profileMap["distance_km"].(float64)
			assert.True(t, okDist, "Profile should have distance_km")
			assert.GreaterOrEqual(t, dist, 0.0, "Distance should be non-negative")

			_, okPrompts := profileMap["prompts"].([]any)
			assert.True(t, okPrompts || profileMap["prompts"] == nil || profileMap["prompts"] == "[]", "Profile should have prompts array or null/empty")


		}
		assert.True(t, foundUser18, "User 18 (potential match) should be in User 13's feed")
	})

	t.Run("GetQuickFeed_User13", func(t *testing.T) {
		req := makeRequest(t, "GET", "/api/quickfeed", &testUser13Token, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)

		profiles, ok := body["profiles"].([]any)
		require.True(t, ok, "'profiles' should be an array")
		foundUser12 := false
		foundUser18 := false
		assert.LessOrEqual(t, len(profiles), 2, "Quick feed should return max 2 profiles")
		for _, p := range profiles {
			profileMap, _ := p.(map[string]any)
			profileID := int32(profileMap["id"].(float64))
			if profileID == testUserID12 {
				foundUser12 = true
			}
			if profileID == testUserID18 {
				foundUser18 = true
			}
			assert.NotEqual(t, testUserID13, profileID, "QuickFeed should not contain self")
			assert.NotEqual(t, testUserID17, profileID, "QuickFeed should not contain User 17 (wrong gender)")

			dist, okDist := profileMap["distance_km"].(float64)
			assert.True(t, okDist, "Profile should have distance_km")
			assert.GreaterOrEqual(t, dist, 0.0, "Distance should be non-negative")
		}
		assert.True(t, foundUser12 || foundUser18, "At least one of User 12 or 18 should be in QuickFeed")
	})

}

func TestLikesMatchesEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("GetMatches_User13_Initial", func(t *testing.T) {
		req := makeRequest(t, "GET", "/api/matches", &testUser13Token, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		matches, ok := body["matches"].([]any)
		require.True(t, ok)
		foundUser12 := false
		require.GreaterOrEqual(t, len(matches), 1, "User 13 should have at least 1 match (User 12)")
		for _, m := range matches {
			matchMap, _ := m.(map[string]any)
			matchedID := int32(matchMap["matched_user_id"].(float64))
			if matchedID == testUserID12 {
				foundUser12 = true
				assert.Equal(t, float64(testUserID13), matchMap["last_event_user_id"], "Last event sender mismatch")
				assert.Equal(t, "media", matchMap["last_event_type"], "Last event type mismatch")
				assert.Equal(t, "image/jpeg", matchMap["last_event_content"], "Last event content mismatch")
				assert.NotEmpty(t, matchMap["last_event_media_url"], "Last event media URL should exist")
			}
		}
		assert.True(t, foundUser12, "User 12 should be in User 13's matches")
	})

	t.Run("LikeContent_User13_Likes_User18_Photo0", func(t *testing.T) {
		payload := handlers.ContentLikeRequest{
			LikedUserID:       testUserID18,
			ContentType:       string(migrations.ContentLikeTypeMedia),
			ContentIdentifier: "0",
			Comment:           ws.Ptr("Nice profile pic!"),
		}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "POST", "/api/like", &testUser13Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		assertSuccessResponse(t, resp, http.StatusOK)
	})

	t.Run("GetWhoLikedYou_User18", func(t *testing.T) {
		req := makeRequest(t, "GET", "/api/likes/received", &testUser18Token, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)

		fullProfiles, okFull := body["full_profiles"].([]any)
		require.True(t, okFull)

		foundUser13 := false
		for _, fp := range fullProfiles {
			profileMap, _ := fp.(map[string]any)
			likerID := int32(profileMap["liker_user_id"].(float64))
			if likerID == testUserID13 {
				foundUser13 = true
				assert.Equal(t, "Nice profile pic!", *ws.Ptr(profileMap["like_comment"].(string)))
				assert.False(t, profileMap["is_rose"].(bool))
			}
		}
		assert.True(t, foundUser13, "User 13 should be in User 18's received likes")
	})

	t.Run("MarkLikesSeen_User18", func(t *testing.T) {
		queries, _ := db.GetDB()
		likes, err := queries.GetLikersForUser(context.Background(), testUserID18)
		require.NoError(t, err)
		var likeID int32 = 0
		for _, l := range likes {
			if l.LikerUserID == testUserID13 {
				likeID = l.LikeID
				break
			}
		}
		require.NotEqual(t, 0, likeID, "Could not find like from user 13 to 18")

		payload := handlers.MarkLikesSeenUntilRequest{LikeID: likeID}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "POST", "/api/likes/seen-until", &testUser18Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		assert.Equal(t, float64(1), body["likes_marked_as_seen"])
	})

	t.Run("GetLikerProfile_User18_Gets_User13", func(t *testing.T) {
		req := makeRequest(t, "GET", fmt.Sprintf("/api/liker-profile/%d", testUserID13), &testUser18Token, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		assert.NotNil(t, body["profile"], "Profile data should be present")
		assert.NotNil(t, body["like_details"], "Like details should be present")
		likeDetailsMap, _ := body["like_details"].(map[string]any)
		assert.Equal(t, "Nice profile pic!", likeDetailsMap["like_comment"])
		assert.False(t, likeDetailsMap["is_rose"].(bool))
	})

	t.Run("LikeContent_User18_LikesBack_User13_Profile", func(t *testing.T) {
		payload := handlers.ContentLikeRequest{
			LikedUserID:       testUserID13,
			ContentType:       string(migrations.ContentLikeTypeProfile),
			ContentIdentifier: "profile",
		}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "POST", "/api/like", &testUser18Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		assertSuccessResponse(t, resp, http.StatusOK)
	})

	t.Run("GetMatches_User13_AfterMatchWith18", func(t *testing.T) {
		req := makeRequest(t, "GET", "/api/matches", &testUser13Token, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		matches, ok := body["matches"].([]any)
		require.True(t, ok)
		foundUser12 := false
		foundUser18 := false
		require.GreaterOrEqual(t, len(matches), 2, "User 13 should now have at least 2 matches")
		for _, m := range matches {
			matchMap, _ := m.(map[string]any)
			matchedID := int32(matchMap["matched_user_id"].(float64))
			if matchedID == testUserID12 {
				foundUser12 = true
			}
			if matchedID == testUserID18 {
				foundUser18 = true
				assert.Nil(t, matchMap["last_event_user_id"], "Last event user ID should be null/absent for new match")
				assert.Nil(t, matchMap["last_event_type"], "Last event type should be null/absent for new match")
			}
		}
		assert.True(t, foundUser12, "User 12 should still be in User 13's matches")
		assert.True(t, foundUser18, "User 18 should now be in User 13's matches")
	})

	t.Run("Dislike_User17_Dislikes_User13", func(t *testing.T) {
		payload := handlers.DislikeRequest{DislikedUserID: testUserID13}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "POST", "/api/dislike", &testUser17Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		assertSuccessResponse(t, resp, http.StatusOK)

		reqFeed := makeRequest(t, "GET", "/api/homefeed", &testUser13Token, nil)
		respFeed := executeRequest(t, reqFeed)
		bodyFeed := assertSuccessResponse(t, respFeed, http.StatusOK)
		profilesFeed, _ := bodyFeed["profiles"].([]any)
		for _, p := range profilesFeed {
			profileMap, _ := p.(map[string]any)
			profileID := int32(profileMap["ID"].(float64))
			assert.NotEqual(t, testUserID17, profileID, "Disliked User 17 should not be in User 13's feed")
		}
	})

	t.Run("Unmatch_User13_Unmatches_User12", func(t *testing.T) {
		payload := handlers.UnmatchRequest{TargetUserID: testUserID12}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "POST", "/api/unmatch", &testUser13Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		assertSuccessResponse(t, resp, http.StatusOK)

		reqMatches := makeRequest(t, "GET", "/api/matches", &testUser13Token, nil)
		respMatches := executeRequest(t, reqMatches)
		bodyMatches := assertSuccessResponse(t, respMatches, http.StatusOK)
		matches, _ := bodyMatches["matches"].([]any)
		for _, m := range matches {
			matchMap, _ := m.(map[string]any)
			matchedID := int32(matchMap["matched_user_id"].(float64))
			assert.NotEqual(t, testUserID12, matchedID, "Unmatched User 12 should not be in User 13's matches")
		}

		reqFeed := makeRequest(t, "GET", "/api/homefeed", &testUser13Token, nil)
		respFeed := executeRequest(t, reqFeed)
		bodyFeed := assertSuccessResponse(t, respFeed, http.StatusOK)
		profilesFeed, _ := bodyFeed["profiles"].([]any)
		for _, p := range profilesFeed {
			profileMap, _ := p.(map[string]any)
			profileID := int32(profileMap["ID"].(float64))
			assert.NotEqual(t, testUserID12, profileID, "Unmatched User 12 (disliked by 13) should not be in User 13's feed")
		}
	})

}

func TestChatAndRelatedEndpoints(t *testing.T) {
	conn13, _ := connectWebSocket(t, testUser13Token)
	defer conn13.Close()
	conn12, _ := connectWebSocket(t, testUser12Token)
	defer conn12.Close()

	t.Run("WebSocket_SendMessage_13_to_12", func(t *testing.T) {
		testMsgText := fmt.Sprintf("Hello Shruti! Unique: %d", time.Now().UnixNano())
		msgToSend := ws.WsMessage{
			Type:            "chat_message",
			RecipientUserID: PtrInt32(testUserID12),
			Text:            ws.Ptr(testMsgText),
		}
		err := writeWsMessage(t, conn13, msgToSend)
		require.NoError(t, err)

		ackMsg := assertWsMessageType(t, conn13, "message_ack", wsReadWait)
		require.NotNil(t, ackMsg.ID, "ACK should have message ID")
		assert.Equal(t, "Message delivered.", *ackMsg.Content)
		savedMsgID := *ackMsg.ID

		receivedMsg := assertWsMessageType(t, conn12, "chat_message", wsReadWait)
		require.NotNil(t, receivedMsg.ID, "Received message should have ID")
		assert.Equal(t, savedMsgID, *receivedMsg.ID)
		require.NotNil(t, receivedMsg.SenderUserID)
		assert.Equal(t, testUserID13, *receivedMsg.SenderUserID)
		require.NotNil(t, receivedMsg.RecipientUserID)
		assert.Equal(t, testUserID12, *receivedMsg.RecipientUserID)
		require.NotNil(t, receivedMsg.Text)
		assert.Equal(t, testMsgText, *receivedMsg.Text)
		assert.Nil(t, receivedMsg.MediaURL)
		assert.Nil(t, receivedMsg.MediaType)
		require.NotNil(t, receivedMsg.SentAt)
		_, err = time.Parse(time.RFC3339Nano, *receivedMsg.SentAt)
		assert.NoError(t, err, "SentAt should be valid timestamp")
	})

	time.Sleep(100 * time.Millisecond)
	var lastMsgID13to12 int64
	t.Run("WebSocket_Helper_GetLastMessageID", func(t *testing.T) {
		reqBody := handlers.GetConversationRequest{OtherUserID: testUserID12}
		jsonBody, _ := json.Marshal(reqBody)
		req := makeRequest(t, "POST", "/api/conversation", &testUser13Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		var convResp handlers.GetConversationResponse
		decodeResponse(t, resp, &convResp)
		require.True(t, convResp.Success)
		require.NotEmpty(t, convResp.Messages)
		lastMsg := convResp.Messages[len(convResp.Messages)-1]
		require.Equal(t, testUserID13, lastMsg.SenderUserID)
		lastMsgID13to12 = lastMsg.ID
		t.Logf("Found last message ID from 13 -> 12: %d", lastMsgID13to12)
	})
	require.NotZero(t, lastMsgID13to12, "Failed to get last message ID for subsequent tests")


	t.Run("WebSocket_SendReaction_12_to_13s_Message", func(t *testing.T) {
		reactionEmoji := "😊"
		msgToSend := ws.WsMessage{
			Type:      "react_to_message",
			MessageID: ws.PtrInt64(lastMsgID13to12),
			Emoji:     ws.Ptr(reactionEmoji),
		}
		err := writeWsMessage(t, conn12, msgToSend)
		require.NoError(t, err)

		ackMsg := assertWsMessageType(t, conn12, "reaction_ack", wsReadWait)
		require.NotNil(t, ackMsg.MessageID)
		assert.Equal(t, lastMsgID13to12, *ackMsg.MessageID)
		assert.Equal(t, "Reaction added.", *ackMsg.Content)

		updateMsg := assertWsMessageType(t, conn13, "reaction_update", wsReadWait)
		require.NotNil(t, updateMsg.MessageID)
		assert.Equal(t, lastMsgID13to12, *updateMsg.MessageID)
		require.NotNil(t, updateMsg.ReactorUserID)
		assert.Equal(t, testUserID12, *updateMsg.ReactorUserID)
		require.NotNil(t, updateMsg.Emoji)
		assert.Equal(t, reactionEmoji, *updateMsg.Emoji)
		require.NotNil(t, updateMsg.IsRemoved)
		assert.False(t, *updateMsg.IsRemoved)
	})

	t.Run("WebSocket_SendTyping_13_to_12", func(t *testing.T) {
		msgStart := ws.WsMessage{Type: "typing_event", RecipientUserID: PtrInt32(testUserID12), IsTyping: ws.PtrBool(true)}
		err := writeWsMessage(t, conn13, msgStart)
		require.NoError(t, err)

		statusMsg := assertWsMessageType(t, conn12, "typing_status", wsReadWait)
		require.NotNil(t, statusMsg.TypingUserID)
		assert.Equal(t, testUserID13, *statusMsg.TypingUserID)
		require.NotNil(t, statusMsg.IsTyping)
		assert.True(t, *statusMsg.IsTyping)

		time.Sleep(100 * time.Millisecond)

		msgStop := ws.WsMessage{Type: "typing_event", RecipientUserID: PtrInt32(testUserID12), IsTyping: ws.PtrBool(false)}
		err = writeWsMessage(t, conn13, msgStop)
		require.NoError(t, err)

		statusMsgStop := assertWsMessageType(t, conn12, "typing_status", wsReadWait)
		require.NotNil(t, statusMsgStop.TypingUserID)
		assert.Equal(t, testUserID13, *statusMsgStop.TypingUserID)
		require.NotNil(t, statusMsgStop.IsTyping)
		assert.False(t, *statusMsgStop.IsTyping)
	})


	t.Run("WebSocket_MarkRead_12_marks_13s_Message", func(t *testing.T) {
		msgToSend := ws.WsMessage{
			Type:        "mark_read",
			OtherUserID: PtrInt32(testUserID13),
			MessageID:   ws.PtrInt64(lastMsgID13to12),
		}
		err := writeWsMessage(t, conn12, msgToSend)
		require.NoError(t, err)

		ackMsg := assertWsMessageType(t, conn12, "mark_read_ack", wsReadWait)
		require.NotNil(t, ackMsg.MessageID)
		assert.Equal(t, lastMsgID13to12, *ackMsg.MessageID)
		require.NotNil(t, ackMsg.OtherUserID)
		assert.Equal(t, testUserID13, *ackMsg.OtherUserID)
		require.NotNil(t, ackMsg.Count)
		assert.GreaterOrEqual(t, *ackMsg.Count, int64(1), "Should have marked at least one message read")

		updateMsg := assertWsMessageType(t, conn13, "messages_read_update", wsReadWait)
		require.NotNil(t, updateMsg.ReaderUserID)
		assert.Equal(t, testUserID12, *updateMsg.ReaderUserID)
		require.NotNil(t, updateMsg.MessageID)
		assert.Equal(t, lastMsgID13to12, *updateMsg.MessageID)
	})


	t.Run("GetConversation_13_with_12", func(t *testing.T) {
		reqBody := handlers.GetConversationRequest{OtherUserID: testUserID12}
		jsonBody, _ := json.Marshal(reqBody)
		req := makeRequest(t, "POST", "/api/conversation", &testUser13Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		var convResp handlers.GetConversationResponse
		require.Equal(t, http.StatusOK, resp.StatusCode)
		decodeResponse(t, resp, &convResp)

		assert.True(t, convResp.Success)
		assert.NotEmpty(t, convResp.Messages, "Conversation should not be empty")

		lastMsg := convResp.Messages[len(convResp.Messages)-1]
		assert.Equal(t, lastMsgID13to12, lastMsg.ID)
		assert.True(t, lastMsg.IsRead, "Last message should now be marked as read")

		assert.JSONEq(t, `{"😊": 1}`, string(lastMsg.Reactions), "Reactions mismatch")
		assert.Nil(t, lastMsg.CurrentUserReaction, "Current user (13) did not react")
	})


	t.Run("GetUnreadCount_User13", func(t *testing.T) {
		req := makeRequest(t, "GET", "/api/unread-chat-count", &testUser13Token, nil)
		resp := executeRequest(t, req)
		var countsResp handlers.NotificationCountsResponse
		require.Equal(t, http.StatusOK, resp.StatusCode)
		decodeResponse(t, resp, &countsResp)

		assert.True(t, countsResp.Success)
		assert.GreaterOrEqual(t, countsResp.UnreadChatCount, int64(0))
		assert.GreaterOrEqual(t, countsResp.UnseenLikeCount, int64(0))
	})

	t.Run("GetLastOnline_User13_for_User12", func(t *testing.T) {
		reqBody := handlers.FetchLastOnlineRequest{UserID: testUserID12}
		jsonBody, _ := json.Marshal(reqBody)
		req := makeRequest(t, "POST", "/api/user/last-online", &testUser13Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		var onlineResp handlers.FetchLastOnlineResponse
		require.Equal(t, http.StatusOK, resp.StatusCode)
		decodeResponse(t, resp, &onlineResp)

		assert.True(t, onlineResp.Success)
		assert.True(t, onlineResp.IsOnline, "User 12 should be online (WS connection is active)")
		assert.NotNil(t, onlineResp.LastOnline, "LastOnline should not be nil even if online")
	})

}

func TestAdminEndpoints(t *testing.T) {
	t.Parallel()


	t.Run("GetPendingVerifications_Admin", func(t *testing.T) {
		req := makeRequest(t, "GET", "/api/admin/verifications", &testUserAdminToken, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		requests, ok := body["verification_requests"].([]any)
		require.True(t, ok)
		count, okCount := body["count"].(float64)
		require.True(t, okCount)
		assert.Equal(t, float64(len(requests)), count)
		assert.Len(t, requests, 0, "Expected 0 pending verifications based on seed")
	})

	var tempUserID int32 = testUserID17
	t.Run("Admin_Setup_MakeUserPending", func(t *testing.T) {
		ctx := context.Background()
		queries, _ := db.GetDB()
		_, err := queries.UpdateUserVerificationDetails(ctx, migrations.UpdateUserVerificationDetailsParams{
			ID:                 tempUserID,
			VerificationPic:    pgtype.Text{String: "http://example.com/verify.jpg", Valid: true},
			VerificationStatus: migrations.VerificationStatusPending,
		})
		require.NoError(t, err)
	})

	t.Run("UpdateVerificationStatus_Admin_Approve", func(t *testing.T) {
		payload := handlers.VerificationActionRequest{
			UserID:  tempUserID,
			Approve: true,
		}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "POST", "/api/admin/verify", &testUserAdminToken, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		assert.Equal(t, float64(tempUserID), body["user_id"])
		assert.Equal(t, string(migrations.VerificationStatusTrue), body["status"])

		queries, _ := db.GetDB()
		user, err := queries.GetUserByID(context.Background(), tempUserID)
		require.NoError(t, err)
		assert.Equal(t, migrations.VerificationStatusTrue, user.VerificationStatus)
	})

	t.Run("Admin_Setup_MakeUserPendingAgain", func(t *testing.T) {
		ctx := context.Background()
		queries, _ := db.GetDB()
		_, err := queries.UpdateUserVerificationDetails(ctx, migrations.UpdateUserVerificationDetailsParams{
			ID:                 tempUserID,
			VerificationPic:    pgtype.Text{String: "http://example.com/verify2.jpg", Valid: true},
			VerificationStatus: migrations.VerificationStatusPending,
		})
		require.NoError(t, err)
	})

	t.Run("UpdateVerificationStatus_Admin_Reject", func(t *testing.T) {
		payload := handlers.VerificationActionRequest{
			UserID:  tempUserID,
			Approve: false,
		}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "POST", "/api/admin/verify", &testUserAdminToken, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		assert.Equal(t, float64(tempUserID), body["user_id"])
		assert.Equal(t, string(migrations.VerificationStatusFalse), body["status"])

		queries, _ := db.GetDB()
		user, err := queries.GetUserByID(context.Background(), tempUserID)
		require.NoError(t, err)
		assert.Equal(t, migrations.VerificationStatusFalse, user.VerificationStatus)
	})

	t.Run("SetAdminRole_MakeUserAdmin", func(t *testing.T) {
		_ = "kushal@example.com"
		queries, _ := db.GetDB()
		user17, err := queries.GetUserByEmail(context.Background(), "ayushiitroorkie@gmail.com")
		require.NoError(t, err)
		require.Equal(t, migrations.UserRoleUser, user17.Role, "User 17 should initially be a user")

		payload := handlers.SetAdminRequest{
			Email:   user17.Email,
			IsAdmin: true,
		}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "POST", "/api/set-admin", &testUserAdminToken, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		assert.Equal(t, float64(user17.ID), body["user_id"])
		assert.Equal(t, string(migrations.UserRoleAdmin), body["role"])

		user17Updated, _ := queries.GetUserByID(context.Background(), user17.ID)
		assert.Equal(t, migrations.UserRoleAdmin, user17Updated.Role)

		payloadRevert := handlers.SetAdminRequest{Email: user17.Email, IsAdmin: false}
		jsonBodyRevert, _ := json.Marshal(payloadRevert)
		reqRev := makeRequest(t, "POST", "/api/set-admin", &testUserAdminToken, bytes.NewBuffer(jsonBodyRevert))
		executeRequest(t, reqRev)
		user17Reverted, _ := queries.GetUserByID(context.Background(), user17.ID)
		assert.Equal(t, migrations.UserRoleUser, user17Reverted.Role)

	})

	t.Run("AdminEndpoint_NonAdminAccess", func(t *testing.T) {
		req := makeRequest(t, "GET", "/api/admin/verifications", &testUser13Token, nil)
		resp := executeRequest(t, req)
		assertErrorResponse(t, resp, http.StatusForbidden, "Admin access required")
	})

}

func TestMiscEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("ReportUser_User13_Reports_User17", func(t *testing.T) {
		payload := handlers.ReportRequest{
			ReportedUserID: testUserID17,
			Reason:         string(migrations.ReportReasonInappropriate),
		}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "POST", "/api/report", &testUser13Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		assertSuccessResponse(t, resp, http.StatusOK)

	})

	t.Run("ReportUser_Self", func(t *testing.T) {
		payload := handlers.ReportRequest{
			ReportedUserID: testUserID13,
			Reason:         string(migrations.ReportReasonSpam),
		}
		jsonBody, _ := json.Marshal(payload)
		req := makeRequest(t, "POST", "/api/report", &testUser13Token, bytes.NewBuffer(jsonBody))
		resp := executeRequest(t, req)
		assertErrorResponse(t, resp, http.StatusBadRequest, "Cannot report yourself")
	})

	t.Run("AppOpened_User13", func(t *testing.T) {
		req := makeRequest(t, "POST", "/api/app-opened", &testUser13Token, nil)
		resp := executeRequest(t, req)
		assertSuccessResponse(t, resp, http.StatusOK)
	})

	t.Run("BaseProtected_User13", func(t *testing.T) {
		req := makeRequest(t, "GET", "/", &testUser13Token, nil)
		resp := executeRequest(t, req)
		body := assertSuccessResponse(t, resp, http.StatusOK)
		assert.Equal(t, float64(testUserID13), body["user_id"])
	})

	t.Run("BaseProtected_NoToken", func(t *testing.T) {
		req := makeRequest(t, "GET", "/", nil, nil)
		resp := executeRequest(t, req)
		assertErrorResponse(t, resp, http.StatusUnauthorized, "Invalid Authorization header format")
	})

	t.Run("TestRoute_Unprotected", func(t *testing.T) {
		req := makeRequest(t, "GET", "/test", nil, nil)
		resp := executeRequest(t, req)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		defer resp.Body.Close()
		bodyBytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "testing", string(bodyBytes))
	})


}

func PtrInt32(i int32) *int32 {
	return &i
}
