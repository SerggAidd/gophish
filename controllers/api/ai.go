package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gophish/gophish/ai"
	"github.com/gophish/gophish/models"
)

const maxAIRevisionIterations = 5

// GenerateAITemplate creates a new training email using the configured AI
// generator. This endpoint is kept for the existing UI flow while the new
// difficulty-aware flow is introduced.
func (as *Server) GenerateAITemplate(w http.ResponseWriter, r *http.Request) {
	if !aiRequirePost(w, r) {
		return
	}

	if as.aiGenerator == nil {
		aiUnavailableResponse(w)
		return
	}

	var req ai.GenerationRequest
	if !aiDecodeJSONBody(w, r, &req) {
		return
	}

	if message := validateAIGenerationRequest(req); message != "" {
		aiBadRequest(w, message)
		return
	}

	email, err := as.aiGenerator.Generate(r.Context(), req)
	if err != nil {
		aiBadGateway(w, "Failed to generate email: "+err.Error())
		return
	}

	JSONResponse(w, email, http.StatusOK)
}

// ReviseAITemplate updates an existing email using explicit user feedback.
// Difficulty-directed revision uses ChangeAITemplateDifficulty instead.
func (as *Server) ReviseAITemplate(w http.ResponseWriter, r *http.Request) {
	if !aiRequirePost(w, r) {
		return
	}
	if as.aiGenerator == nil {
		aiUnavailableResponse(w)
		return
	}

	var req ai.RevisionRequest
	if !aiDecodeJSONBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Feedback) == "" {
		aiBadRequest(w, "Revision feedback is required")
		return
	}
	if !aiEmailHasContent(req.Email) {
		aiBadRequest(w, "Email to revise is missing")
		return
	}
	if req.EvaluationContext != nil {
		if err := req.EvaluationContext.Validate(); err != nil {
			aiBadRequest(w, "Invalid evaluation context: "+err.Error())
			return
		}
	}

	email, err := as.aiGenerator.Revise(r.Context(), req)
	if err != nil {
		aiBadGateway(w, "Failed to revise email: "+err.Error())
		return
	}

	JSONResponse(w, email, http.StatusOK)
}

// GenerateAndAdjustAITemplate performs the complete AI flow: initial
// generation, evaluation, and feedback-directed revision toward the requested
// NIST detection difficulty.
func (as *Server) GenerateAndAdjustAITemplate(w http.ResponseWriter, r *http.Request) {
	if !aiRequirePost(w, r) {
		return
	}
	if as.aiDifficultyAgent == nil {
		aiUnavailableResponse(w)
		return
	}

	var req ai.DifficultyGenerationRequest
	if !aiDecodeJSONBody(w, r, &req) {
		return
	}
	if message := validateAIGenerationRequest(req.GenerationContext); message != "" {
		aiBadRequest(w, message)
		return
	}
	if !validAITargetDifficulty(req.GenerationContext.TargetDifficulty) {
		aiBadRequest(w, "Invalid target difficulty")
		return
	}
	if err := req.EvaluationContext.Validate(); err != nil {
		aiBadRequest(w, "Invalid evaluation context: "+err.Error())
		return
	}
	if message := validateAIMaxIterations(req.MaxIterations); message != "" {
		aiBadRequest(w, message)
		return
	}

	result, err := as.aiDifficultyAgent.GenerateAndAdjust(r.Context(), req)
	if err != nil {
		aiBadGateway(w, "Failed to generate and evaluate email: "+err.Error())
		return
	}

	JSONResponse(w, result, http.StatusOK)
}

// EvaluateAITemplate evaluates any current template, including a manually
// written template, using the supplied generation and campaign context.
func (as *Server) EvaluateAITemplate(w http.ResponseWriter, r *http.Request) {
	if !aiRequirePost(w, r) {
		return
	}
	if as.aiEvaluator == nil {
		aiUnavailableResponse(w)
		return
	}

	var input ai.EvaluationInput
	if !aiDecodeJSONBody(w, r, &input) {
		return
	}
	if message := validateAIEvaluationInput(input); message != "" {
		aiBadRequest(w, message)
		return
	}

	result, err := as.aiEvaluator.Evaluate(r.Context(), input)
	if err != nil {
		aiBadGateway(w, "Failed to evaluate email: "+err.Error())
		return
	}

	JSONResponse(w, result, http.StatusOK)
}

// ChangeAITemplateDifficulty evaluates an existing email and iteratively
// revises only email-controlled content toward a new target difficulty.
func (as *Server) ChangeAITemplateDifficulty(w http.ResponseWriter, r *http.Request) {
	if !aiRequirePost(w, r) {
		return
	}

	if as.aiDifficultyAgent == nil {
		aiUnavailableResponse(w)
		return
	}

	var req ai.DifficultyAdjustmentRequest
	if !aiDecodeJSONBody(w, r, &req) {
		return
	}
	if message := validateAIEvaluationInput(req.Input); message != "" {
		aiBadRequest(w, message)
		return
	}
	if !validAITargetDifficulty(string(req.Target)) {
		aiBadRequest(w, "Invalid target difficulty")
		return
	}
	if message := validateAIMaxIterations(req.MaxIterations); message != "" {
		aiBadRequest(w, message)
		return
	}

	result, err := as.aiDifficultyAgent.Adjust(r.Context(), req)
	if err != nil {
		aiBadGateway(w, "Failed to change email difficulty: "+err.Error())
		return
	}

	JSONResponse(w, result, http.StatusOK)
}

func validateAIGenerationRequest(req ai.GenerationRequest) string {
	if strings.TrimSpace(req.TargetAudience) == "" {
		return "Target audience is required"
	}
	if req.Scenario == "custom" && strings.TrimSpace(req.CustomScenario) == "" {
		return "Custom scenario description is required"
	}

	return ""
}

func validateAIEvaluationInput(input ai.EvaluationInput) string {
	if !aiEmailHasContent(input.Email) {
		return "Email to evaluate is missing"
	}
	if err := input.EvaluationContext.Validate(); err != nil {
		return "Invalid evaluation context: " + err.Error()
	}

	return ""
}

func aiEmailHasContent(email ai.Email) bool {
	return strings.TrimSpace(email.Subject) != "" ||
		strings.TrimSpace(email.Text) != "" ||
		strings.TrimSpace(email.HTML) != ""
}

func validAITargetDifficulty(value string) bool {
	switch ai.DetectionDifficulty(value) {
	case ai.DifficultyVeryDifficult,
		ai.DifficultyModeratelyDifficult,
		ai.DifficultyModeratelyToLeastDifficult,
		ai.DifficultyLeastDifficult:
		return true
	default:
		return false
	}
}

func validateAIMaxIterations(value int) string {
	if value < 0 {
		return "Max iterations cannot be negative"
	}

	if value > maxAIRevisionIterations {
		return "Max iterations cannot exceed 5"
	}

	return ""
}

func aiDecodeJSONBody(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		aiBadRequest(w, "Invalid request body")
		return false
	}

	return true
}

func aiRequirePost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodPost {
		return true
	}

	JSONResponse(w, models.Response{
		Success: false,
		Message: "Method not allowed",
	}, http.StatusMethodNotAllowed)
	return false
}

func aiUnavailableResponse(w http.ResponseWriter) {
	JSONResponse(w, models.Response{
		Success: false,
		Message: "AI functionality is not configured",
	}, http.StatusServiceUnavailable)
}

func aiBadRequest(w http.ResponseWriter, message string) {
	JSONResponse(w, models.Response{
		Success: false,
		Message: message,
	}, http.StatusBadRequest)
}

func aiBadGateway(w http.ResponseWriter, message string) {
	JSONResponse(w, models.Response{
		Success: false,
		Message: message,
	}, http.StatusBadGateway)
}
