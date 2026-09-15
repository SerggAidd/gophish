package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gophish/gophish/models"
)

// AITemplateGenerationRequest contains the parameters entered in the generation form.
type AITemplateGenerationRequest struct {
	TargetAudience         string `json:"target_audience"`
	RecipientRole          string `json:"recipient_role"`
	OrganizationContext    string `json:"organization_context"`
	Scenario               string `json:"scenario"`
	CustomScenario         string `json:"custom_scenario"`
	Language               string `json:"language"`
	TargetDifficulty       string `json:"target_difficulty"`
	AdditionalInstructions string `json:"additional_instructions"`
}

// AITemplateContent is the email returned to the frontend.
type AITemplateContent struct {
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html"`
}

// AITemplateRevisionRequest contains the current email and the operator's feedback.
type AITemplateRevisionRequest struct {
	Email               AITemplateContent `json:"email"`
	Feedback            string            `json:"feedback"`
	TargetAudience      string            `json:"target_audience"`
	RecipientRole       string            `json:"recipient_role"`
	OrganizationContext string            `json:"organization_context"`
	Scenario            string            `json:"scenario"`
	CustomScenario      string            `json:"custom_scenario"`
	Language            string            `json:"language"`
	TargetDifficulty    string            `json:"target_difficulty"`
}

// GenerateAITemplate handles the first generation request from the AI modal.
// The temporary response will be replaced with a call to the LLM generator later.
func (as *Server) GenerateAITemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Method not allowed",
		}, http.StatusMethodNotAllowed)
		return
	}

	var req AITemplateGenerationRequest
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

	response := buildTestTemplate(req)
	JSONResponse(w, response, http.StatusOK)
}

// ReviseAITemplate handles changes requested by the operator after reviewing the email.
// For now it only marks the test response as revised. The LLM revision step will replace this.
func (as *Server) ReviseAITemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONResponse(w, models.Response{
			Success: false,
			Message: "Method not allowed",
		}, http.StatusMethodNotAllowed)
		return
	}

	var req AITemplateRevisionRequest
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

	response := req.Email

	if req.Language == "ru" {
		response.Subject = strings.TrimSuffix(response.Subject, " (доработано)") + " (доработано)"
	} else {
		response.Subject = strings.TrimSuffix(response.Subject, " (revised)") + " (revised)"
	}

	JSONResponse(w, response, http.StatusOK)
}

// buildTestTemplate gives us a predictable response while the UI workflow is being developed.
// It deliberately does not call an LLM yet.
func buildTestTemplate(req AITemplateGenerationRequest) AITemplateContent {
	subject := "Training notification"

	if req.Language == "ru" {
		switch req.Scenario {
		case "password_expiration":
			subject = "Уведомление об истечении срока действия пароля"
		case "document_approval":
			subject = "Документ ожидает согласования"
		case "account_notification":
			subject = "Уведомление об учетной записи"
		case "hr_notification":
			subject = "Уведомление от HR"
		case "internal_service":
			subject = "Уведомление внутреннего сервиса"
		case "custom":
			subject = "Учебное уведомление"
		}

		text := "Здравствуйте,\n\nЭто временный тестовый шаблон для проверки нового процесса генерации и согласования письма.\n\nС уважением,\nКоманда информационной безопасности"
		html := "<p>Здравствуйте,</p><p>Это временный тестовый шаблон для проверки нового процесса генерации и согласования письма.</p><p>С уважением,<br>Команда информационной безопасности</p>"

		return AITemplateContent{
			Subject: subject,
			Text:    text,
			HTML:    html,
		}
	}

	switch req.Scenario {
	case "password_expiration":
		subject = "Password expiration notice"
	case "document_approval":
		subject = "Document approval requested"
	case "account_notification":
		subject = "Account notification"
	case "hr_notification":
		subject = "HR notification"
	case "internal_service":
		subject = "Internal service notification"
	case "custom":
		subject = "Training notification"
	}

	text := "Hello,\n\nThis is a temporary training template used to test the new generation and review workflow.\n\nRegards,\nInformation Security Team"
	html := "<p>Hello,</p><p>This is a temporary training template used to test the new generation and review workflow.</p><p>Regards,<br>Information Security Team</p>"

	return AITemplateContent{
		Subject: subject,
		Text:    text,
		HTML:    html,
	}
}
