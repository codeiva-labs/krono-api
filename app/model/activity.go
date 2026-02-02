package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Activity represents a time tracking activity
type Activity struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	UserID      primitive.ObjectID `bson:"user_id" json:"user_id"`
	CategoryID  primitive.ObjectID `bson:"category_id" json:"category_id"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	StartTime   time.Time          `bson:"start_time" json:"start_time"`
	EndTime     *time.Time         `bson:"end_time,omitempty" json:"end_time,omitempty"` // nil if activity is ongoing
	Duration    int64              `bson:"duration,omitempty" json:"duration,omitempty"` // in seconds, calculated when ended
	Tags        []string           `bson:"tags,omitempty" json:"tags,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}
