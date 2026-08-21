package handler

import (
	"net/http"

	"codeiva/krono-api/app/model"
	"codeiva/krono-api/pkg/response"
)

// GetHealthStatus is a simple handler to check if the service is running
func GetHealthStatus(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	response.Success(w, map[string]string{"status": "ok"})
}
