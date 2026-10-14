package httpx

import (
    "encoding/json"
    "log"
    "net/http"
)

type ErrorBody struct {
    Error   string `json:"error"`
    Message string `json:"message,omitempty"`
}

func JSON(w http.ResponseWriter, status int, body any) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(status)
    if body != nil {
        if err := json.NewEncoder(w).Encode(body); err != nil {
            log.Printf("httpx: encode: %v", err)
        }
    }
}

func Error(w http.ResponseWriter, status int, code, msg string) {
    JSON(w, status, ErrorBody{Error: code, Message: msg})
}