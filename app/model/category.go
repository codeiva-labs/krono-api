package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Category represents a category for activities
type Category struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string              `bson:"name" json:"name"`
	Color       string              `bson:"color,omitempty" json:"color,omitempty"`         // hex color code for UI
	Icon        string              `bson:"icon,omitempty" json:"icon,omitempty"`           // icon name or emoji
	IsMain      bool                `bson:"is_main" json:"is_main"`                         // true for a user's top-level category, false for a subcategory
	ParentID    *primitive.ObjectID `bson:"parent_id,omitempty" json:"parent_id,omitempty"` // nil for main categories, set for subcategories
	UserID      *primitive.ObjectID `bson:"user_id,omitempty" json:"user_id,omitempty"`     // owning user; always set except on legacy pre-migration global categories
	// Archived is set on a user's category when they choose "start
	// fresh" on a new device.
	Archived   bool       `bson:"archived,omitempty" json:"-"`
	ArchivedAt *time.Time `bson:"archived_at,omitempty" json:"-"`
	// Deleted is set when the owner deletes a category that still has
	// activities referencing it: the category is hidden from creation/edit
	// but - unlike Archived - still returned by GetCategories (flagged) so
	// those activities can keep resolving its name/color/icon.
	Deleted    bool       `bson:"deleted,omitempty" json:"deleted,omitempty"`
	DeletedAt  *time.Time `bson:"deleted_at,omitempty" json:"-"`
	CreatedAt  time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `bson:"updated_at" json:"updated_at"`
}
