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
	// CurrentSessionID is embedded in the JWT issued at the most recent login.
	// A request whose token carries a different (or missing) session id is
	// treated as logged out, which is how logging in on a new device forces
	// out any previously logged-in device.
	CurrentSessionID string `bson:"current_session_id,omitempty" json:"-"`
	// LastDeviceID is the device_id supplied at the most recent login/auth,
	// used to detect when a login comes from a device we haven't seen before.
	LastDeviceID string    `bson:"last_device_id,omitempty" json:"-"`
	CreatedAt    time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updated_at"`
}
