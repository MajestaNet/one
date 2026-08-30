package rungraph

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var graphKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var bakedPayloadKeys = map[string]struct{}{
	"rows":        {},
	"data":        {},
	"fields":      {},
	"recordIds":   {},
	"cards":       {},
	"messages":    {},
	"recordId":    {},
	"value":       {},
	"operations":  {},
	"hydrated":    {},
	"snapshot":    {},
	"queryResult": {},
	"records":     {},
}

func ValidateGraphKey(graphKey string) error {
	if graphKey == "" {
		return fmt.Errorf("graph key is required")
	}
	if len(graphKey) > MaxGraphKeyBytes {
		return fmt.Errorf("graph key exceeds max %d bytes", MaxGraphKeyBytes)
	}
	if !graphKeyPattern.MatchString(graphKey) {
		return fmt.Errorf("graph key may contain only letters, numbers, dot, underscore, and hyphen")
	}
	return nil
}

// ValidateDocument validates the closed one.runGraph/v1 topology schema.
// Known baked-payload keys are temporarily tolerated so SanitizeDocument can
// strip them on write; all other unknown structural keys fail closed.
func ValidateDocument(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("document body is required")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("document: invalid JSON")
	}
	if err := allowedKeys("document", doc, "apiVersion", "id", "title", "revision", "nodes", "edges", "dataBindings", "lenses", "viewport"); err != nil {
		return err
	}
	if doc["apiVersion"] != DocumentAPIVersion {
		return fmt.Errorf(`apiVersion must be %q`, DocumentAPIVersion)
	}
	id, _ := doc["id"].(string)
	if err := ValidateGraphKey(strings.TrimSpace(id)); err != nil {
		return fmt.Errorf("id: %w", err)
	}
	title, _ := doc["title"].(string)
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("title is required")
	}
	nodes, ok := doc["nodes"].([]any)
	if !ok {
		return fmt.Errorf("nodes must be an array")
	}
	edges, ok := doc["edges"].([]any)
	if !ok {
		return fmt.Errorf("edges must be an array")
	}

	nodeIDs := make(map[string]struct{}, len(nodes))
	for i, rawNode := range nodes {
		node, ok := rawNode.(map[string]any)
		if !ok {
			return fmt.Errorf("nodes[%d] must be an object", i)
		}
		if err := validateNode(i, node); err != nil {
			return err
		}
		nodeID := node["id"].(string)
		if _, duplicate := nodeIDs[nodeID]; duplicate {
			return fmt.Errorf("duplicate node id %q", nodeID)
		}
		nodeIDs[nodeID] = struct{}{}
	}

	edgeIDs := make(map[string]struct{}, len(edges))
	for i, rawEdge := range edges {
		edge, ok := rawEdge.(map[string]any)
		if !ok {
			return fmt.Errorf("edges[%d] must be an object", i)
		}
		if err := allowedKeys(fmt.Sprintf("edges[%d]", i), edge, "id", "from", "to", "kind", "weight"); err != nil {
			return err
		}
		id, _ := edge["id"].(string)
		from, _ := edge["from"].(string)
		to, _ := edge["to"].(string)
		kind, _ := edge["kind"].(string)
		if strings.TrimSpace(id) == "" || strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
			return fmt.Errorf("edges[%d] requires id, from, and to", i)
		}
		if _, duplicate := edgeIDs[id]; duplicate {
			return fmt.Errorf("duplicate edge id %q", id)
		}
		edgeIDs[id] = struct{}{}
		if _, ok := AllowedEdgeKinds[kind]; !ok {
			return fmt.Errorf("edges[%d].kind %q is not allowlisted (ADR-023)", i, kind)
		}
		if _, ok := nodeIDs[from]; !ok {
			return fmt.Errorf("edges[%d].from references unknown node %q", i, from)
		}
		if _, ok := nodeIDs[to]; !ok {
			return fmt.Errorf("edges[%d].to references unknown node %q", i, to)
		}
		if weight, exists := edge["weight"]; exists && weight != nil {
			if _, ok := weight.(float64); !ok {
				return fmt.Errorf("edges[%d].weight must be a number", i)
			}
		}
	}

	bindingObjects := map[string]string{}
	if bindingsRaw, exists := doc["dataBindings"]; exists && bindingsRaw != nil {
		bindings, ok := bindingsRaw.([]any)
		if !ok {
			return fmt.Errorf("dataBindings must be an array")
		}
		seen := map[string]struct{}{}
		for i, rawBinding := range bindings {
			binding, ok := rawBinding.(map[string]any)
			if !ok {
				return fmt.Errorf("dataBindings[%d] must be an object", i)
			}
			if err := allowedKeys(fmt.Sprintf("dataBindings[%d]", i), binding, "id", "objectApiName", "fields", "filters", "sort", "limit"); err != nil {
				return err
			}
			id, _ := binding["id"].(string)
			objectAPIName, _ := binding["objectApiName"].(string)
			if strings.TrimSpace(id) == "" || strings.TrimSpace(objectAPIName) == "" {
				return fmt.Errorf("dataBindings[%d] requires id and objectApiName", i)
			}
			if _, duplicate := seen[id]; duplicate {
				return fmt.Errorf("duplicate data binding id %q", id)
			}
			seen[id] = struct{}{}
			bindingObjects[id] = objectAPIName
			if fields, exists := binding["fields"]; exists && fields != nil && !isStringArray(fields) {
				return fmt.Errorf("dataBindings[%d].fields must be a string array", i)
			}
			if limit, exists := binding["limit"]; exists && limit != nil {
				n, ok := limit.(float64)
				if !ok || n < 0 || n != float64(int(n)) {
					return fmt.Errorf("dataBindings[%d].limit must be a non-negative integer", i)
				}
			}
		}
	}
	if err := validateCollectionBindings(nodes, bindingObjects); err != nil {
		return err
	}
	if viewportRaw, exists := doc["viewport"]; exists && viewportRaw != nil {
		if err := validateNumericObject("viewport", viewportRaw, []string{"x", "y", "zoom"}, []string{"x", "y", "zoom"}); err != nil {
			return err
		}
	}
	if lensesRaw, exists := doc["lenses"]; exists && lensesRaw != nil {
		if err := validateLenses(lensesRaw); err != nil {
			return err
		}
	}
	return nil
}

func validateNode(index int, node map[string]any) error {
	path := fmt.Sprintf("nodes[%d]", index)
	if err := allowedKeys(path, node, "id", "kind", "ref", "toolRef", "layout", "cardProjection", "label", "text", "proposalId", "bindingId", "searchQ"); err != nil {
		return err
	}
	id, _ := node["id"].(string)
	kind, _ := node["kind"].(string)
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s.id is required", path)
	}
	if _, ok := AllowedNodeKinds[kind]; !ok {
		return fmt.Errorf("%s.kind %q is not allowlisted (ADR-023)", path, kind)
	}
	if layout, exists := node["layout"]; exists && layout != nil {
		if err := validateNumericObject(path+".layout", layout, []string{"x", "y", "w", "z"}, []string{"x", "y"}); err != nil {
			return err
		}
	}
	if projection, exists := node["cardProjection"]; exists && projection != nil && !isStringArray(projection) {
		return fmt.Errorf("%s.cardProjection must be a string array", path)
	}
	switch kind {
	case "record":
		if err := validateRef(path+".ref", node["ref"], "objectApiName", "recordId"); err != nil {
			return err
		}
		ref := node["ref"].(map[string]any)
		if _, err := uuid.Parse(strings.TrimSpace(ref["recordId"].(string))); err != nil {
			return fmt.Errorf("%s.ref.recordId must be a UUID", path)
		}
	case "cluster":
		if label, _ := node["label"].(string); strings.TrimSpace(label) == "" {
			return fmt.Errorf("%s.label is required", path)
		}
	case "tool":
		toolRef, ok := node["toolRef"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s.toolRef is required", path)
		}
		if err := allowedKeys(path+".toolRef", toolRef, "toolSpecApiName", "workingToolId"); err != nil {
			return err
		}
		spec, _ := toolRef["toolSpecApiName"].(string)
		working, _ := toolRef["workingToolId"].(string)
		if (strings.TrimSpace(spec) == "") == (strings.TrimSpace(working) == "") {
			return fmt.Errorf("%s.toolRef requires exactly one of toolSpecApiName or workingToolId", path)
		}
	case "insight", "question":
		if text, _ := node["text"].(string); strings.TrimSpace(text) == "" {
			return fmt.Errorf("%s.text is required", path)
		}
	case "proposal":
		if value, _ := node["proposalId"].(string); strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s.proposalId is required", path)
		}
	case "signal":
		if value, _ := node["bindingId"].(string); strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s.bindingId is required", path)
		}
	case "person":
		ref, ok := node["ref"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s.ref is required", path)
		}
		if err := allowedKeys(path+".ref", ref, "principalId", "contactRecordId"); err != nil {
			return err
		}
		principal, _ := ref["principalId"].(string)
		contact, _ := ref["contactRecordId"].(string)
		if strings.TrimSpace(principal) == "" && strings.TrimSpace(contact) == "" {
			return fmt.Errorf("%s.ref requires principalId or contactRecordId", path)
		}
	case "collection":
		if err := validateRef(path+".ref", node["ref"], "objectApiName"); err != nil {
			return err
		}
		ref := node["ref"].(map[string]any)
		if _, hasRecordID := ref["recordId"]; hasRecordID {
			return fmt.Errorf("%s.ref.recordId is not allowed on collection nodes", path)
		}
		if searchQ, exists := node["searchQ"]; exists && searchQ != nil {
			value, ok := searchQ.(string)
			if !ok {
				return fmt.Errorf("%s.searchQ must be a string", path)
			}
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s.searchQ must be non-empty when set", path)
			}
		}
	}
	return nil
}

func validateCollectionBindings(nodes []any, bindingObjects map[string]string) error {
	for i, rawNode := range nodes {
		node, ok := rawNode.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := node["kind"].(string)
		if kind != "collection" {
			continue
		}
		bindingID, _ := node["bindingId"].(string)
		if strings.TrimSpace(bindingID) == "" {
			continue
		}
		objectAPIName, ok := bindingObjects[bindingID]
		if !ok {
			return fmt.Errorf("nodes[%d].bindingId %q does not reference a dataBinding", i, bindingID)
		}
		ref, _ := node["ref"].(map[string]any)
		refObject, _ := ref["objectApiName"].(string)
		if strings.TrimSpace(refObject) != strings.TrimSpace(objectAPIName) {
			return fmt.Errorf("nodes[%d].bindingId object %q does not match collection %q", i, objectAPIName, refObject)
		}
	}
	return nil
}

func validateRef(path string, raw any, required ...string) error {
	ref, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("%s is required", path)
	}
	if err := allowedKeys(path, ref, required...); err != nil {
		return err
	}
	for _, key := range required {
		value, _ := ref[key].(string)
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s.%s is required", path, key)
		}
	}
	return nil
}

func validateNumericObject(path string, raw any, allowed, required []string) error {
	obj, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("%s must be an object", path)
	}
	if err := allowedKeys(path, obj, allowed...); err != nil {
		return err
	}
	for _, key := range required {
		if _, ok := obj[key].(float64); !ok {
			return fmt.Errorf("%s.%s must be a number", path, key)
		}
	}
	for _, key := range allowed {
		if value, exists := obj[key]; exists && value != nil {
			if _, ok := value.(float64); !ok {
				return fmt.Errorf("%s.%s must be a number", path, key)
			}
		}
	}
	return nil
}

func validateLenses(raw any) error {
	lenses, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("lenses must be an array")
	}
	for i, item := range lenses {
		lens, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("lenses[%d] must be an object", i)
		}
		if err := allowedKeys(fmt.Sprintf("lenses[%d]", i), lens, "id", "label", "filter"); err != nil {
			return err
		}
		id, _ := lens["id"].(string)
		label, _ := lens["label"].(string)
		if strings.TrimSpace(id) == "" || strings.TrimSpace(label) == "" {
			return fmt.Errorf("lenses[%d] requires id and label", i)
		}
		filter, ok := lens["filter"].(map[string]any)
		if !ok {
			return fmt.Errorf("lenses[%d].filter must be an object", i)
		}
		if err := allowedKeys(fmt.Sprintf("lenses[%d].filter", i), filter, "cluster", "kind", "kinds", "edgeKind", "edgeKinds", "nodeIds", "watching", "minWeight"); err != nil {
			return err
		}
	}
	return nil
}

func allowedKeys(path string, obj map[string]any, allowed ...string) error {
	set := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		set[key] = struct{}{}
	}
	for key := range obj {
		if _, ok := set[key]; ok {
			continue
		}
		if _, sanitizable := bakedPayloadKeys[key]; sanitizable {
			continue
		}
		return fmt.Errorf("%s.%s is not allowed", path, key)
	}
	return nil
}

func isStringArray(raw any) bool {
	items, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return false
		}
	}
	return true
}
