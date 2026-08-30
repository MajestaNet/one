package dataengine

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func lookupStr(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	v, ok := data[key]
	if !ok || v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func requiresCommercialParty(object string) bool {
	switch object {
	case "Opportunity", "Quote", "Order":
		return true
	default:
		return false
	}
}

func applyCommercialCreateDefaults(objectAPIName string, input map[string]any) {
	if objectAPIName != "Order" || input == nil {
		return
	}
	if lookupStr(input, "Status") == "" {
		input["Status"] = "Draft"
	}
}

func validateCommercialParty(object string, data map[string]any) error {
	if !requiresCommercialParty(object) {
		return nil
	}
	if lookupStr(data, "AccountId") != "" || lookupStr(data, "ContactId") != "" {
		return nil
	}
	return validationErrorf("%s requires AccountId and/or ContactId", object)
}

func validateOrderLineMutable(ctx context.Context, s *Service, data map[string]any) error {
	orderID := lookupStr(data, "OrderId")
	if orderID == "" {
		return nil
	}
	parent, err := s.Get(ctx, "Order", orderID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return validationErrorf("parent Order %s not found", orderID)
		}
		return fmt.Errorf("load parent Order: %w", err)
	}
	status := lookupStr(parent, "Status")
	if status == "" || status == "Draft" {
		return nil
	}
	return validationErrorf("OrderLine cannot be changed unless parent Order Status is Draft (got %s)", status)
}

func (s *Service) validateCommercialWrite(ctx context.Context, object string, data map[string]any, op string) error {
	if err := validateCommercialParty(object, data); err != nil {
		return err
	}
	if object != "OrderLine" {
		return nil
	}
	if op != "create" && op != "update" && op != "delete" {
		return nil
	}
	return validateOrderLineMutable(ctx, s, data)
}
