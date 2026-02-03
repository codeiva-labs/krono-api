package app

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"

	"codeiva/krono-api/app/handler"
	"codeiva/krono-api/app/model"
	"codeiva/krono-api/config"
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
	if err := model.SeedMainCategories(context.Background(), a.DB); err != nil {
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
	a.Router.HandleFunc("/auth/request-password-reset", a.handleRequest(handler.RequestPasswordReset)).Methods("POST")
	a.Router.HandleFunc("/auth/reset-password", a.handleRequest(handler.ResetPassword)).Methods("POST")

	// Category routes (protected)
	a.Router.Handle("/categories", handler.AuthMiddleware(a.handleRequest(handler.GetCategories))).Methods("GET")
	a.Router.Handle("/categories/add", handler.AuthMiddleware(a.handleRequest(handler.CreateCategory))).Methods("POST")
	a.Router.Handle("/categories/{category_id}/delete", handler.AuthMiddleware(a.handleRequest(handler.DeleteCategory))).Methods("DELETE")
	a.Router.Handle("/categories/{category_id}/edit", handler.AuthMiddleware(a.handleRequest(handler.EditCategory))).Methods("PUT")

	// Activity routes (protected)
	a.Router.Handle("/activities", handler.AuthMiddleware(a.handleRequest(handler.GetActivities))).Methods("GET")
	a.Router.Handle("/activities/{activity_id}", handler.AuthMiddleware(a.handleRequest(handler.GetActivityDetail))).Methods("GET")
	a.Router.Handle("/activities/add", handler.AuthMiddleware(a.handleRequest(handler.AddActivity))).Methods("POST")
	a.Router.Handle("/activities/{activity_id}/edit", handler.AuthMiddleware(a.handleRequest(handler.EditActivity))).Methods("PUT")
	a.Router.Handle("/activities/{activity_id}/delete", handler.AuthMiddleware(a.handleRequest(handler.DeleteActivity))).Methods("DELETE")
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
