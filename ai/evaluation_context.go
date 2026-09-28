package ai

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"
)

// SenderIdentity describes the sender identity used by the campaign. ReplyTo
// is optional and is relevant when it differs from the visible From address.
type SenderIdentity struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	ReplyTo     string `json:"reply_to,omitempty"`
}

// LinkUsage describes whether a phishing link is planned for the template.
type LinkUsage string

const (
	LinkUsageUnknown LinkUsage = "unknown"
	LinkUsageNone    LinkUsage = "none"
	LinkUsageUsed    LinkUsage = "used"
)

// LinkContext contains the simulated link and the legitimate domain it is
// intended to resemble. Unknown values are allowed during draft evaluation.
type LinkContext struct {
	Usage          LinkUsage `json:"usage"`
	SimulatedURL   string    `json:"simulated_url,omitempty"`
	ExpectedDomain string    `json:"expected_domain,omitempty"`
}

// AttachmentUsage distinguishes an explicitly attachment-free template from a
// draft whose attachment configuration has not been confirmed yet.
type AttachmentUsage string

const (
	AttachmentUsageUnknown AttachmentUsage = "unknown"
	AttachmentUsageNone    AttachmentUsage = "none"
	AttachmentUsageUsed    AttachmentUsage = "used"
)

// AttachmentMetadata contains only the information required by the difficulty
// evaluator. Attachment contents are intentionally not part of the model.
type AttachmentMetadata struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// AttachmentContext represents the shared attachment state used by generation
// and evaluation UI flows.
type AttachmentContext struct {
	Usage AttachmentUsage      `json:"usage"`
	Files []AttachmentMetadata `json:"files,omitempty"`
}

// EvaluationContext contains campaign-level information that affects phishing
// difficulty but is not part of the generated email body itself.
type EvaluationContext struct {
	PriorTrainingExposure TrainingExposure  `json:"prior_training_exposure"`
	SimulatedSender       *SenderIdentity   `json:"simulated_sender,omitempty"`
	ExpectedSender        *SenderIdentity   `json:"expected_sender,omitempty"`
	SituationContext      string            `json:"situation_context,omitempty"`
	Link                  LinkContext       `json:"link"`
	Attachments           AttachmentContext `json:"attachments"`
}

// EvaluationInput combines the email with the generation and campaign context
// required by the cue and premise-alignment evaluators.
type EvaluationInput struct {
	Email             Email             `json:"email"`
	GenerationContext GenerationRequest `json:"generation_context"`
	EvaluationContext EvaluationContext `json:"evaluation_context"`
}

// Validate checks structural consistency. Missing values remain allowed so a
// later evaluator can return a range instead of inventing a value.
func (c EvaluationContext) Validate() error {
	if !validTrainingExposure(c.PriorTrainingExposure) {
		return fmt.Errorf(
			"invalid prior training exposure: %q",
			c.PriorTrainingExposure,
		)
	}

	if err := validateSenderIdentity("simulated sender", c.SimulatedSender); err != nil {
		return err
	}

	if err := validateSenderIdentity("expected sender", c.ExpectedSender); err != nil {
		return err
	}

	if err := c.Link.validate(); err != nil {
		return err
	}

	if err := c.Attachments.validate(); err != nil {
		return err
	}

	return nil
}

// MissingFields returns context fields that are still unknown. This is intended
// for partial-evaluation status and UI guidance, not as validation failure.
func (c EvaluationContext) MissingFields() []string {
	missing := make([]string, 0, 7)

	if c.PriorTrainingExposure == "" || c.PriorTrainingExposure == TrainingExposureUnknown {
		missing = append(missing, "prior_training_exposure")
	}

	if senderUnknown(c.SimulatedSender) {
		missing = append(missing, "simulated_sender")
	}

	if senderUnknown(c.ExpectedSender) {
		missing = append(missing, "expected_sender")
	}

	if strings.TrimSpace(c.SituationContext) == "" {
		missing = append(missing, "situation_context")
	}

	usage := normalizedLinkUsage(c.Link.Usage)
	if usage == LinkUsageUnknown {
		missing = append(missing, "link")
	} else if usage == LinkUsageUsed {
		if strings.TrimSpace(c.Link.SimulatedURL) == "" {
			missing = append(missing, "simulated_link")
		}
		if strings.TrimSpace(c.Link.ExpectedDomain) == "" {
			missing = append(missing, "expected_link_domain")
		}
	}

	if normalizedAttachmentUsage(c.Attachments.Usage) == AttachmentUsageUnknown {
		missing = append(missing, "attachments")
	}

	return missing
}

func validTrainingExposure(exposure TrainingExposure) bool {
	switch exposure {
	case "", TrainingExposureUnknown, TrainingExposureNone, TrainingExposureLow,
		TrainingExposureModerate, TrainingExposureSignificant, TrainingExposureExtreme:
		return true
	default:
		return false
	}
}

func validateSenderIdentity(label string, sender *SenderIdentity) error {
	if sender == nil {
		return nil
	}

	if strings.TrimSpace(sender.DisplayName) == "" && strings.TrimSpace(sender.Email) == "" {
		return fmt.Errorf("%s is empty", label)
	}

	if email := strings.TrimSpace(sender.Email); email != "" {
		if _, err := parseMailboxAddress(email); err != nil {
			return fmt.Errorf("%s has invalid email: %w", label, err)
		}
	}

	if replyTo := strings.TrimSpace(sender.ReplyTo); replyTo != "" {
		if _, err := parseMailboxAddress(replyTo); err != nil {
			return fmt.Errorf("%s has invalid reply-to: %w", label, err)
		}
	}

	return nil
}

func senderUnknown(sender *SenderIdentity) bool {
	return sender == nil || strings.TrimSpace(sender.Email) == ""
}

func (c LinkContext) validate() error {
	usage := normalizedLinkUsage(c.Usage)

	switch usage {
	case LinkUsageUnknown:
		if strings.TrimSpace(c.SimulatedURL) != "" || strings.TrimSpace(c.ExpectedDomain) != "" {
			return fmt.Errorf("link usage is unknown but link context contains values")
		}
	case LinkUsageNone:
		if strings.TrimSpace(c.SimulatedURL) != "" || strings.TrimSpace(c.ExpectedDomain) != "" {
			return fmt.Errorf("link usage is none but link context contains values")
		}
	case LinkUsageUsed:
		if simulatedURL := strings.TrimSpace(c.SimulatedURL); simulatedURL != "" {
			if _, err := normalizedURLHost(simulatedURL); err != nil {
				return fmt.Errorf("invalid simulated URL: %w", err)
			}
		}

		if expectedDomain := strings.TrimSpace(c.ExpectedDomain); expectedDomain != "" {
			if _, err := normalizedDomain(expectedDomain); err != nil {
				return fmt.Errorf("invalid expected link domain: %w", err)
			}
		}
	default:
		return fmt.Errorf("invalid link usage: %q", c.Usage)
	}

	return nil
}

func normalizedLinkUsage(usage LinkUsage) LinkUsage {
	if usage == "" {
		return LinkUsageUnknown
	}

	return usage
}

func (c AttachmentContext) validate() error {
	usage := normalizedAttachmentUsage(c.Usage)

	switch usage {
	case AttachmentUsageUnknown:
		if len(c.Files) > 0 {
			return fmt.Errorf("attachment usage is unknown but files are present")
		}
	case AttachmentUsageNone:
		if len(c.Files) > 0 {
			return fmt.Errorf("attachment usage is none but files are present")
		}
	case AttachmentUsageUsed:
		if len(c.Files) == 0 {
			return fmt.Errorf("attachment usage is used but no files are present")
		}
	default:
		return fmt.Errorf("invalid attachment usage: %q", c.Usage)
	}

	for i, file := range c.Files {
		if strings.TrimSpace(file.Name) == "" {
			return fmt.Errorf("attachment %d has an empty name", i)
		}
	}

	return nil
}

func normalizedAttachmentUsage(usage AttachmentUsage) AttachmentUsage {
	if usage == "" {
		return AttachmentUsageUnknown
	}

	return usage
}

func parseMailboxAddress(value string) (*mail.Address, error) {
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(address.Address) == "" {
		return nil, fmt.Errorf("empty address")
	}

	return address, nil
}

func mailboxDomain(value string) (string, error) {
	address, err := parseMailboxAddress(value)
	if err != nil {
		return "", err
	}

	parts := strings.Split(address.Address, "@")
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return "", fmt.Errorf("address has no domain")
	}

	return normalizeDomainString(parts[1]), nil
}

func normalizedURLHost(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("empty URL")
	}

	if !strings.Contains(value, "://") {
		value = "https://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}

	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("URL has no hostname")
	}

	return normalizeDomainString(host), nil
}

func normalizedDomain(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("empty domain")
	}

	if strings.Contains(value, "/") || strings.Contains(value, "://") {
		return normalizedURLHost(value)
	}

	value = normalizeDomainString(value)
	if value == "" || strings.ContainsAny(value, " @") {
		return "", fmt.Errorf("invalid domain")
	}

	return value, nil
}

func normalizeDomainString(value string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
}

func sameDomainOrSubdomain(actual, expected string) bool {
	actual = normalizeDomainString(actual)
	expected = normalizeDomainString(expected)

	return actual == expected || strings.HasSuffix(actual, "."+expected)
}
