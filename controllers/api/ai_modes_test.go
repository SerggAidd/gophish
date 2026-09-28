package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gophish/gophish/ai"
)

func TestEvaluateRejectsUnrecognizedModeBeforeCallingModel(t *testing.T) {
	server := &Server{aiEvaluator: &ai.EmailEvaluator{}}
	request := httptest.NewRequest(http.MethodPost, "/api/ai/templates/evaluate",
		strings.NewReader(`{"email":{"subject":"Example"},"evaluation_mode":"unexpected"}`))
	response := httptest.NewRecorder()
	server.EvaluateAITemplate(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid evaluation mode: HTTP %d, response %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Invalid evaluation_mode") {
		t.Fatalf("unexpected error: %s", response.Body.String())
	}
}
