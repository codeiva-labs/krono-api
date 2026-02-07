package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ProviderLink represents a linked authentication provider
type ProviderLink struct {
	Provider   string `bson:"provider" json:"provider"`       // "google", "apple", "email", etc.
	ProviderID string `bson:"provider_id" json:"provider_id"` // ID from OAuth provider
}

// User represents a user in the system
type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Email     string             `bson:"email" json:"email"`
	Password  string             `bson:"password" json:"-"` // never expose in JSON
	Name      string             `bson:"name" json:"name"`
	Providers []ProviderLink     `bson:"providers,omitempty" json:"providers,omitempty"` // Multiple linked providers
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}
