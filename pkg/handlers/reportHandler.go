package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
)

type ReportRequest struct {
	ReportedUserID int32  `json:"reported_user_id"`
	Reason         string `json:"reason"`
}

type ReportResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func parseReportReason(reasonStr string) (migrations.ReportReason, error) {
	reasonEnum := migrations.ReportReason(reasonStr)
	switch reasonEnum {
	case migrations.ReportReasonNotInterested,
		migrations.ReportReasonFakeProfile,
		migrations.ReportReasonInappropriate,
		migrations.ReportReasonMinor,
		migrations.ReportReasonSpam:
		return reasonEnum, nil
	default:
		return "", fmt.Errorf("invalid report reason: '%s'", reasonStr)
	}
}

func ReportHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, er := db.GetDB()
	if er != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Database connection not available")
		return
	}

	if r.Method != http.MethodPost {
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use POST")
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	reporterUserID := int32(claims.UserID)

	var req ReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body format")
		return
	}
	defer r.Body.Close()

	if req.ReportedUserID <= 0 {
		utils.RespondWithError(w, http.StatusBadRequest, "Valid reported_user_id is required")
		return
	}

	if req.ReportedUserID == reporterUserID {
		utils.RespondWithError(w, http.StatusBadRequest, "Cannot report yourself")
		return
	}

	reasonEnum, err := parseReportReason(req.Reason)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	_, err = queries.GetUserByID(ctx, req.ReportedUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondWithError(w, http.StatusNotFound, "User being reported does not exist")
		} else {
			log.Printf("ReportHandler: Error checking reported user %d existence: %v", req.ReportedUserID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Error checking reported user")
		}
		return
	}

	params := migrations.CreateReportParams{
		ReporterUserID: reporterUserID,
		ReportedUserID: req.ReportedUserID,
		Reason:         reasonEnum,
	}

	log.Printf("ReportHandler: Attempting to create report: Reporter=%d, Reported=%d, Reason=%s",
		reporterUserID, req.ReportedUserID, reasonEnum)

	createdReport, err := queries.CreateReport(ctx, params)
	if err != nil {
		log.Printf("ReportHandler: Error creating report: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to submit report")
		return
	}

	log.Printf("ReportHandler: Report created successfully: ID=%d", createdReport.ID)

	utils.RespondWithJSON(w, http.StatusOK, ReportResponse{
		Success: true,
		Message: "User reported successfully",
	})
}
