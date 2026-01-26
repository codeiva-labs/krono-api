package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"

	"codeiva/krono-api/pkg/mail"
	"codeiva/krono-api/pkg/response"
)

type userModel struct {
	ID        interface{} `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string      `bson:"name" json:"name"`
	Email     string      `bson:"email" json:"email"`
	Password  string      `bson:"password" json:"-"`
	CreatedAt time.Time   `bson:"created_at" json:"created_at"`
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name,omitempty"`
}

type passwordResetModel struct {
	ID        interface{} `bson:"_id,omitempty" json:"id,omitempty"`
	Email     string      `bson:"email" json:"email"`
	OTP       string      `bson:"otp" json:"otp"`
	CreatedAt time.Time   `bson:"created_at" json:"created_at"`
}

type passwordResetRequest struct {
	Email string `json:"email"`
}

type passwordResetConfirm struct {
	Email    string `json:"email"`
	OTP      string `json:"otp"`
	Password string `json:"password"`
}

// ensureUsersIndex creates a unique index on email
func ensureUsersIndex(ctx context.Context, col *mongo.Collection) error {
	idxModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err := col.Indexes().CreateOne(ctx, idxModel)
	return err
}

// Register creates a new user with hashed password
func Register(db *mongo.Database, w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		response.Error(w, http.StatusBadRequest, "name, email and password required")
		return
	}

	col := db.Collection("users")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// ensure unique index
	_ = ensureUsersIndex(ctx, col)

	// hash password
	pw, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	u := userModel{
		Name:      req.Name,
		Email:     req.Email,
		Password:  string(pw),
		CreatedAt: time.Now().UTC(),
	}

	_, err = col.InsertOne(ctx, u)
	if err != nil {
		// if duplicate key
		if mongo.IsDuplicateKeyError(err) {
			response.Error(w, http.StatusConflict, "email already registered")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	response.Created(w, map[string]string{"email": u.Email})
}

// Login validates credentials and returns a JWT token
func Login(db *mongo.Database, w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Email == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "email and password required")
		return
	}

	col := db.Collection("users")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var u userModel
	if err := col.FindOne(ctx, bson.M{"email": req.Email}).Decode(&u); err != nil {
		if err == mongo.ErrNoDocuments {
			response.Error(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		response.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		response.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// sign JWT
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret"
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": u.Email,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	// send email for login notification (optional)

	// send email for login notification (optional)
	go func(email string) {
		log.Printf("sending login notification email to %s", email)
		m := mail.NewMailerFromEnv()
		if m.Host == "" || m.Username == "" {
			log.Printf("mailer config: host=%s, port=%d, user=%s", m.Host, m.Port, m.Username)
			log.Printf("smtp not configured, skipping login notification email to %s", email)
			return
		}

		subject := "New login to your account"
		body := fmt.Sprintf("<p>Hi %s,</p><p>We detected a login to your account at %s from our service.</p>", u.Name, time.Now().Format(time.RFC1123))
		if err := m.SendSimple(email, subject, body); err != nil {
			log.Printf("failed to send login notification to %s: %v", email, err)
		}
		log.Printf("sent login notification email to %s", email)
	}(u.Email)

	response.Success(w, map[string]string{"token": signed})
}

// RequestPasswordReset generates an OTP and emails it to the user.
// Previous OTPs remain valid until expiry (1 hour).
func RequestPasswordReset(db *mongo.Database, w http.ResponseWriter, r *http.Request) {
	var req passwordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Email == "" {
		response.Error(w, http.StatusBadRequest, "email required")
		return
	}

	colUsers := db.Collection("users")
	colReset := db.Collection("password_resets")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var u userModel
	if err := colUsers.FindOne(ctx, bson.M{"email": req.Email}).Decode(&u); err != nil {
		// Do not reveal whether the email exists. Respond success either way.
		response.Success(w, map[string]string{"status": "ok"})
		return
	}

	otp, err := generateOTP6()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to generate otp")
		return
	}

	pr := passwordResetModel{
		Email:     req.Email,
		OTP:       otp,
		CreatedAt: time.Now().UTC(),
	}
	if _, err := colReset.InsertOne(ctx, pr); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to save otp")
		return
	}

	go func(email, name, otp string) {
		m := mail.NewMailerFromEnv()
		if m.Host == "" || m.Username == "" {
			log.Printf("smtp not configured, skipping password reset email to %s", email)
			return
		}
		subject := "Your password reset code"
		body := fmt.Sprintf("<p>Hi %s,</p><p>Your password reset code is: <strong>%s</strong></p><p>This code is valid for 1 hour.</p>", name, otp)
		if err := m.SendSimple(email, subject, body); err != nil {
			log.Printf("failed to send password reset to %s: %v", email, err)
		}
	}(u.Email, u.Name, otp)

	response.Success(w, map[string]string{"status": "ok"})
}

// ResetPassword verifies OTP and changes the user's password. On success, invalidate all OTPs for the email.
func ResetPassword(db *mongo.Database, w http.ResponseWriter, r *http.Request) {
	var req passwordResetConfirm
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Email == "" || req.OTP == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "email, otp and password required")
		return
	}

	colUsers := db.Collection("users")
	colReset := db.Collection("password_resets")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	cutoff := time.Now().UTC().Add(-1 * time.Hour)
	var pr passwordResetModel
	err := colReset.FindOne(ctx, bson.M{"email": req.Email, "otp": req.OTP, "created_at": bson.M{"$gte": cutoff}}).Decode(&pr)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			response.Error(w, http.StatusBadRequest, "invalid or expired otp")
			return
		}
		response.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	pw, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if _, err := colUsers.UpdateOne(ctx, bson.M{"email": req.Email}, bson.M{"$set": bson.M{"password": string(pw)}}); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	if _, err := colReset.DeleteMany(ctx, bson.M{"email": req.Email}); err != nil {
		log.Printf("warning: failed to delete password resets for %s: %v", req.Email, err)
	}

	response.Success(w, map[string]string{"status": "ok"})
}

// generateOTP6 returns a 6-digit numeric OTP using crypto/rand
func generateOTP6() (string, error) {
	var out strings.Builder
	for i := 0; i < 6; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		out.WriteString(fmt.Sprintf("%d", n.Int64()))
	}
	return out.String(), nil
}
