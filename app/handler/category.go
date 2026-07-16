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

type createCategoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
	Icon        string `json:"icon,omitempty"`
	ParentID    string `json:"parent_id"` // required - must reference a main category
}

type categoryResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Color       string  `json:"color,omitempty"`
	Icon        string  `json:"icon,omitempty"`
	IsMain      bool    `json:"is_main"`
	ParentID    *string `json:"parent_id,omitempty"`
	UserID      *string `json:"user_id,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// GetCategories returns all main categories plus the authenticated user's custom categories
func GetCategories(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Find all main categories OR categories created by this user
	filter := bson.M{
		"$or": []bson.M{
			{"is_main": true},
			{"user_id": userObjID},
		},
		"archived": bson.M{"$ne": true},
	}

	cursor, err := collections.Categories.Find(ctx, filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch categories")
		return
	}
	defer cursor.Close(ctx)

	var categories []model.Category
	if err := cursor.All(ctx, &categories); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to decode categories")
		return
	}

	// Convert to response format
	result := make([]categoryResponse, len(categories))
	for i, cat := range categories {
		result[i] = toCategoryResponse(cat)
	}

	response.Success(w, result)
}

// CreateCategory creates a new custom category for the authenticated user
func CreateCategory(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Name == "" {
		response.Error(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.ParentID == "" {
		response.Error(w, http.StatusBadRequest, "parent_id is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}

	parentObjID, err := primitive.ObjectIDFromHex(req.ParentID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid parent_id")
		return
	}

	// Verify parent category exists and is a main category
	var parent model.Category
	err = collections.Categories.FindOne(ctx, bson.M{"_id": parentObjID, "is_main": true}).Decode(&parent)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			response.Error(w, http.StatusBadRequest, "parent category must be a main category")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to verify parent category")
		return
	}

	// Create new category
	now := time.Now().UTC()
	category := model.Category{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		Icon:        req.Icon,
		IsMain:      false,
		ParentID:    &parentObjID,
		UserID:      &userObjID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	result, err := collections.Categories.InsertOne(ctx, category)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create category")
		return
	}

	category.ID = result.InsertedID.(primitive.ObjectID)
	response.Created(w, toCategoryResponse(category))
}

// DeleteCategory deletes a custom category if it's owned by the user and not used in any activity
func DeleteCategory(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	vars := mux.Vars(r)
	categoryID := vars["category_id"]
	if categoryID == "" {
		response.Error(w, http.StatusBadRequest, "category id is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}

	categoryObjID, err := primitive.ObjectIDFromHex(categoryID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid category id")
		return
	}

	// Verify category exists and is owned by the user
	var category model.Category
	err = collections.Categories.FindOne(ctx, bson.M{"_id": categoryObjID, "user_id": userObjID, "archived": bson.M{"$ne": true}}).Decode(&category)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			response.Error(w, http.StatusNotFound, "category not found or not owned by you")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to find category")
		return
	}

	// Check if category is a main category (cannot delete main categories)
	if category.IsMain {
		response.Error(w, http.StatusForbidden, "cannot delete main category")
		return
	}

	// Check if category is used in any activity
	count, err := collections.Activities.CountDocuments(ctx, bson.M{"category_id": categoryObjID})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to check category usage")
		return
	}

	if count > 0 {
		response.Error(w, http.StatusConflict, "cannot delete category that is used in activities")
		return
	}

	// Delete the category
	_, err = collections.Categories.DeleteOne(ctx, bson.M{"_id": categoryObjID})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to delete category")
		return
	}

	response.Success(w, map[string]string{"status": "deleted"})
}

// EditCategory updates a user's custom category (name/description/color/icon)
func EditCategory(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	vars := mux.Vars(r)
	categoryID := vars["category_id"]
	if categoryID == "" {
		response.Error(w, http.StatusBadRequest, "category id is required")
		return
	}

	var req createCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}

	categoryObjID, err := primitive.ObjectIDFromHex(categoryID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid category id")
		return
	}

	// Verify category exists and is owned by the user
	var category model.Category
	err = collections.Categories.FindOne(ctx, bson.M{"_id": categoryObjID, "user_id": userObjID, "archived": bson.M{"$ne": true}}).Decode(&category)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			response.Error(w, http.StatusNotFound, "category not found or not owned by you")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to find category")
		return
	}

	if category.IsMain {
		response.Error(w, http.StatusForbidden, "cannot edit main category")
		return
	}

	// Build update document with only provided fields
	set := bson.M{"updated_at": time.Now().UTC()}
	if req.Name != "" {
		set["name"] = req.Name
	}
	if req.Description != "" {
		set["description"] = req.Description
	}
	if req.Color != "" {
		set["color"] = req.Color
	}
	if req.Icon != "" {
		set["icon"] = req.Icon
	}

	if len(set) == 1 { // only updated_at
		response.Error(w, http.StatusBadRequest, "no fields to update")
		return
	}

	_, err = collections.Categories.UpdateOne(ctx, bson.M{"_id": categoryObjID, "user_id": userObjID}, bson.M{"$set": set})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update category")
		return
	}

	// Return updated category
	err = collections.Categories.FindOne(ctx, bson.M{"_id": categoryObjID}).Decode(&category)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch updated category")
		return
	}

	response.Success(w, toCategoryResponse(category))
}

// toCategoryResponse converts a model.Category to categoryResponse
func toCategoryResponse(cat model.Category) categoryResponse {
	resp := categoryResponse{
		ID:          cat.ID.Hex(),
		Name:        cat.Name,
		Description: cat.Description,
		Color:       cat.Color,
		Icon:        cat.Icon,
		IsMain:      cat.IsMain,
		CreatedAt:   cat.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   cat.UpdatedAt.Format(time.RFC3339),
	}

	if cat.ParentID != nil {
		parentID := cat.ParentID.Hex()
		resp.ParentID = &parentID
	}

	if cat.UserID != nil {
		userID := cat.UserID.Hex()
		resp.UserID = &userID
	}

	return resp
}
