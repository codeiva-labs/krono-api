package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"codeiva/krono-api/app/model"
	"codeiva/krono-api/pkg/response"
)

type dataDecisionRequest struct {
	Action string `json:"action"` // "restore" or "fresh"
}

// SetDataDecision applies the user's choice, shown on a new device when
// existing (non-archived) data is available, of whether to keep using that
// data ("restore") or start over. "fresh" archives the user's current
// activities and custom categories instead of deleting them, so a start-over
// never loses data.
func SetDataDecision(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req dataDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Action != "restore" && req.Action != "fresh" {
		response.Error(w, http.StatusBadRequest, "action must be 'restore' or 'fresh'")
		return
	}

	if req.Action == "restore" {
		response.Success(w, map[string]string{"status": "restored"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	archiveFilter := bson.M{"user_id": userObjID, "archived": bson.M{"$ne": true}}
	archiveUpdate := bson.M{"$set": bson.M{"archived": true, "archived_at": now}}

	if _, err := collections.Activities.UpdateMany(ctx, archiveFilter, archiveUpdate); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to archive activities")
		return
	}
	if _, err := collections.Categories.UpdateMany(ctx, archiveFilter, archiveUpdate); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to archive categories")
		return
	}

	response.Success(w, map[string]string{"status": "fresh"})
}
