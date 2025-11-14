package notifications

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"path"
	"regexp"
	"strings"
	"sync"
	texttemplate "text/template"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
)

//go:embed templates/*
var templatesFS embed.FS

// TemplateResolver loads and executes notification templates
type TemplateResolver struct {
	subjectTemplates map[string]*texttemplate.Template
	bodyTemplates    map[string]*texttemplate.Template
	htmlTemplates    map[string]*template.Template
	mu               sync.RWMutex
}

// NewTemplateResolver creates a new template resolver
func NewTemplateResolver() (*TemplateResolver, error) {
	resolver := &TemplateResolver{
		subjectTemplates: make(map[string]*texttemplate.Template),
		bodyTemplates:    make(map[string]*texttemplate.Template),
		htmlTemplates:    make(map[string]*template.Template),
	}

	// Preload templates for all notification types
	templateTypes := []notifications.NotificationType{
		notifications.NotificationTypeDocumentApproved,
		notifications.NotificationTypeReviewRequested,
		notifications.NotificationTypeNewOwner,
		notifications.NotificationTypeDocumentPublished,
	}

	for _, notifType := range templateTypes {
		if err := resolver.loadTemplates(string(notifType)); err != nil {
			return nil, fmt.Errorf("failed to load templates for %s: %w", notifType, err)
		}
	}

	return resolver, nil
}

// loadTemplates loads all three template files for a notification type
func (tr *TemplateResolver) loadTemplates(notifType string) error {
	baseDir := path.Join("templates", notifType)

	// Load subject template (text)
	subjectPath := path.Join(baseDir, "subject.tmpl")
	subjectData, err := templatesFS.ReadFile(subjectPath)
	if err != nil {
		return fmt.Errorf("failed to read subject template: %w", err)
	}
	subjectTmpl, err := texttemplate.New(notifType + "_subject").Parse(string(subjectData))
	if err != nil {
		return fmt.Errorf("failed to parse subject template: %w", err)
	}
	tr.subjectTemplates[notifType] = subjectTmpl

	// Load body markdown template (text)
	bodyPath := path.Join(baseDir, "body.md.tmpl")
	bodyData, err := templatesFS.ReadFile(bodyPath)
	if err != nil {
		return fmt.Errorf("failed to read body template: %w", err)
	}
	bodyTmpl, err := texttemplate.New(notifType + "_body").Parse(string(bodyData))
	if err != nil {
		return fmt.Errorf("failed to parse body template: %w", err)
	}
	tr.bodyTemplates[notifType] = bodyTmpl

	// Load HTML template (html/template for auto-escaping)
	htmlPath := path.Join(baseDir, "body.html.tmpl")
	htmlData, err := templatesFS.ReadFile(htmlPath)
	if err != nil {
		return fmt.Errorf("failed to read HTML template: %w", err)
	}
	htmlTmpl, err := template.New(notifType + "_html").Parse(string(htmlData))
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}
	tr.htmlTemplates[notifType] = htmlTmpl

	return nil
}

// ResolvedContent holds the rendered template output
type ResolvedContent struct {
	Subject  string
	Body     string
	BodyHTML string
}

// Resolve renders all templates for a notification type with the given context
func (tr *TemplateResolver) Resolve(notifType notifications.NotificationType, context map[string]any) (*ResolvedContent, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	typeStr := string(notifType)

	// Resolve subject
	subjectTmpl, ok := tr.subjectTemplates[typeStr]
	if !ok {
		return nil, fmt.Errorf("no subject template found for notification type: %s", notifType)
	}
	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, context); err != nil {
		return nil, fmt.Errorf("failed to execute subject template: %w", err)
	}
	subject := subjectBuf.String()

	// Validate subject has no unexpanded template values
	if err := validateNoUnexpandedValues(subject, "subject", typeStr); err != nil {
		return nil, err
	}

	// Resolve body (markdown)
	bodyTmpl, ok := tr.bodyTemplates[typeStr]
	if !ok {
		return nil, fmt.Errorf("no body template found for notification type: %s", notifType)
	}
	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, context); err != nil {
		return nil, fmt.Errorf("failed to execute body template: %w", err)
	}
	body := bodyBuf.String()

	// Validate body has no unexpanded template values
	if err := validateNoUnexpandedValues(body, "body", typeStr); err != nil {
		return nil, err
	}

	// Resolve HTML
	htmlTmpl, ok := tr.htmlTemplates[typeStr]
	if !ok {
		return nil, fmt.Errorf("no HTML template found for notification type: %s", notifType)
	}
	var htmlBuf bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBuf, context); err != nil {
		return nil, fmt.Errorf("failed to execute HTML template: %w", err)
	}
	bodyHTML := htmlBuf.String()

	// Validate HTML has no unexpanded template values
	if err := validateNoUnexpandedValues(bodyHTML, "HTML body", typeStr); err != nil {
		return nil, err
	}

	return &ResolvedContent{
		Subject:  subject,
		Body:     body,
		BodyHTML: bodyHTML,
	}, nil
}

// validateNoUnexpandedValues checks if the rendered content contains any unexpanded template values.
// This is a critical error that indicates missing template context variables.
func validateNoUnexpandedValues(content, templateName, notificationType string) error {
	// Check for <no value> (Go template default for missing values)
	if strings.Contains(content, "<no value>") {
		return fmt.Errorf("template validation failed for %s in notification type %s: found unexpanded template value '<no value>' - missing template context variable", templateName, notificationType)
	}

	// Check for unexpanded template syntax {{...}}
	// This regex matches {{.Variable}} or {{if .Variable}} etc.
	templatePattern := regexp.MustCompile(`\{\{[^}]*\}\}`)
	matches := templatePattern.FindAllString(content, -1)
	if len(matches) > 0 {
		return fmt.Errorf("template validation failed for %s in notification type %s: found unexpanded template syntax: %v - template may have syntax errors or missing context", templateName, notificationType, matches)
	}

	return nil
}
