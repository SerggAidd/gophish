package ai

import (
	"errors"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// An HTML tag alone can satisfy the JSON schema while leaving the actual
// email body blank in a mail client. Keep this check separate from the NIST
// scoring rules: an unusable template should never be scored as a draft.
var errUnusableEmailHTML = errors.New("HTML body has no readable message text")

func validateEmailHTMLContent(body string) error {
	if strings.TrimSpace(body) == "" {
		return errUnusableEmailHTML
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: %v", errUnusableEmailHTML, err)
	}
	visible := doc.Find("body")
	visible.Find("script, style, template").Remove()
	if strings.TrimSpace(visible.Text()) == "" {
		return errUnusableEmailHTML
	}
	return nil
}
