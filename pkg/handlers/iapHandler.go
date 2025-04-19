package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

// VerifyPurchaseRequest is the expected structure from the mobile app
type VerifyPurchaseRequest struct {
	Platform      string `json:"platform"`
	ReceiptData   string `json:"receipt_data"`
	ProductID     string `json:"product_id"`
	TransactionID string `json:"transaction_id"`
}

// VerifyPurchaseResponse is sent back to the mobile app
type VerifyPurchaseResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// --- Placeholder Verification ---
func verifyReceipt(platform, receiptData, productID, transactionID string) (bool, error) {
	log.Printf("[DEBUG IAP Verify] Simulating receipt verification...")
	log.Printf("[DEBUG IAP Verify]   Platform: %s", platform)
	log.Printf("[DEBUG IAP Verify]   ProductID: %s", productID)
	log.Printf("[DEBUG IAP Verify]   TxID: %s", transactionID)
	if platform != "ios" && platform != "android" {
		log.Printf("[ERROR IAP Verify] Invalid platform")
		return false, errors.New("invalid platform specified")
	}
	if receiptData == "" || productID == "" || transactionID == "" {
		log.Printf("[ERROR IAP Verify] Missing required data")
		return false, errors.New("missing required purchase data for verification")
	}
	time.Sleep(50 * time.Millisecond)
	log.Printf("[DEBUG IAP Verify] Simulation successful.")
	return true, nil
}

// VerifyPurchaseHandler handles requests from the app to verify IAPs
func VerifyPurchaseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, _ := db.GetDB()
	if queries == nil {
		log.Println("ERROR: VerifyPurchaseHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, VerifyPurchaseResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, VerifyPurchaseResponse{Success: false, Message: "Method Not Allowed: Use POST"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, VerifyPurchaseResponse{Success: false, Message: "Authentication required"})
		return
	}
	userID := int32(claims.UserID)

	var req VerifyPurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, VerifyPurchaseResponse{Success: false, Message: "Invalid request body format"})
		return
	}
	defer r.Body.Close()

	if req.Platform == "" || req.ReceiptData == "" || req.ProductID == "" || req.TransactionID == "" {
		utils.RespondWithJSON(w, http.StatusBadRequest, VerifyPurchaseResponse{Success: false, Message: "Missing required fields: platform, receipt_data, product_id, transaction_id"})
		return
	}
	log.Printf("[DEBUG VerifyHandler] Processing IAP: User=%d, Product=%s, TxID=%s", userID, req.ProductID, req.TransactionID)

	// --- Step 1: Verify Receipt (Placeholder) ---
	verified, err := verifyReceipt(req.Platform, req.ReceiptData, req.ProductID, req.TransactionID)
	if err != nil || !verified {
		log.Printf("[ERROR VerifyHandler] IAP verification failed: User=%d, TxID=%s, Error=%v", userID, req.TransactionID, err)
		utils.RespondWithJSON(w, http.StatusBadRequest, VerifyPurchaseResponse{Success: false, Message: "Purchase verification failed"})
		return
	}
	log.Printf("[DEBUG VerifyHandler] IAP receipt/token verified (simulated): User=%d, TxID=%s", userID, req.TransactionID)

	// --- Step 2: Check for Replay Attack (TODO) ---
	log.Printf("[DEBUG VerifyHandler] Skipping replay check: User=%d, TxID=%s (TODO)", userID, req.TransactionID)

	// --- Step 3: Grant Feature based on Product ID ---
	var grantErr error
	productParts := strings.Split(req.ProductID, "_")
	log.Printf("[DEBUG VerifyHandler] Product ID Parts (Split by '_'): %q", productParts)

	if len(productParts) < 3 {
		log.Printf("[ERROR VerifyHandler] Invalid product ID format (expected at least 3 parts separated by _): User=%d, Product=%s, Parts=%d", userID, req.ProductID, len(productParts))
		utils.RespondWithJSON(w, http.StatusBadRequest, VerifyPurchaseResponse{Success: false, Message: "Invalid product identifier format"})
		return
	}

	// ** FINAL PARSING LOGIC **
	detail := productParts[len(productParts)-1] // Value/Duration is always last.
	log.Printf("[DEBUG VerifyHandler] Assumed Detail Part: %s", detail)

	// Identify feature based on expected structure parts using HasSuffix for flexibility
	actualFeatureType := ""
	isConsumable := false
	isSubscription := false

	partBeforeDetail := productParts[len(productParts)-2]
	partTwoBeforeDetail := productParts[len(productParts)-3] // Part potentially containing the core feature name

	log.Printf("[DEBUG VerifyHandler] Part before detail: '%s'", partBeforeDetail)
	log.Printf("[DEBUG VerifyHandler] Part two before detail: '%s'", partTwoBeforeDetail)

	// Check Consumables: ..._feature_pack_VALUE
	if partBeforeDetail == "pack" {
		if strings.HasSuffix(partTwoBeforeDetail, "rose") { // Check if it ends with rose (e.g., com.yourapp.rose)
			actualFeatureType = "rose"
			isConsumable = true
			log.Printf("[DEBUG VerifyHandler] Matched pattern: ..._rose_pack_...")
		} else if strings.HasSuffix(partTwoBeforeDetail, "spotlight") { // Check if it ends with spotlight
			actualFeatureType = "spotlight"
			isConsumable = true
			log.Printf("[DEBUG VerifyHandler] Matched pattern: ..._spotlight_pack_...")
		}
		// Check Subscriptions: ..._feature_descriptor_VALUE
	} else if partBeforeDetail == "likes" { // Descriptor is feature itself
		if strings.HasSuffix(partTwoBeforeDetail, "unlimited") { // Check qualifier before
			actualFeatureType = "likes"
			isSubscription = true
			log.Printf("[DEBUG VerifyHandler] Matched pattern: ..._unlimited_likes_...")
		}
	} else if partBeforeDetail == "mode" { // Descriptor is mode
		if strings.HasSuffix(partTwoBeforeDetail, "travel") { // Check qualifier before
			actualFeatureType = "travel"
			isSubscription = true
			log.Printf("[DEBUG VerifyHandler] Matched pattern: ..._travel_mode_...")
		}
	}

	// Final check if a type was identified
	if actualFeatureType == "" {
		log.Printf("[ERROR VerifyHandler] Could not match known product structure: Product='%s', User=%d", req.ProductID, userID)
		utils.RespondWithJSON(w, http.StatusBadRequest, VerifyPurchaseResponse{Success: false, Message: "Unrecognized product structure"})
		return
	}
	log.Printf("[DEBUG VerifyHandler] Determined Feature Type: '%s', Detail: '%s', IsConsumable: %t, IsSubscription: %t", actualFeatureType, detail, isConsumable, isSubscription)

	// Grant based on the identified feature key and type
	if isConsumable {
		switch actualFeatureType {
		case "rose":
			grantErr = grantConsumable(ctx, queries, userID, migrations.PremiumFeatureTypeRose, detail)
		case "spotlight":
			grantErr = grantConsumable(ctx, queries, userID, migrations.PremiumFeatureTypeSpotlight, detail)
		default:
			grantErr = fmt.Errorf("internal error: unknown consumable type '%s'", actualFeatureType)
		}
	} else if isSubscription {
		switch actualFeatureType {
		case "likes":
			grantErr = grantSubscription(ctx, queries, userID, migrations.PremiumFeatureTypeUnlimitedLikes, detail)
		case "travel":
			grantErr = grantSubscription(ctx, queries, userID, migrations.PremiumFeatureTypeTravelMode, detail)
		default:
			grantErr = fmt.Errorf("internal error: unknown subscription type '%s'", actualFeatureType)
		}
	} else {
		// Should not happen if logic above is complete
		grantErr = fmt.Errorf("internal error: product identified but type (consumable/subscription) unknown")
	}
	// ** END OF FINAL PARSING LOGIC **

	if grantErr != nil {
		log.Printf("[ERROR VerifyHandler] Failed to grant feature: Product=%s, Type=%s, Detail=%s, User=%d, Error=%v", req.ProductID, actualFeatureType, detail, userID, grantErr)
		utils.RespondWithJSON(w, http.StatusInternalServerError, VerifyPurchaseResponse{Success: false, Message: "Failed to update user features"})
		return
	}

	// --- Step 4: Mark Transaction as Processed (TODO) ---
	log.Printf("[DEBUG VerifyHandler] Skipping marking TxID %s as processed (TODO)", req.TransactionID)

	// --- Step 5: Respond Success ---
	log.Printf("[INFO VerifyHandler] Successfully processed IAP: User=%d, Product=%s, TxID=%s", userID, req.ProductID, req.TransactionID)
	utils.RespondWithJSON(w, http.StatusOK, VerifyPurchaseResponse{Success: true, Message: "Purchase verified and feature granted"})
}

// grantConsumable updates the user's consumable balance
func grantConsumable(ctx context.Context, queries *migrations.Queries, userID int32, consumableType migrations.PremiumFeatureType, detail string) error {
	log.Printf("[DEBUG grantConsumable] Granting: User=%d, Type=%s, Detail=%s", userID, consumableType, detail)
	quantity, err := strconv.Atoi(detail)
	if err != nil || quantity <= 0 {
		log.Printf("[ERROR grantConsumable] Invalid quantity: Detail='%s', User=%d, Error=%v", detail, userID, err)
		return fmt.Errorf("invalid quantity detail '%s' for consumable product", detail)
	}

	_, dbErr := queries.UpsertUserConsumable(ctx, migrations.UpsertUserConsumableParams{
		UserID:         userID,
		ConsumableType: consumableType,
		Quantity:       int32(quantity),
	})

	if dbErr != nil {
		log.Printf("[ERROR grantConsumable] DB Error: User=%d, Type=%s, Error=%v", userID, consumableType, dbErr)
		return fmt.Errorf("database error upserting consumable %s for user %d: %w", consumableType, userID, dbErr)
	}
	log.Printf("[INFO grantConsumable] Granted %d of %s to User %d", quantity, consumableType, userID)
	return nil
}

// grantSubscription adds a new subscription record for the user
func grantSubscription(ctx context.Context, queries *migrations.Queries, userID int32, featureType migrations.PremiumFeatureType, detail string) error {
	log.Printf("[DEBUG grantSubscription] Granting: User=%d, Type=%s, Detail=%s", userID, featureType, detail)
	var duration time.Duration
	normalizedDetail := strings.ToLower(strings.TrimSpace(detail))

	switch normalizedDetail {
	case "1day":
		duration = 24 * time.Hour
	case "1week":
		duration = 7 * 24 * time.Hour
	// Add other durations as needed
	default:
		log.Printf("[ERROR grantSubscription] Invalid duration: Detail='%s', User=%d", detail, userID)
		return fmt.Errorf("invalid duration detail '%s' for subscription product", detail)
	}

	expiresAt := time.Now().Add(duration)

	_, dbErr := queries.AddUserSubscription(ctx, migrations.AddUserSubscriptionParams{
		UserID:      userID,
		FeatureType: featureType,
		ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})

	if dbErr != nil {
		log.Printf("[ERROR grantSubscription] DB Error: User=%d, Type=%s, Error=%v", userID, featureType, dbErr)
		return fmt.Errorf("database error adding subscription %s for user %d: %w", featureType, userID, dbErr)
	}

	log.Printf("[INFO grantSubscription] Granted subscription %s to User %d, expiring at %s", featureType, userID, expiresAt.Format(time.RFC3339))
	return nil
}
