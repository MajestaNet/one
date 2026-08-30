package dataengine_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
	"github.com/MajestaNet/ide/internal/seed"
)

func TestCommercialPartyAndOrderLineFreeze(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.EnsureKernel(ctx); err != nil {
		t.Fatal(err)
	}
	meta := metadata.NewService(pool)
	if err := seed.Bootstrap(ctx, pool, meta, seed.Options{
		OwnerID:  "00000000-0000-4000-8000-000000000001",
		AutoSeed: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.EnablePackage(ctx, meta, "catalog"); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.EnablePackage(ctx, meta, "sales"); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.EnablePackage(ctx, meta, "billing"); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE object_api_name = ANY($1::text[])`,
		[]string{"Opportunity", "Quote", "QuoteLine", "Order", "OrderLine", "Account", "Contact", "Product"})
	_, _ = pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	svc := dataengine.NewService(pool, meta)
	actor := &authz.Actor{ID: "00000000-0000-4000-8000-000000000001", IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}

	_, err = svc.Create(ctx, "Opportunity", map[string]any{
		"Name": "No Party", "StageName": "Prospecting", "CloseDate": "2026-12-31",
	}, actor)
	if !isValidation(err) {
		t.Fatalf("opportunity without party: %v", err)
	}

	acct, err := svc.Create(ctx, "Account", map[string]any{"Name": "Commercial Co"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	acctID, _ := acct["Id"].(string)

	opp, err := svc.Create(ctx, "Opportunity", map[string]any{
		"Name": "With Party", "StageName": "Prospecting", "CloseDate": "2026-12-31", "AccountId": acctID,
	}, actor)
	if err != nil {
		t.Fatalf("opportunity with party: %v", err)
	}

	_, err = svc.Create(ctx, "Quote", map[string]any{
		"Name": "Q1", "Status": "Draft",
	}, actor)
	if !isValidation(err) {
		t.Fatalf("quote without party: %v", err)
	}

	quote, err := svc.Create(ctx, "Quote", map[string]any{
		"Name": "Q1", "Status": "Draft", "AccountId": acctID, "TotalAmount": 100.0,
	}, actor)
	if err != nil {
		t.Fatalf("quote with party: %v", err)
	}

	contact, err := svc.Create(ctx, "Contact", map[string]any{"LastName": "Solo"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "Order", map[string]any{
		"Name": "Contact only", "ContactId": contact["Id"],
	}, actor); err != nil {
		t.Fatalf("order contact-only party: %v", err)
	}

	_, err = svc.Create(ctx, "Order", map[string]any{"Name": "No party"}, actor)
	if !isValidation(err) {
		t.Fatalf("order without party: %v", err)
	}

	order, err := svc.Create(ctx, "Order", map[string]any{
		"Name": "Draft Order", "AccountId": acctID,
	}, actor)
	if err != nil {
		t.Fatalf("draft order: %v", err)
	}
	if order["Status"] != "Draft" {
		t.Fatalf("default Status=%v", order["Status"])
	}
	orderID, _ := order["Id"].(string)

	product, err := svc.Create(ctx, "Product", map[string]any{"Name": "Widget"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	productID, _ := product["Id"].(string)

	line, err := svc.Create(ctx, "OrderLine", map[string]any{
		"OrderId": orderID, "ProductId": productID, "Quantity": 2.0, "Amount": 20.0,
	}, actor)
	if err != nil {
		t.Fatalf("draft order line: %v", err)
	}
	lineID, _ := line["Id"].(string)
	if _, err := svc.Update(ctx, "OrderLine", lineID, map[string]any{"Quantity": 3.0}, actor); err != nil {
		t.Fatalf("update draft line: %v", err)
	}

	if _, err := svc.Update(ctx, "Order", orderID, map[string]any{"Status": "Activated"}, actor); err != nil {
		t.Fatalf("activate order: %v", err)
	}
	if _, err := svc.Update(ctx, "OrderLine", lineID, map[string]any{"Quantity": 4.0}, actor); !isValidation(err) {
		t.Fatalf("update activated line: %v", err)
	}
	if _, err := svc.Create(ctx, "OrderLine", map[string]any{
		"OrderId": orderID, "ProductId": productID, "Quantity": 1.0,
	}, actor); !isValidation(err) {
		t.Fatalf("create line on activated order: %v", err)
	}
	if err := svc.Delete(ctx, "OrderLine", lineID, actor); !isValidation(err) {
		t.Fatalf("delete activated line: %v", err)
	}

	_ = opp
	_ = quote
}

func isValidation(err error) bool {
	var ve *dataengine.ValidationError
	return errors.As(err, &ve)
}
