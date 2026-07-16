package app

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"codeiva/krono-api/app/db"
	"codeiva/krono-api/app/handler"
	"codeiva/krono-api/app/model"
	"codeiva/krono-api/config"
	"codeiva/krono-api/pkg/response"
)

// App has router and db instances
type App struct {
	Router      *mux.Router
	DB          *mongo.Database
	Collections *model.Collections
	client      *mongo.Client
}

// Initialize initializes the app with predefined configuration (MongoDB)
func (a *App) Initialize(cfg *config.Config) {
	client, err := cfg.ConnectMongo(context.Background())
	if err != nil {
		log.Fatalf("Could not connect to MongoDB: %v", err)
	}

	a.client = client
	a.DB = cfg.Database(client)
	log.Printf("connected to MongoDB: %s (database=%s)", cfg.DB.MongoURI, cfg.DB.Database)

	// Initialize collections
	a.Collections = model.NewCollections(a.DB)

	// Ensure indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Collections.EnsureIndexes(ctx); err != nil {
		log.Fatalf("Could not create indexes: %v", err)
	}
	log.Println("Database indexes created successfully")

	// Seed main categories
	if err := db.SeedMainCategories(context.Background(), a.DB); err != nil {
		log.Fatalf("Could not seed main categories: %v", err)
	}
	log.Println("Main categories seeded successfully")

	a.Router = mux.NewRouter()
	a.setRouters()
}

// setRouters sets the all required routers
func (a *App) setRouters() {
	a.Get("/health", a.handleRequest(handler.GetHealthStatus))

	a.Router.Use(a.loggingMiddleware)

	// Auth routes (public)
	a.Router.HandleFunc("/auth/register", a.handleRequest(handler.Register)).Methods("POST")
	a.Router.HandleFunc("/auth/login", a.handleRequest(handler.Login)).Methods("POST")
	a.Router.HandleFunc("/auth/authenticate-google", a.handleRequest(handler.AuthenticateWithGoogle)).Methods("POST")
	a.Router.HandleFunc("/auth/request-password-reset", a.handleRequest(handler.RequestPasswordReset)).Methods("POST")
	a.Router.HandleFunc("/auth/reset-password", a.handleRequest(handler.ResetPassword)).Methods("POST")

	// Profile routes (protected)
	a.Router.Handle("/profile", a.Protected(handler.GetProfile)).Methods("GET")
	a.Router.Handle("/profile", a.Protected(handler.UpdateProfile)).Methods("PUT")

	// Account routes (protected)
	a.Router.Handle("/account/data-decision", a.Protected(handler.SetDataDecision)).Methods("POST")

	// Category routes (protected)
	a.Router.Handle("/categories", a.Protected(handler.GetCategories)).Methods("GET")
	a.Router.Handle("/categories/add", a.Protected(handler.CreateCategory)).Methods("POST")
	a.Router.Handle("/categories/{category_id}/delete", a.Protected(handler.DeleteCategory)).Methods("DELETE")
	a.Router.Handle("/categories/{category_id}/edit", a.Protected(handler.EditCategory)).Methods("PUT")

	// Activity routes (protected)
	a.Router.Handle("/activities", a.Protected(handler.GetActivities)).Methods("GET")
	a.Router.Handle("/activities/{activity_id}", a.Protected(handler.GetActivityDetail)).Methods("GET")
	a.Router.Handle("/activities/add", a.Protected(handler.AddActivity)).Methods("POST")
	a.Router.Handle("/activities/{activity_id}/edit", a.Protected(handler.EditActivity)).Methods("PUT")
	a.Router.Handle("/activities/{activity_id}/delete", a.Protected(handler.DeleteActivity)).Methods("DELETE")

	// Statistics routes (protected)
	a.Router.Handle("/stats/daily", a.Protected(handler.GetDailyStats)).Methods("GET")
	a.Router.Handle("/stats/most-used-categories", a.Protected(handler.GetMostUsedCategories)).Methods("GET")
}

// statusRecorder wraps http.ResponseWriter to capture the response status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware logs method, path, status and duration for each request.
func (a *App) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sr, r)
		dur := time.Since(start)
		log.Printf("%s %s %d %s", r.Method, r.RequestURI, sr.status, dur)
	})
}

// Get wraps the router for GET method
func (a *App) Get(path string, f func(w http.ResponseWriter, r *http.Request)) {
	a.Router.HandleFunc(path, f).Methods("GET")
}

// Post wraps the router for POST method
func (a *App) Post(path string, f func(w http.ResponseWriter, r *http.Request)) {
	a.Router.HandleFunc(path, f).Methods("POST")
}

// Put wraps the router for PUT method
func (a *App) Put(path string, f func(w http.ResponseWriter, r *http.Request)) {
	a.Router.HandleFunc(path, f).Methods("PUT")
}

// Delete wraps the router for DELETE method
func (a *App) Delete(path string, f func(w http.ResponseWriter, r *http.Request)) {
	a.Router.HandleFunc(path, f).Methods("DELETE")
}

// Run the app on it's router
func (a *App) Run(host string) {
	log.Fatal(http.ListenAndServe(host, a.Router))
}

type RequestHandlerFunction func(collections *model.Collections, w http.ResponseWriter, r *http.Request)

func (a *App) handleRequest(handler RequestHandlerFunction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(a.Collections, w, r)
	}
}

// requireActiveSession rejects requests whose token session id no longer
// matches the user's current session id in the database. A login elsewhere
// rotates that session id, which is how a login on a new device forces out
// any previously logged-in device.
func (a *App) requireActiveSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, _ := r.Context().Value("user_id").(string)
		sessionID, _ := r.Context().Value("session_id").(string)

		userObjID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			response.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var u model.User
		if err := a.Collections.Users.FindOne(ctx, bson.M{"_id": userObjID}).Decode(&u); err != nil {
			response.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if sessionID == "" || u.CurrentSessionID == "" || sessionID != u.CurrentSessionID {
			response.Error(w, http.StatusUnauthorized, "session ended: logged in on another device")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Protected wraps a handler with JWT auth and active-session validation.
func (a *App) Protected(f RequestHandlerFunction) http.Handler {
	return handler.AuthMiddleware(a.requireActiveSession(a.handleRequest(f)))
}
