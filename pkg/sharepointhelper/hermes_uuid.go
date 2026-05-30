// Package sharepointhelper contains Microsoft Graph helpers for SharePoint.
package sharepointhelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp-forge/hermes/pkg/docid"
)

const hermesUUIDFieldName = "HermesUuid"

// GetHermesUUID reads the persisted Hermes UUID from the SharePoint item's list fields.
func (s *Service) GetHermesUUID(itemID string) (docid.UUID, bool, error) {
	requestURL := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/sites/%s/drives/%s/items/%s/listItem/fields?$select=%s",
		s.SiteID, s.DriveID, itemID, hermesUUIDFieldName,
	)

	resp, err := s.InvokeAPI("GET", requestURL, nil)
	if err != nil {
		return docid.UUID{}, false, fmt.Errorf("error reading Hermes UUID field: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return docid.UUID{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return docid.UUID{}, false, fmt.Errorf("failed to read Hermes UUID field: %s, %s", resp.Status, string(body))
	}

	var fields map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&fields); err != nil {
		return docid.UUID{}, false, fmt.Errorf("error decoding Hermes UUID field response: %w", err)
	}

	value, ok := fields[hermesUUIDFieldName].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return docid.UUID{}, false, nil
	}

	uuid, err := docid.ParseUUID(value)
	if err != nil {
		return docid.UUID{}, false, fmt.Errorf("invalid Hermes UUID field value %q: %w", value, err)
	}
	return uuid, true, nil
}

// SetHermesUUID persists the Hermes UUID on the SharePoint item's list fields.
func (s *Service) SetHermesUUID(itemID string, uuid docid.UUID) error {
	requestURL := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/sites/%s/drives/%s/items/%s/listItem/fields",
		s.SiteID, s.DriveID, itemID,
	)
	body, err := json.Marshal(map[string]string{hermesUUIDFieldName: uuid.String()})
	if err != nil {
		return fmt.Errorf("error marshaling Hermes UUID field request: %w", err)
	}

	resp, err := s.InvokeAPI("PATCH", requestURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("error setting Hermes UUID field: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set Hermes UUID field: %s, %s", resp.Status, string(body))
	}
	return nil
}

// FindFileByHermesUUID returns the SharePoint drive item with the matching Hermes UUID.
func (s *Service) FindFileByHermesUUID(uuid docid.UUID) (*Document, error) {
	filter := fmt.Sprintf("fields/%s eq '%s'", hermesUUIDFieldName, strings.ReplaceAll(uuid.String(), "'", "''"))
	expand := fmt.Sprintf("fields($select=%s),driveItem", hermesUUIDFieldName)
	requestURL := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/sites/%s/drives/%s/list/items?$filter=%s&$expand=%s",
		s.SiteID, s.DriveID, url.QueryEscape(filter), url.QueryEscape(expand),
	)

	options := &APIOptions{Headers: map[string]string{"ConsistencyLevel": "eventual"}}
	resp, err := s.InvokeAPIWithOptions("GET", requestURL, nil, options)
	if err != nil {
		return nil, fmt.Errorf("error searching SharePoint by Hermes UUID: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search SharePoint by Hermes UUID: %s, %s", resp.Status, string(body))
	}

	var result struct {
		Value []struct {
			DriveItem Document `json:"driveItem"`
		} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding Hermes UUID search response: %w", err)
	}

	if len(result.Value) == 0 {
		return nil, fmt.Errorf("SharePoint document with Hermes UUID %s not found", uuid.String())
	}
	if len(result.Value) > 1 {
		return nil, fmt.Errorf("multiple SharePoint documents found with Hermes UUID %s", uuid.String())
	}

	doc := result.Value[0].DriveItem
	if doc.ID == "" {
		return nil, fmt.Errorf("SharePoint UUID search result did not include drive item ID")
	}
	return &doc, nil
}
