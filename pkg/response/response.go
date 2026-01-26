package response

import (
    "encoding/json"
    "net/http"
)

type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// JSON writes a generic response with explicit HTTP status, code and message.
func JSON(w http.ResponseWriter, httpStatus int, code int, message string, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(httpStatus)
    _ = json.NewEncoder(w).Encode(Response{Code: code, Message: message, Data: data})
}

// Success responds with 200 OK and message "ok".
func Success(w http.ResponseWriter, data interface{}) {
    JSON(w, http.StatusOK, http.StatusOK, "ok", data)
}

// Created responds with 201 Created.
func Created(w http.ResponseWriter, data interface{}) {
    JSON(w, http.StatusCreated, http.StatusCreated, "created", data)
}

// Error responds with the provided HTTP status and message.
func Error(w http.ResponseWriter, httpStatus int, message string) {
    JSON(w, httpStatus, httpStatus, message, nil)
}
