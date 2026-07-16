package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Category represents a category for activities
type Category struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string              `bson:"name" json:"name"`
	Description string              `bson:"description,omitempty" json:"description,omitempty"`
	Color       string              `bson:"color,omitempty" json:"color,omitempty"`         // hex color code for UI
	Icon        string              `bson:"icon,omitempty" json:"icon,omitempty"`           // icon name or emoji
	IsMain      bool                `bson:"is_main" json:"is_main"`                         // true for main categories
	ParentID    *primitive.ObjectID `bson:"parent_id,omitempty" json:"parent_id,omitempty"` // nil for main categories
	UserID      *primitive.ObjectID `bson:"user_id,omitempty" json:"user_id,omitempty"`     // nil for main categories, set for custom
	// Archived is set on a user's custom category when they choose "start
	// fresh" on a new device; main categories are never archived.
	Archived   bool       `bson:"archived,omitempty" json:"-"`
	ArchivedAt *time.Time `bson:"archived_at,omitempty" json:"-"`
	CreatedAt  time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `bson:"updated_at" json:"updated_at"`
}
