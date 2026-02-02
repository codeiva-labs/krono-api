package model

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// MainCategories defines the default main categories for the time tracker
var MainCategories = []Category{
	{
		Name:        "Work",
		Description: "Work-related activities and tasks",
		Color:       "#3B82F6", // blue
		Icon:        "💼",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Personal",
		Description: "Personal activities and hobbies",
		Color:       "#10B981", // green
		Icon:        "🏠",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Learning",
		Description: "Education and skill development",
		Color:       "#F59E0B", // amber
		Icon:        "📚",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Health & Fitness",
		Description: "Exercise, sports, and wellness activities",
		Color:       "#EF4444", // red
		Icon:        "💪",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Social",
		Description: "Social activities and entertainment",
		Color:       "#8B5CF6", // purple
		Icon:        "👥",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Rest",
		Description: "Breaks, sleep, and relaxation",
		Color:       "#6B7280", // gray
		Icon:        "😴",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Other",
		Description: "Miscellaneous activities",
		Color:       "#EC4899", // pink
		Icon:        "📋",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
}

// SeedMainCategories inserts the main categories into the database if they don't exist
func SeedMainCategories(ctx context.Context, db *mongo.Database) error {
	collection := db.Collection("categories")

	// Check if main categories already exist
	count, err := collection.CountDocuments(ctx, bson.M{"is_main": true})
	if err != nil {
		return err
	}

	// Skip seeding if main categories already exist
	if count > 0 {
		return nil
	}

	// Prepare documents for insertion
	docs := make([]interface{}, len(MainCategories))
	for i, cat := range MainCategories {
		docs[i] = cat
	}

	// Insert main categories
	_, err = collection.InsertMany(ctx, docs)
	return err
}
