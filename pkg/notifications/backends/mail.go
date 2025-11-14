package backends

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
)

// MailBackend sends notification emails via SMTP
type MailBackend struct {
	smtpHost     string
	smtpPort     string
	smtpUsername string
	smtpPassword string
	fromAddress  string
	fromName     string
	useTLS       bool
}

// MailBackendConfig configures the mail backend
type MailBackendConfig struct {
	SMTPHost     string // SMTP server hostname
	SMTPPort     string // SMTP server port (typically 587 for TLS, 25 for plaintext)
	SMTPUsername string // SMTP username (optional for auth)
	SMTPPassword string // SMTP password (optional for auth)
	FromAddress  string // From email address
	FromName     string // From display name
	UseTLS       bool   // Use STARTTLS (recommended for port 587)
}

// NewMailBackend creates a new mail backend
func NewMailBackend(cfg MailBackendConfig) *MailBackend {
	return &MailBackend{
		smtpHost:     cfg.SMTPHost,
		smtpPort:     cfg.SMTPPort,
		smtpUsername: cfg.SMTPUsername,
		smtpPassword: cfg.SMTPPassword,
		fromAddress:  cfg.FromAddress,
		fromName:     cfg.FromName,
		useTLS:       cfg.UseTLS,
	}
}

// Name returns the backend identifier
func (b *MailBackend) Name() string {
	return "mail"
}

// SupportsBackend checks if this backend should process the message
func (b *MailBackend) SupportsBackend(backend string) bool {
	return backend == "mail" || backend == "email"
}

// Handle processes a notification message by sending emails
func (b *MailBackend) Handle(ctx context.Context, msg *notifications.NotificationMessage) error {
	// Extract email recipients
	var recipients []string
	for _, r := range msg.Recipients {
		if r.Email != "" {
			recipients = append(recipients, r.Email)
		}
	}

	if len(recipients) == 0 {
		return fmt.Errorf("no email recipients found in notification")
	}

	// Render email subject and body based on template
	subject, body, err := b.renderEmail(msg)
	if err != nil {
		return fmt.Errorf("failed to render email: %w", err)
	}

	// Send email to each recipient
	for _, to := range recipients {
		if err := b.sendEmail(to, subject, body); err != nil {
			return fmt.Errorf("failed to send email to %s: %w", to, err)
		}
	}

	return nil
}

// renderEmail generates email subject and HTML body from notification message
func (b *MailBackend) renderEmail(msg *notifications.NotificationMessage) (string, string, error) {
	// Build subject based on notification type
	subject := b.buildSubject(msg)

	// Build HTML body based on template
	body, err := b.buildBody(msg)
	if err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// buildSubject creates email subject based on notification type and context
func (b *MailBackend) buildSubject(msg *notifications.NotificationMessage) string {
	switch msg.Type {
	case notifications.NotificationTypeDocumentApproved:
		if approver, ok := msg.TemplateContext["ApproverName"].(string); ok {
			if docName, ok := msg.TemplateContext["DocumentShortName"].(string); ok {
				return fmt.Sprintf("%s approved by %s", docName, approver)
			}
		}
		return "Document approved"

	case notifications.NotificationTypeReviewRequested:
		if docName, ok := msg.TemplateContext["DocumentShortName"].(string); ok {
			return fmt.Sprintf("Document review requested for %s", docName)
		}
		return "Document review requested"

	case notifications.NotificationTypeDocumentPublished:
		if docName, ok := msg.TemplateContext["DocumentShortName"].(string); ok {
			return fmt.Sprintf("Document published: %s", docName)
		}
		return "Document published"

	default:
		return "Hermes notification"
	}
}

// buildBody creates HTML email body from template and context
func (b *MailBackend) buildBody(msg *notifications.NotificationMessage) (string, error) {
	// Simple HTML template for now
	// In production, this would load templates from files or embed.FS
	tmplText := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Subject}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background-color: #5c4ee5;
            color: white;
            padding: 20px;
            border-radius: 5px 5px 0 0;
        }
        .content {
            background-color: #f9f9f9;
            padding: 20px;
            border: 1px solid #ddd;
            border-top: none;
            border-radius: 0 0 5px 5px;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #5c4ee5;
            color: white;
            text-decoration: none;
            border-radius: 4px;
            margin-top: 15px;
        }
        .footer {
            margin-top: 20px;
            padding-top: 20px;
            border-top: 1px solid #ddd;
            font-size: 12px;
            color: #666;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{.Type}}</h1>
    </div>
    <div class="content">
        {{range $key, $value := .Context}}
        <p><strong>{{$key}}:</strong> {{$value}}</p>
        {{end}}
        {{if .DocumentURL}}
        <a href="{{.DocumentURL}}" class="button">View Document</a>
        {{end}}
    </div>
    <div class="footer">
        <p>This is an automated notification from Hermes.</p>
    </div>
</body>
</html>`

	tmpl, err := template.New("email").Parse(tmplText)
	if err != nil {
		return "", fmt.Errorf("failed to parse email template: %w", err)
	}

	// Prepare template data
	data := struct {
		Subject     string
		Type        string
		Context     map[string]any
		DocumentURL string
	}{
		Subject:     b.buildSubject(msg),
		Type:        string(msg.Type),
		Context:     msg.TemplateContext,
		DocumentURL: "",
	}

	// Extract document URL if available
	if baseURL, ok := msg.TemplateContext["BaseURL"].(string); ok {
		if docID, ok := msg.TemplateContext["DocumentID"].(string); ok {
			data.DocumentURL = baseURL + "/document/" + docID
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute email template: %w", err)
	}

	return buf.String(), nil
}

// sendEmail sends an email via SMTP
func (b *MailBackend) sendEmail(to, subject, htmlBody string) error {
	from := b.fromAddress
	if b.fromName != "" {
		from = fmt.Sprintf("%s <%s>", b.fromName, b.fromAddress)
	}

	// Build email message with headers
	msg := []byte(fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n"+
			"\r\n"+
			"%s",
		from, to, subject, htmlBody,
	))

	addr := fmt.Sprintf("%s:%s", b.smtpHost, b.smtpPort)

	// Setup authentication
	var auth smtp.Auth
	if b.smtpUsername != "" && b.smtpPassword != "" {
		auth = smtp.PlainAuth("", b.smtpUsername, b.smtpPassword, b.smtpHost)
	}

	// Send email
	if b.useTLS {
		return b.sendMailTLS(addr, auth, b.fromAddress, []string{to}, msg)
	}

	return smtp.SendMail(addr, auth, b.fromAddress, []string{to}, msg)
}

// sendMailTLS sends email with STARTTLS support
func (b *MailBackend) sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	// Connect to SMTP server
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// Start TLS
	if err = client.StartTLS(&tls.Config{
		ServerName:         b.smtpHost,
		InsecureSkipVerify: false,
	}); err != nil {
		return fmt.Errorf("failed to start TLS: %w", err)
	}

	// Authenticate
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Set sender
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, addr := range to {
		if err = client.Rcpt(addr); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", addr, err)
		}
	}

	// Send message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}
