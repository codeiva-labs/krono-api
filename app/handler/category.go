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
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	Icon     string `json:"icon,omitempty"`
	ParentID string `json:"parent_id,omitempty"` // omit to create a main category; set to one of the caller's own main categories to create a subcategory
}

type categoryResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Color     string  `json:"color,omitempty"`
	Icon      string  `json:"icon,omitempty"`
	IsMain    bool    `json:"is_main"`
	ParentID  *string `json:"parent_id,omitempty"`
	UserID    *string `json:"user_id,omitempty"`
	Deleted   bool    `json:"deleted,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// GetCategories returns the authenticated user's own categories (main and sub).
// Soft-deleted categories (Category.Deleted) are intentionally still included, flagged
// via categoryResponse.Deleted, so clients can keep resolving names/colors/icons for
// activities that reference them; only archived (bulk "start fresh" reset) is excluded.
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

	// A user's categories (main and sub) are always fully self-owned
	filter := bson.M{
		"user_id":  userObjID,
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

// CreateCategory creates a new main category (no parent_id) or subcategory (parent_id set) for the authenticated user
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

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}

	now := time.Now().UTC()
	category := model.Category{
		Name:      req.Name,
		Color:     req.Color,
		Icon:      req.Icon,
		UserID:    &userObjID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if req.ParentID == "" {
		// No parent: this is a new main category owned by the caller
		category.IsMain = true
		category.ParentID = nil
	} else {
		parentObjID, err := primitive.ObjectIDFromHex(req.ParentID)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid parent_id")
			return
		}

		// Parent must be a main category owned by the caller
		var parent model.Category
		err = collections.Categories.FindOne(ctx, bson.M{"_id": parentObjID, "is_main": true, "user_id": userObjID, "deleted": bson.M{"$ne": true}}).Decode(&parent)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				response.Error(w, http.StatusBadRequest, "parent_id must reference one of your own main categories")
				return
			}
			response.Error(w, http.StatusInternalServerError, "failed to verify parent category")
			return
		}

		category.IsMain = false
		category.ParentID = &parentObjID
	}

	result, err := collections.Categories.InsertOne(ctx, category)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create category")
		return
	}

	category.ID = result.InsertedID.(primitive.ObjectID)
	response.Created(w, toCategoryResponse(category))
}

// DeleteCategory removes a category owned by the user, as long as it has no subcategories
// (if main). If the category is referenced by any activity it's soft-deleted (flagged
// "deleted" but kept) instead of removed, so those activities keep resolving its
// name/color/icon; otherwise it's removed outright.
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
	err = collections.Categories.FindOne(ctx, bson.M{"_id": categoryObjID, "user_id": userObjID, "archived": bson.M{"$ne": true}, "deleted": bson.M{"$ne": true}}).Decode(&category)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			response.Error(w, http.StatusNotFound, "category not found or not owned by you")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to find category")
		return
	}

	// If this is a main category, it can't be deleted while subcategories still reference it
	if category.IsMain {
		subCount, err := collections.Categories.CountDocuments(ctx, bson.M{"parent_id": categoryObjID})
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to check subcategory usage")
			return
		}
		if subCount > 0 {
			response.Error(w, http.StatusConflict, "cannot delete category that has subcategories")
			return
		}
	}

	// Check if category is used in any activity
	count, err := collections.Activities.CountDocuments(ctx, bson.M{"category_id": categoryObjID})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to check category usage")
		return
	}

	if count > 0 {
		// In use - soft-delete instead of removing, so those activities keep
		// resolving this category's name/color/icon. Hidden from creation/edit
		// (the "deleted" exclusion above) but still returned by GetCategories.
		now := time.Now().UTC()
		_, err = collections.Categories.UpdateOne(ctx,
			bson.M{"_id": categoryObjID},
			bson.M{"$set": bson.M{"deleted": true, "deleted_at": now, "updated_at": now}},
		)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to delete category")
			return
		}
		response.Success(w, map[string]string{"status": "deleted"})
		return
	}

	// Never used - safe to remove outright
	_, err = collections.Categories.DeleteOne(ctx, bson.M{"_id": categoryObjID})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to delete category")
		return
	}

	response.Success(w, map[string]string{"status": "deleted"})
}

// EditCategory updates a user's category (name/color/icon), main or sub
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
	err = collections.Categories.FindOne(ctx, bson.M{"_id": categoryObjID, "user_id": userObjID, "archived": bson.M{"$ne": true}, "deleted": bson.M{"$ne": true}}).Decode(&category)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			response.Error(w, http.StatusNotFound, "category not found or not owned by you")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to find category")
		return
	}

	// Build update document with only provided fields
	set := bson.M{"updated_at": time.Now().UTC()}
	if req.Name != "" {
		set["name"] = req.Name
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
		ID:        cat.ID.Hex(),
		Name:      cat.Name,
		Color:     cat.Color,
		Icon:      cat.Icon,
		IsMain:    cat.IsMain,
		Deleted:   cat.Deleted,
		CreatedAt: cat.CreatedAt.Format(time.RFC3339),
		UpdatedAt: cat.UpdatedAt.Format(time.RFC3339),
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
