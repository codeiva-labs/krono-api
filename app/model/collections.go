package model

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Collections provides centralized access to all MongoDB collections
type Collections struct {
	Users          *mongo.Collection
	Activities     *mongo.Collection
	Categories     *mongo.Collection
	PasswordResets *mongo.Collection
}

// NewCollections creates a new Collections instance with all collection references
func NewCollections(db *mongo.Database) *Collections {
	return &Collections{
		Users:          db.Collection("users"),
		Activities:     db.Collection("activities"),
		Categories:     db.Collection("categories"),
		PasswordResets: db.Collection("password_resets"),
	}
}

// EnsureIndexes creates all necessary indexes for the collections
func (c *Collections) EnsureIndexes(ctx context.Context) error {
	// Users: unique email index
	_, err := c.Users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Activities: index on user_id and start_time for efficient queries
	_, err = c.Activities.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "start_time", Value: -1},
		},
	})
	if err != nil {
		return err
	}

	// Activities: index on category_id for filtering by category
	_, err = c.Activities.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "category_id", Value: 1}},
	})
	if err != nil {
		return err
	}

	// Categories: index on parent_id for subcategory queries
	_, err = c.Categories.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "parent_id", Value: 1}},
	})
	if err != nil {
		return err
	}

	// Categories: compound index on user_id and parent_id for custom categories
	_, err = c.Categories.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "parent_id", Value: 1},
		},
	})
	if err != nil {
		return err
	}

	// PasswordResets: index on email and created_at for OTP validation
	_, err = c.PasswordResets.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "email", Value: 1},
			{Key: "created_at", Value: -1},
		},
	})
	if err != nil {
		return err
	}

	// PasswordResets: TTL index to auto-delete expired resets after 1 hour
	_, err = c.PasswordResets.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(3600), // 1 hour
	})
	if err != nil {
		return err
	}

	return nil
}
