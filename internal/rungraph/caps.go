package rungraph

import (
	"encoding/json"
	"fmt"
)

// EnforceCaps applies ADR-023 resource limits to an already-sanitized document.
func EnforceCaps(raw json.RawMessage) error {
	if len(raw) > MaxDocumentBytes {
		return fmt.Errorf("document exceeds max %d bytes", MaxDocumentBytes)
	}
	var doc struct {
		Title string `json:"title"`
		Nodes []struct {
			Kind           string   `json:"kind"`
			Text           string   `json:"text"`
			SearchQ        string   `json:"searchQ"`
			CardProjection []string `json:"cardProjection"`
		} `json:"nodes"`
		Edges        []json.RawMessage `json:"edges"`
		DataBindings []json.RawMessage `json:"dataBindings"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("document: invalid JSON")
	}
	if len(doc.Title) > MaxTitleBytes {
		return fmt.Errorf("title exceeds max %d bytes", MaxTitleBytes)
	}
	if len(doc.Nodes) > MaxNodes {
		return fmt.Errorf("nodes exceeds max %d", MaxNodes)
	}
	if len(doc.Edges) > MaxEdges {
		return fmt.Errorf("edges exceeds max %d", MaxEdges)
	}
	if len(doc.DataBindings) > MaxBindings {
		return fmt.Errorf("dataBindings exceeds max %d", MaxBindings)
	}
	for i, node := range doc.Nodes {
		if (node.Kind == "insight" || node.Kind == "question") && len(node.Text) > MaxAnnotationBytes {
			return fmt.Errorf("nodes[%d].text exceeds max %d bytes", i, MaxAnnotationBytes)
		}
		if node.Kind == "collection" && len(node.SearchQ) > MaxSearchQueryBytes {
			return fmt.Errorf("nodes[%d].searchQ exceeds max %d bytes", i, MaxSearchQueryBytes)
		}
		if len(node.CardProjection) > MaxCardProjectionFields {
			return fmt.Errorf("nodes[%d].cardProjection exceeds max %d fields", i, MaxCardProjectionFields)
		}
	}
	return nil
}

// PrepareDocument applies the mandatory write path in order: validate,
// sanitize, validate the sanitized closed shape, then enforce caps.
func PrepareDocument(raw json.RawMessage) (json.RawMessage, error) {
	if err := ValidateDocument(raw); err != nil {
		return nil, err
	}
	sanitized, err := SanitizeDocument(raw)
	if err != nil {
		return nil, err
	}
	if err := ValidateDocument(sanitized); err != nil {
		return nil, err
	}
	if err := EnforceCaps(sanitized); err != nil {
		return nil, err
	}
	return sanitized, nil
}
