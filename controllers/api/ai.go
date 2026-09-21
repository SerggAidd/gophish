package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gophish/gophish/ai"
	"github.com/gophish/gophish/models"
)

// GenerateAITemplate creates a new training email using the configured AI generator.
func (as *Server) GenerateAITemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Method not allowed",
		}, http.StatusMethodNotAllowed)
		return
	}

	var req ai.GenerationRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Invalid request body",
		}, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.TargetAudience) == "" {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Target audience is required",
		}, http.StatusBadRequest)
		return
	}

	if req.Scenario == "custom" && strings.TrimSpace(req.CustomScenario) == "" {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Custom scenario description is required",
		}, http.StatusBadRequest)
		return
	}

	email, err := as.aiGenerator.Generate(r.Context(), req)
	if err != nil {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Failed to generate email: " + err.Error(),
		}, http.StatusBadGateway)
		return
	}

	JSONResponse(w, email, http.StatusOK)
}

// ReviseAITemplate updates a generated email using feedback from the user.
func (as *Server) ReviseAITemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Method not allowed",
		}, http.StatusMethodNotAllowed)
		return
	}

	var req ai.RevisionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Invalid request body",
		}, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Feedback) == "" {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Revision feedback is required",
		}, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Email.Subject) == "" {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Email to revise is missing",
		}, http.StatusBadRequest)
		return
	}

	email, err := as.aiGenerator.Revise(r.Context(), req)
	if err != nil {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Failed to revise email: " + err.Error(),
		}, http.StatusBadGateway)
		return
	}

	JSONResponse(w, email, http.StatusOK)
}
