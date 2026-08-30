// Package rungraph owns the reference-only Run personal graph contract and
// principal-scoped persistence described by ADR-023.
package rungraph

import (
	"encoding/json"
	"time"
)

const (
	DocumentAPIVersion = "one.runGraph/v1"
	HomeGraphKey       = "home"

	MaxNodes                = 200
	MaxEdges                = 400
	MaxDocumentBytes        = 256 * 1024
	MaxAnnotationBytes      = 4 * 1024
	MaxBindings             = 50
	MaxCardProjectionFields = 12
	MaxGraphKeyBytes        = 80
	MaxTitleBytes           = 200
	MaxSearchQueryBytes     = 200
	MaxResolveNodes         = 200
)

var AllowedNodeKinds = map[string]struct{}{
	"record":     {},
	"cluster":    {},
	"tool":       {},
	"insight":    {},
	"question":   {},
	"proposal":   {},
	"signal":     {},
	"person":     {},
	"collection": {},
}

var AllowedEdgeKinds = map[string]struct{}{
	"relates":     {},
	"owns":        {},
	"watches":     {},
	"blocks":      {},
	"next":        {},
	"explains":    {},
	"opens":       {},
	"derivedFrom": {},
}

type Layout struct {
	X float64  `json:"x"`
	Y float64  `json:"y"`
	W *float64 `json:"w,omitempty"`
	Z *float64 `json:"z,omitempty"`
}

type RecordRef struct {
	ObjectAPIName string `json:"objectApiName"`
	RecordID      string `json:"recordId"`
}

type ToolRef struct {
	ToolSpecAPIName string `json:"toolSpecApiName,omitempty"`
	WorkingToolID   string `json:"workingToolId,omitempty"`
}

type RunGraphNode struct {
	ID             string          `json:"id"`
	Kind           string          `json:"kind"`
	Ref            json.RawMessage `json:"ref,omitempty"`
	ToolRef        *ToolRef        `json:"toolRef,omitempty"`
	Layout         *Layout         `json:"layout,omitempty"`
	CardProjection []string        `json:"cardProjection,omitempty"`
	Label          string          `json:"label,omitempty"`
	Text           string          `json:"text,omitempty"`
	ProposalID     string          `json:"proposalId,omitempty"`
	BindingID      string          `json:"bindingId,omitempty"`
	SearchQ        string          `json:"searchQ,omitempty"`
}

type RunGraphEdge struct {
	ID     string   `json:"id"`
	From   string   `json:"from"`
	To     string   `json:"to"`
	Kind   string   `json:"kind"`
	Weight *float64 `json:"weight,omitempty"`
}

type RunGraphBinding struct {
	ID            string          `json:"id"`
	ObjectAPIName string          `json:"objectApiName"`
	Fields        []string        `json:"fields,omitempty"`
	Filters       json.RawMessage `json:"filters,omitempty"`
	Sort          json.RawMessage `json:"sort,omitempty"`
	Limit         *int            `json:"limit,omitempty"`
}

type Viewport struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

type RunGraphDocument struct {
	APIVersion   string            `json:"apiVersion"`
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Nodes        []RunGraphNode    `json:"nodes"`
	Edges        []RunGraphEdge    `json:"edges"`
	DataBindings []RunGraphBinding `json:"dataBindings,omitempty"`
	Lenses       json.RawMessage   `json:"lenses,omitempty"`
	Viewport     *Viewport         `json:"viewport,omitempty"`
}

type Graph struct {
	ID          string          `json:"id"`
	PrincipalID string          `json:"-"`
	GraphKey    string          `json:"graphKey"`
	Title       string          `json:"title"`
	Document    json.RawMessage `json:"document"`
	Revision    int64           `json:"revision"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}
