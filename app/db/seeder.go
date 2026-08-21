package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"codeiva/krono-api/app/model"
)

// MainCategories defines the default starting categories given to every user
var MainCategories = []model.Category{
	{
		Name:      "Sleep",
		Color:     "#C7D2FE",
		Icon:      "😴",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Eat & Drink",
		Color:     "#FED7AA",
		Icon:      "🍽️",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Sport & Exercise",
		Color:     "#FECACA",
		Icon:      "🏃",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Self Care",
		Color:     "#E9D5FF",
		Icon:      "💆",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Work",
		Color:     "#BFDBFE",
		Icon:      "💼",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Education",
		Color:     "#FDE68A",
		Icon:      "📚",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Cooking",
		Color:     "#FFE4C7",
		Icon:      "🍳",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Cleaning",
		Color:     "#A5F3FC",
		Icon:      "🧹",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Home Maintenance",
		Color:     "#E7E5E4",
		Icon:      "🔧",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Shopping",
		Color:     "#A7F3D0",
		Icon:      "🛒",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Gardening",
		Color:     "#FEF3C7",
		Icon:      "🌱",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Petcare",
		Color:     "#FDE1C3",
		Icon:      "🐾",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Family Time",
		Color:     "#FBCFE8",
		Icon:      "👨‍👩‍👧‍👦",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Religion",
		Color:     "#DDD6FE",
		Icon:      "🙏",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Voluntary Work",
		Color:     "#99F6E4",
		Icon:      "🤝",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Commute / Travel",
		Color:     "#BAE6FD",
		Icon:      "🚗",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Social Media / TV / Radio",
		Color:     "#F5D0FE",
		Icon:      "📺",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Read",
		Color:     "#FED7AA",
		Icon:      "📖",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Gaming",
		Color:     "#D9F99D",
		Icon:      "🎮",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Holiday",
		Color:     "#FDE68A",
		Icon:      "🏖️",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
	{
		Name:      "Others",
		Color:     "#E5E7EB",
		Icon:      "📋",
		IsMain:    true,
		ParentID:  nil,
		UserID:    nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	},
}

// SeedUserDefaultCategories gives a newly created user their own owned copy
// of the default starting categories. Unlike the old global seed, these are
// fully owned by the user (UserID set) so they can be freely edited or
// deleted, same as any category the user creates themselves.
func SeedUserDefaultCategories(ctx context.Context, categories *mongo.Collection, userID primitive.ObjectID) error {
	now := time.Now().UTC()
	docs := make([]interface{}, len(MainCategories))
	for i, cat := range MainCategories {
		c := cat
		c.ID = primitive.NilObjectID
		c.UserID = &userID
		c.CreatedAt = now
		c.UpdatedAt = now
		docs[i] = c
	}

	_, err := categories.InsertMany(ctx, docs)
	return err
}
