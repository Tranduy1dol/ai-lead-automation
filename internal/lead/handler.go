package lead

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

const maxRequestBytes = 1 << 20

type Creator interface {
	Create(context.Context, CreateParams) (Lead, error)
}

type Handler struct {
	creator Creator
	logger  *slog.Logger
}

func NewHandler(creator Creator, logger *slog.Logger) *Handler {
	return &Handler{creator: creator, logger: logger}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request createRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "request body must contain one JSON object")
		return
	}

	request.normalize()
	if message := request.validate(); message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}

	created, err := h.creator.Create(r.Context(), CreateParams{
		Name:    request.Name,
		Email:   request.Email,
		Company: request.Company,
		Message: request.Message,
	})
	if err != nil {
		h.logger.Error("create lead failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, newResponse(created))
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
