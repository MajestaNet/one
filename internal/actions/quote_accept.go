package actions

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
)

var quoteToOrderFields = []string{
	"AccountId", "ContactId", "OpportunityId", "PriceListId", "CurrencyCode",
	"Subtotal", "TaxAmount", "ShippingAmount", "TotalAmount",
	"BillingStreet", "BillingCity", "BillingState", "BillingPostalCode", "BillingCountry",
	"ShippingStreet", "ShippingCity", "ShippingState", "ShippingPostalCode", "ShippingCountry",
	"Description",
}

var quoteLineToOrderLineFields = []string{
	"ProductId", "PriceListEntryId", "UnitId", "LineNumber", "Quantity",
	"ListPrice", "UnitPrice", "DiscountPercent", "Amount", "Description", "PriceSource",
}

func acceptQuote(ctx context.Context, s *Service, actor *authz.Actor, input map[string]any) (map[string]any, error) {
	quoteID := strings.TrimSpace(strVal(input["quoteId"]))
	if quoteID == "" {
		return nil, errValidation("quoteId is required")
	}
	createOrder, err := s.resolveCreateOrder(ctx, input)
	if err != nil {
		return nil, err
	}

	if err := s.assertObject(ctx, actor, "Quote", authz.ActionRead); err != nil {
		return nil, err
	}
	quote, err := s.Data.Get(ctx, "Quote", quoteID)
	if err != nil {
		return nil, err
	}
	if err := s.assertViewRecord(ctx, actor, quote, "Quote"); err != nil {
		return nil, err
	}

	quoteRead := []string{
		"Status", "Name", "AccountId", "ContactId", "OpportunityId", "PriceListId", "CurrencyCode",
		"Subtotal", "TaxAmount", "ShippingAmount", "TotalAmount",
		"BillingStreet", "BillingCity", "BillingState", "BillingPostalCode", "BillingCountry",
		"ShippingStreet", "ShippingCity", "ShippingState", "ShippingPostalCode", "ShippingCountry",
		"Description",
	}
	enabled, err := s.enabledPackages(ctx)
	if err != nil {
		return nil, err
	}
	if enabled["billing"] {
		quoteRead = append(quoteRead, "OrderId")
	}
	if err := s.assertReadableFields(ctx, actor, "Quote", quoteRead...); err != nil {
		return nil, err
	}

	status := strings.TrimSpace(strVal(quote["Status"]))
	existingOrderID := strings.TrimSpace(strVal(quote["OrderId"]))
	alreadyAccepted := status == "Accepted"

	if alreadyAccepted {
		if !createOrder {
			return acceptedResult(quoteID, existingOrderID, true), nil
		}
		if existingOrderID != "" {
			return acceptedResult(quoteID, existingOrderID, true), nil
		}
	} else if status != "Draft" && status != "Presented" {
		return nil, errValidation("Quote Status " + status + " cannot be accepted")
	}

	if strings.TrimSpace(strVal(quote["AccountId"])) == "" && strings.TrimSpace(strVal(quote["ContactId"])) == "" {
		return nil, errValidation("Quote requires AccountId and/or ContactId")
	}

	if err := s.assertObject(ctx, actor, "QuoteLine", authz.ActionRead); err != nil {
		return nil, err
	}
	if err := s.assertReadableFields(ctx, actor, "QuoteLine", append([]string{"QuoteId"}, quoteLineToOrderLineFields...)...); err != nil {
		return nil, err
	}
	lines, err := s.listQuoteLines(ctx, quoteID)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, errValidation("Quote must have at least one QuoteLine")
	}

	if err := s.assertModifyRecord(ctx, actor, quote, "Quote"); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var orderID string
	if createOrder {
		orderID, err = s.snapshotOrderFromQuote(ctx, actor, quote, quoteID, lines, now)
		if err != nil {
			return nil, err
		}
	}

	quotePatch := map[string]any{}
	if !alreadyAccepted {
		quotePatch["Status"] = "Accepted"
		quotePatch["AcceptedAt"] = now.Format(time.RFC3339Nano)
	}
	if createOrder {
		quotePatch["OrderId"] = orderID
	}
	if len(quotePatch) > 0 {
		if err := s.assertEditable(ctx, actor, "Quote", quotePatch); err != nil {
			return nil, err
		}
		if err := dataengine.CountSyncMutation(ctx); err != nil {
			return nil, err
		}
		if _, err := s.Data.Update(ctx, "Quote", quoteID, quotePatch, actor); err != nil {
			return nil, err
		}
	}

	return acceptedResult(quoteID, orderID, alreadyAccepted), nil
}

func acceptedResult(quoteID, orderID string, already bool) map[string]any {
	out := map[string]any{
		"quoteId":         quoteID,
		"alreadyAccepted": already,
	}
	if orderID != "" {
		out["orderId"] = orderID
	}
	return out
}

func (s *Service) resolveCreateOrder(ctx context.Context, input map[string]any) (bool, error) {
	if _, ok := input["createOrder"]; ok {
		want := boolVal(input["createOrder"])
		if want {
			if err := s.requirePackage(ctx, "billing", "createOrder"); err != nil {
				return false, err
			}
		}
		return want, nil
	}
	enabled, err := s.enabledPackages(ctx)
	if err != nil {
		return false, err
	}
	return enabled["billing"], nil
}

func (s *Service) listQuoteLines(ctx context.Context, quoteID string) ([]dataengine.SObjectRecord, error) {
	raw, err := json.Marshal(map[string]any{
		"object": "QuoteLine",
		"filters": []map[string]any{
			{"field": "QuoteId", "op": "eq", "value": quoteID},
		},
		"sort":  []map[string]any{{"field": "LineNumber", "direction": "asc"}},
		"limit": 500,
	})
	if err != nil {
		return nil, err
	}
	res, err := s.Data.Query(ctx, raw, dataengine.QueryVisibility{})
	if err != nil {
		return nil, err
	}
	return res.Records, nil
}

func (s *Service) snapshotOrderFromQuote(
	ctx context.Context,
	actor *authz.Actor,
	quote map[string]any,
	quoteID string,
	lines []dataengine.SObjectRecord,
	now time.Time,
) (string, error) {
	if err := s.assertObject(ctx, actor, "Order", authz.ActionCreate); err != nil {
		return "", err
	}
	if err := s.assertObject(ctx, actor, "OrderLine", authz.ActionCreate); err != nil {
		return "", err
	}

	orderData := copyPresent(quote, quoteToOrderFields)
	name := strings.TrimSpace(strVal(quote["Name"]))
	if name == "" {
		name = "Order"
	}
	orderData["Name"] = name
	orderData["QuoteId"] = quoteID
	orderData["Status"] = "Draft"
	orderData["EffectiveDate"] = now.Format("2006-01-02")

	if err := s.assertEditable(ctx, actor, "Order", orderData); err != nil {
		return "", err
	}
	if err := dataengine.CountSyncMutation(ctx); err != nil {
		return "", err
	}
	order, err := s.Data.Create(ctx, "Order", orderData, actor)
	if err != nil {
		return "", err
	}
	orderID, _ := order["Id"].(string)
	if orderID == "" {
		return "", errValidation("Order create did not return Id")
	}

	for _, line := range lines {
		lineData := copyPresent(line, quoteLineToOrderLineFields)
		lineData["OrderId"] = orderID
		if lineID := strings.TrimSpace(strVal(line["Id"])); lineID != "" {
			lineData["QuoteLineId"] = lineID
		}
		if strings.TrimSpace(strVal(lineData["ProductId"])) == "" {
			return "", errValidation("QuoteLine.ProductId is required")
		}
		if _, ok := lineData["Quantity"]; !ok {
			return "", errValidation("QuoteLine.Quantity is required")
		}
		if err := s.assertEditable(ctx, actor, "OrderLine", lineData); err != nil {
			return "", err
		}
		if err := dataengine.CountSyncMutation(ctx); err != nil {
			return "", err
		}
		if _, err := s.Data.Create(ctx, "OrderLine", lineData, actor); err != nil {
			return "", err
		}
	}

	orderPatch := map[string]any{
		"Status":      "Activated",
		"ActivatedAt": now.Format(time.RFC3339Nano),
	}
	if err := s.assertEditable(ctx, actor, "Order", orderPatch); err != nil {
		return "", err
	}
	if err := dataengine.CountSyncMutation(ctx); err != nil {
		return "", err
	}
	if _, err := s.Data.Update(ctx, "Order", orderID, orderPatch, actor); err != nil {
		return "", err
	}
	return orderID, nil
}

func copyPresent(src map[string]any, keys []string) map[string]any {
	out := map[string]any{}
	if src == nil {
		return out
	}
	for _, k := range keys {
		v, ok := src[k]
		if !ok || v == nil {
			continue
		}
		if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
			continue
		}
		out[k] = v
	}
	return out
}
