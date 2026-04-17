package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"codeiva/krono-api/app/model"
)

// MainCategories defines the default main categories for the time tracker
var MainCategories = []model.Category{
	{
		Name:        "Sleep",
		Description: "Sleep and rest",
		Color:       "#6366F1", // indigo
		Icon:        "😴",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Eat & Drink",
		Description: "Meals and beverages",
		Color:       "#F97316", // orange
		Icon:        "🍽️",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Sport & Exercise",
		Description: "Physical activities and sports",
		Color:       "#EF4444", // red
		Icon:        "💪",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Self Care",
		Description: "Personal care and wellness",
		Color:       "#EC4899", // pink
		Icon:        "💆",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
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
		Name:        "Education",
		Description: "Learning and skill development",
		Color:       "#F59E0B", // amber
		Icon:        "📚",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Cooking",
		Description: "Preparing meals and food",
		Color:       "#FB923C", // orange
		Icon:        "🍳",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Cleaning",
		Description: "Household cleaning and tidying",
		Color:       "#06B6D4", // cyan
		Icon:        "🧹",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Home Maintenance",
		Description: "Repairs and home improvement",
		Color:       "#78716C", // stone
		Icon:        "🔧",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Shopping",
		Description: "Shopping for goods and services",
		Color:       "#10B981", // green
		Icon:        "🛒",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Gardening",
		Description: "Garden work and plant care",
		Color:       "#22C55E", // green
		Icon:        "🌱",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Petcare",
		Description: "Taking care of pets",
		Color:       "#92400E", // brown
		Icon:        "🐾",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Family Time",
		Description: "Spending time with family",
		Color:       "#D946EF", // fuchsia
		Icon:        "👨‍👩‍👧‍👦",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Religion",
		Description: "Religious and spiritual activities",
		Color:       "#8B5CF6", // purple
		Icon:        "🙏",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Voluntary Work",
		Description: "Volunteer and community service",
		Color:       "#14B8A6", // teal
		Icon:        "🤝",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Commute / Travel",
		Description: "Traveling and commuting",
		Color:       "#0EA5E9", // sky blue
		Icon:        "🚗",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Social Media/ TV/ Radio",
		Description: "Media consumption and entertainment",
		Color:       "#A855F7", // purple
		Icon:        "📺",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Read",
		Description: "Reading books, articles, and more",
		Color:       "#D97706", // amber
		Icon:        "📖",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Gaming",
		Description: "Playing games and gaming",
		Color:       "#84CC16", // lime
		Icon:        "🎮",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Holiday",
		Description: "Vacation and leisure time",
		Color:       "#FBBF24", // yellow
		Icon:        "🏖️",
		IsMain:      true,
		ParentID:    nil,
		UserID:      nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	},
	{
		Name:        "Others",
		Description: "Miscellaneous activities",
		Color:       "#6B7280", // gray
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
