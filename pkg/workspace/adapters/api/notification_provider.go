package api

import (
	"context"
	"fmt"
)

// ===================================================================
// NotificationProvider Implementation
// ===================================================================
// All methods delegate to remote Hermes /api/v2/notifications/* endpoints

// SendEmail sends an email notification via remote Hermes
func (p *Provider) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
	if err := p.checkCapability("email"); err != nil {
		return err
	}

	path := "/api/v2/notifications/email"

	requestBody := map[string]interface{}{
		"to":      to,
		"from":    from,
		"subject": subject,
		"body":    body,
	}

	if err := p.doRequest(ctx, "POST", path, requestBody, nil); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// SendEmailWithTemplate sends email using template on remote Hermes
func (p *Provider) SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error {
	if err := p.checkCapability("email"); err != nil {
		return err
	}

	path := "/api/v2/notifications/email/template"

	requestBody := map[string]interface{}{
		"to":       to,
		"template": template,
		"data":     data,
	}

	if err := p.doRequest(ctx, "POST", path, requestBody, nil); err != nil {
		return fmt.Errorf("failed to send email with template: %w", err)
	}

	return nil
}
