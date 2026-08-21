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

type userResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type updateProfileRequest struct {
	Name string `json:"name"`
}

// GetProfile returns the authenticated user's profile
func GetProfile(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
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

	var u model.User
	if err := collections.Users.FindOne(ctx, bson.M{"_id": userObjID}).Decode(&u); err != nil {
		response.Error(w, http.StatusNotFound, "user not found")
		return
	}

	response.Success(w, toUserResponse(u))
}

// UpdateProfile lets the authenticated user change their name
func UpdateProfile(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
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

	var req updateProfileRequest
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

	_, err = collections.Users.UpdateOne(ctx,
		bson.M{"_id": userObjID},
		bson.M{"$set": bson.M{"name": req.Name, "updated_at": time.Now().UTC()}},
	)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	var u model.User
	if err := collections.Users.FindOne(ctx, bson.M{"_id": userObjID}).Decode(&u); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch updated profile")
		return
	}

	response.Success(w, toUserResponse(u))
}

func toUserResponse(u model.User) userResponse {
	return userResponse{
		ID:        u.ID.Hex(),
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
		UpdatedAt: u.UpdatedAt.Format(time.RFC3339),
	}
}
