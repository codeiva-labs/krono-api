package model

import "time"

// PasswordReset represents a password reset request with OTP
type PasswordReset struct {
	Email     string    `bson:"email" json:"email"`
	OTP       string    `bson:"otp" json:"otp"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}
