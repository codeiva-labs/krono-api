package handler

import (
	"net/http"

	"go.mongodb.org/mongo-driver/mongo"

	"codeiva/krono-api/pkg/response"
)

// GetHealthStatus is a simple handler to check if the service is running
func GetHealthStatus(db *mongo.Database, w http.ResponseWriter, r *http.Request) {
	response.Success(w, map[string]string{"status": "ok"})
}
