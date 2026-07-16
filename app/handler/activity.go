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
	CategoryID string     `json:"category_id"`
	Title      string     `json:"title"`
	StartTime  time.Time  `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
}

type activityResponse struct {
	ID         string            `json:"id"`
	CategoryID string            `json:"category_id"`
	Category   *categoryResponse `json:"category,omitempty"`
	Title      string            `json:"title"`
	StartTime  string            `json:"start_time"`
	EndTime    *string           `json:"end_time,omitempty"`
	Duration   int64             `json:"duration,omitempty"`
	CreatedAt  string            `json:"created_at"`
	UpdatedAt  string            `json:"updated_at"`
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

	// Build match filter
	matchFilter := bson.D{{Key: "user_id", Value: userObjID}}

	if startStr := r.URL.Query().Get("start"); startStr != "" {
		startTime, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid start time, use RFC3339 format")
			return
		}
		matchFilter = append(matchFilter, bson.E{Key: "start_time", Value: bson.D{{Key: "$gte", Value: startTime}}})
	}

	if endStr := r.URL.Query().Get("end"); endStr != "" {
		endTime, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid end time, use RFC3339 format")
			return
		}
		matchFilter = append(matchFilter, bson.E{Key: "end_time", Value: bson.D{{Key: "$lte", Value: endTime}}})
	}

	if categoryIDStr := r.URL.Query().Get("category_id"); categoryIDStr != "" {
		categoryObjID, err := primitive.ObjectIDFromHex(categoryIDStr)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid category id")
			return
		}
		matchFilter = append(matchFilter, bson.E{Key: "category_id", Value: categoryObjID})
	}

	// Use aggregation pipeline to join with categories
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: matchFilter}},
		bson.D{{Key: "$lookup", Value: bson.M{
			"from":         "categories",
			"localField":   "category_id",
			"foreignField": "_id",
			"as":           "category",
		}}},
		bson.D{{Key: "$unwind", Value: bson.M{
			"path":                       "$category",
			"preserveNullAndEmptyArrays": true,
		}}},
	}

	cursor, err := collections.Activities.Aggregate(ctx, pipeline)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch activities")
		return
	}
	defer cursor.Close(ctx)

	type activityWithCategory struct {
		model.Activity `bson:",inline"`
		Category       *model.Category `bson:"category,omitempty"`
	}

	var activities []activityWithCategory
	if err := cursor.All(ctx, &activities); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to decode activities")
		return
	}

	result := make([]activityResponse, len(activities))
	for i, act := range activities {
		result[i] = toActivityResponse(act.Activity, act.Category)
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

	// Use aggregation pipeline to join with category
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{
			"_id":     activityObjID,
			"user_id": userObjID,
		}}},
		bson.D{{Key: "$lookup", Value: bson.M{
			"from":         "categories",
			"localField":   "category_id",
			"foreignField": "_id",
			"as":           "category",
		}}},
		bson.D{{Key: "$unwind", Value: bson.M{
			"path":                       "$category",
			"preserveNullAndEmptyArrays": true,
		}}},
	}

	type activityWithCategory struct {
		model.Activity `bson:",inline"`
		Category       *model.Category `bson:"category,omitempty"`
	}

	cursor, err := collections.Activities.Aggregate(ctx, pipeline)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch activity")
		return
	}
	defer cursor.Close(ctx)

	if !cursor.Next(ctx) {
		response.Error(w, http.StatusNotFound, "activity not found")
		return
	}

	var result activityWithCategory
	if err := cursor.Decode(&result); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to decode activity")
		return
	}

	response.Success(w, toActivityResponse(result.Activity, result.Category))
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
		UserID:     userObjID,
		CategoryID: categoryObjID,
		Title:      req.Title,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		CreatedAt:  now,
		UpdatedAt:  now,
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

	// Fetch category for the response
	var category model.Category
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	err = collections.Categories.FindOne(ctx2, bson.M{"_id": categoryObjID}).Decode(&category)
	if err != nil {
		response.Created(w, toActivityResponse(activity, nil))
	} else {
		response.Created(w, toActivityResponse(activity, &category))
	}
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
		"title":      req.Title,
		"updated_at": time.Now().UTC(),
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
func toActivityResponse(act model.Activity, cat *model.Category) activityResponse {
	resp := activityResponse{
		ID:         act.ID.Hex(),
		CategoryID: act.CategoryID.Hex(),
		Title:      act.Title,
		StartTime:  act.StartTime.Format(time.RFC3339),
		CreatedAt:  act.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  act.UpdatedAt.Format(time.RFC3339),
	}
	if act.EndTime != nil {
		end := act.EndTime.Format(time.RFC3339)
		resp.EndTime = &end
		resp.Duration = act.Duration
	}
	if cat != nil {
		catResp := toCategoryResponse(*cat)
		resp.Category = &catResp
	}
	return resp
}
