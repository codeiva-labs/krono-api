package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"codeiva/krono-api/app/model"
	"codeiva/krono-api/pkg/response"
)

type activityRequest struct {
	CategoryID  string     `json:"category_id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
}

type activityResponse struct {
	ID          string   `json:"id"`
	CategoryID  string   `json:"category_id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	StartTime   string   `json:"start_time"`
	EndTime     *string  `json:"end_time,omitempty"`
	Duration    int64    `json:"duration,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// GetActivities returns all activities for the authenticated user
func GetActivities(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
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
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	filter := bson.M{"user_id": userObjID}
	cursor, err := collections.Activities.Find(ctx, filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch activities")
		return
	}
	defer cursor.Close(ctx)
	var activities []model.Activity
	if err := cursor.All(ctx, &activities); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to decode activities")
		return
	}
	result := make([]activityResponse, len(activities))
	for i, act := range activities {
		result[i] = toActivityResponse(act)
	}
	response.Success(w, result)
}

// GetActivityDetail returns a single activity by id for the authenticated user
func GetActivityDetail(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	vars := mux.Vars(r)
	activityID := vars["activity_id"]
	if activityID == "" {
		response.Error(w, http.StatusBadRequest, "activity id is required")
		return
	}
	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}
	activityObjID, err := primitive.ObjectIDFromHex(activityID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid activity id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var activity model.Activity
	err = collections.Activities.FindOne(ctx, bson.M{"_id": activityObjID, "user_id": userObjID}).Decode(&activity)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			response.Error(w, http.StatusNotFound, "activity not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to fetch activity")
		return
	}
	response.Success(w, toActivityResponse(activity))
}

// AddActivity creates a new activity for the authenticated user
func AddActivity(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req activityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.CategoryID == "" || req.Title == "" || req.StartTime.IsZero() {
		response.Error(w, http.StatusBadRequest, "category_id, title, and start_time are required")
		return
	}
	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}
	categoryObjID, err := primitive.ObjectIDFromHex(req.CategoryID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid category id")
		return
	}
	now := time.Now().UTC()
	activity := model.Activity{
		UserID:      userObjID,
		CategoryID:  categoryObjID,
		Title:       req.Title,
		Description: req.Description,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Tags:        req.Tags,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if req.EndTime != nil {
		activity.Duration = int64(req.EndTime.Sub(req.StartTime).Seconds())
	}
	result, err := collections.Activities.InsertOne(context.Background(), activity)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create activity")
		return
	}
	activity.ID = result.InsertedID.(primitive.ObjectID)
	response.Created(w, toActivityResponse(activity))
}

// EditActivity updates an activity for the authenticated user
func EditActivity(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	vars := mux.Vars(r)
	activityID := vars["activity_id"]
	if activityID == "" {
		response.Error(w, http.StatusBadRequest, "activity id is required")
		return
	}
	var req activityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}
	activityObjID, err := primitive.ObjectIDFromHex(activityID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid activity id")
		return
	}
	update := bson.M{
		"title":       req.Title,
		"description": req.Description,
		"tags":        req.Tags,
		"updated_at":  time.Now().UTC(),
	}
	if req.CategoryID != "" {
		categoryObjID, err := primitive.ObjectIDFromHex(req.CategoryID)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid category id")
			return
		}
		update["category_id"] = categoryObjID
	}
	if req.StartTime != (time.Time{}) {
		update["start_time"] = req.StartTime
	}
	if req.EndTime != nil {
		update["end_time"] = req.EndTime
		update["duration"] = int64(req.EndTime.Sub(req.StartTime).Seconds())
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := collections.Activities.UpdateOne(ctx, bson.M{"_id": activityObjID, "user_id": userObjID}, bson.M{"$set": update})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update activity")
		return
	}
	if result.MatchedCount == 0 {
		response.Error(w, http.StatusNotFound, "activity not found or not owned by you")
		return
	}
	response.Success(w, map[string]string{"status": "updated"})
}

// DeleteActivity deletes an activity for the authenticated user
func DeleteActivity(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	vars := mux.Vars(r)
	activityID := vars["activity_id"]
	if activityID == "" {
		response.Error(w, http.StatusBadRequest, "activity id is required")
		return
	}
	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}
	activityObjID, err := primitive.ObjectIDFromHex(activityID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid activity id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := collections.Activities.DeleteOne(ctx, bson.M{"_id": activityObjID, "user_id": userObjID})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to delete activity")
		return
	}
	if result.DeletedCount == 0 {
		response.Error(w, http.StatusNotFound, "activity not found or not owned by you")
		return
	}
	response.Success(w, map[string]string{"status": "deleted"})
}

// toActivityResponse converts a model.Activity to activityResponse
func toActivityResponse(act model.Activity) activityResponse {
	resp := activityResponse{
		ID:          act.ID.Hex(),
		CategoryID:  act.CategoryID.Hex(),
		Title:       act.Title,
		Description: act.Description,
		StartTime:   act.StartTime.Format(time.RFC3339),
		Tags:        act.Tags,
		CreatedAt:   act.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   act.UpdatedAt.Format(time.RFC3339),
	}
	if act.EndTime != nil {
		end := act.EndTime.Format(time.RFC3339)
		resp.EndTime = &end
		resp.Duration = act.Duration
	}
	return resp
}
