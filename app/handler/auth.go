package handler

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"

    "golang.org/x/crypto/bcrypt"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    jwt "github.com/golang-jwt/jwt/v5"

    "codeiva/krono-api/pkg/response"
    "codeiva/krono-api/pkg/mail"
)

type userModel struct {
    ID       interface{} `bson:"_id,omitempty" json:"id,omitempty"`
    Name     string      `bson:"name" json:"name"`
    Email    string      `bson:"email" json:"email"`
    Password string      `bson:"password" json:"-"`
    CreatedAt  time.Time   `bson:"created_at" json:"created_at"`
}

type authRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
    Name     string `json:"name,omitempty"`
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
