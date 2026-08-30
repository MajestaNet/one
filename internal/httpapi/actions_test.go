package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/MajestaNet/ide/internal/seed"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestPlatformActionsCatalogShell(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	_, _ = seed.DisablePackage(ctx, d.Meta, "billing")
	_, _ = seed.DisablePackage(ctx, d.Meta, "sales")
	_, _ = seed.DisablePackage(ctx, d.Meta, "catalog")
	_, _ = seed.DisablePackage(ctx, d.Meta, "lead_marketing")

	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,client-key:client",
	})

	rr := testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/actions", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
	}
	var listed map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &listed)
	for _, raw := range asSlice(listed["actions"]) {
		item, _ := raw.(map[string]any)
		if item["apiName"] == "lead.convert" {
			t.Fatal("catalog must omit lead.convert while lead_marketing is disabled")
		}
		if item["apiName"] == "quote.accept" {
			t.Fatal("catalog must omit quote.accept while sales is disabled")
		}
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/actions/no.such", "admin-key", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown get: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "ACTION_NOT_FOUND")

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/no.such", "admin-key", map[string]any{"input": map[string]any{}})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown post: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "ACTION_NOT_FOUND")

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/actions/lead.convert", "admin-key", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("disabled get: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "PACKAGE_NOT_ENABLED")

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/actions/quote.accept", "admin-key", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("quote.accept disabled get: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "PACKAGE_NOT_ENABLED")

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/lead.convert", "admin-key", map[string]any{
		"input": map[string]any{"leadId": "00000000-0000-4000-8000-000000000099"},
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("disabled post: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "PACKAGE_NOT_ENABLED")

	// Flat /v1 alias must not register actions.
	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/v1/actions", "admin-key", nil)
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("flat alias should not serve actions: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/metadata/v1/packages/lead_marketing", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("package status: %d %s", rr.Code, rr.Body.String())
	}
	var pkg map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &pkg)
	found := false
	for _, raw := range asSlice(pkg["actionApiNames"]) {
		if raw == "lead.convert" {
			found = true
		}
	}
	if !found {
		t.Fatalf("metadata package should declare lead.convert even when disabled: %s", rr.Body.String())
	}
	// Catalog objects (including lookups) ship even when the package is not enabled.
	objs, _ := pkg["objects"].([]any)
	var lead map[string]any
	for _, raw := range objs {
		row, _ := raw.(map[string]any)
		if row["apiName"] == "Lead" {
			lead = row
			break
		}
	}
	if lead == nil {
		t.Fatalf("disabled package should still list Lead in objects: %s", rr.Body.String())
	}
	foundLookup := false
	for _, raw := range asSlice(lead["fields"]) {
		field, _ := raw.(map[string]any)
		if field["apiName"] == "AccountId" && field["referenceTo"] == "Account" {
			foundLookup = true
		}
	}
	if !foundLookup {
		t.Fatalf("Lead.AccountId catalog lookup missing: %s", rr.Body.String())
	}
}

func TestLeadConvertHappyPathAndGates(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	if _, err := seed.EnablePackage(ctx, d.Meta, "lead_marketing"); err != nil {
		t.Fatalf("enable lead_marketing: %v", err)
	}
	_, _ = seed.DisablePackage(ctx, d.Meta, "sales")
	t.Cleanup(func() {
		_, _ = seed.DisablePackage(ctx, d.Meta, "lead_marketing")
		_, _ = seed.DisablePackage(ctx, d.Meta, "sales")
	})
	// Shared-DB suites (e.g. dataengine sync guest tests) may leave Lead triggers.
	if _, err := d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE object_api_name='Lead'`); err != nil {
		t.Fatalf("disable leftover Lead automations: %v", err)
	}
	_, _ = d.Pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,client-key:client",
	})

	rr := testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/actions", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list enabled: %d %s", rr.Code, rr.Body.String())
	}
	var listed map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &listed)
	found := false
	for _, raw := range asSlice(listed["actions"]) {
		item, _ := raw.(map[string]any)
		if item["apiName"] == "lead.convert" {
			found = true
			if item["syncSafe"] != true {
				t.Fatalf("syncSafe=%v", item["syncSafe"])
			}
		}
	}
	if !found {
		t.Fatalf("catalog missing lead.convert: %s", rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/actions/lead.convert", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("describe: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/sobjects/Lead", "admin-key", map[string]any{
		"LastName": "Avery", "Company": "Acme Convert", "Email": "avery@example.com", "Status": "New",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create lead: %d %s", rr.Code, rr.Body.String())
	}
	var lead map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &lead)
	leadID, _ := lead["Id"].(string)

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/lead.convert", "admin-key", map[string]any{
		"input": map[string]any{"leadId": leadID, "createOpportunity": true},
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("createOpportunity without sales: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "PACKAGE_NOT_ENABLED")

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/lead.convert", "admin-key", map[string]any{
		"input": map[string]any{"leadId": leadID},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("convert: %d %s", rr.Code, rr.Body.String())
	}
	var converted map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &converted)
	if converted["alreadyConverted"] != false || converted["accountId"] == nil || converted["contactId"] == nil {
		t.Fatalf("convert result=%v", converted)
	}
	accountID, _ := converted["accountId"].(string)
	contactID, _ := converted["contactId"].(string)

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/sobjects/Lead/"+leadID, "admin-key", nil)
	var updatedLead map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &updatedLead)
	if updatedLead["Status"] != "Converted" {
		t.Fatalf("lead status=%v", updatedLead["Status"])
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/sobjects/Account/"+accountID, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("account: %d %s", rr.Code, rr.Body.String())
	}
	var acct map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &acct)
	if acct["Name"] != "Acme Convert" {
		t.Fatalf("account name=%v", acct["Name"])
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/sobjects/Contact/"+contactID, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("contact: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/lead.convert", "admin-key", map[string]any{
		"input": map[string]any{"leadId": leadID},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("idempotent: %d %s", rr.Code, rr.Body.String())
	}
	var again map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &again)
	if again["alreadyConverted"] != true || again["accountId"] != accountID {
		t.Fatalf("already converted=%v", again)
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/sobjects/Lead", "admin-key", map[string]any{
		"LastName": "NoCo", "Status": "New",
	})
	var noCo map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &noCo)
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/lead.convert", "admin-key", map[string]any{
		"input": map[string]any{"leadId": noCo["Id"]},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("derived account name: %d %s", rr.Code, rr.Body.String())
	}
	var derived map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &derived)
	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/sobjects/Account/"+derived["accountId"].(string), "admin-key", nil)
	var derivedAcct map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &derivedAcct)
	if derivedAcct["Name"] != "NoCo" {
		t.Fatalf("derived account name=%v", derivedAcct["Name"])
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/lead.convert", "admin-key", map[string]any{
		"input": map[string]any{"leadId": leadID, "convertedStatus": "Qualified"},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad convertedStatus: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "VALIDATION_FAILED")

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/lead.convert", "client-key", map[string]any{
		"input": map[string]any{"leadId": leadID},
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("create/FLS deny: %d %s", rr.Code, rr.Body.String())
	}

	acctRR := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/sobjects/Account", "admin-key", map[string]any{"Name": "Existing Co"})
	var existingAcct map[string]any
	_ = json.Unmarshal(acctRR.Body.Bytes(), &existingAcct)
	contactRR := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/sobjects/Contact", "admin-key", map[string]any{
		"LastName": "Existing", "AccountId": existingAcct["Id"],
	})
	var existingContact map[string]any
	_ = json.Unmarshal(contactRR.Body.Bytes(), &existingContact)
	leadRR := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/sobjects/Lead", "admin-key", map[string]any{
		"LastName": "LinkMe", "Company": "Skip Create", "Status": "Qualified",
	})
	var linkLead map[string]any
	_ = json.Unmarshal(leadRR.Body.Bytes(), &linkLead)
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/lead.convert", "admin-key", map[string]any{
		"input": map[string]any{
			"leadId":    linkLead["Id"],
			"accountId": existingAcct["Id"],
			"contactId": existingContact["Id"],
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("existing party: %d %s", rr.Code, rr.Body.String())
	}
	var linked map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &linked)
	if linked["accountId"] != existingAcct["Id"] || linked["contactId"] != existingContact["Id"] {
		t.Fatalf("linked=%v", linked)
	}

	if _, err := seed.EnablePackage(ctx, d.Meta, "catalog"); err != nil {
		t.Fatalf("enable catalog: %v", err)
	}
	if _, err := seed.EnablePackage(ctx, d.Meta, "sales"); err != nil {
		t.Fatalf("enable sales: %v", err)
	}
	oppLead := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/sobjects/Lead", "admin-key", map[string]any{
		"LastName": "Opp", "Company": "Opp Co", "Status": "Qualified", "Source": "Web",
	})
	var oppLeadBody map[string]any
	_ = json.Unmarshal(oppLead.Body.Bytes(), &oppLeadBody)
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/lead.convert", "admin-key", map[string]any{
		"input": map[string]any{"leadId": oppLeadBody["Id"], "createOpportunity": true},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("convert+opp: %d %s", rr.Code, rr.Body.String())
	}
	var withOpp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &withOpp)
	oppID, _ := withOpp["opportunityId"].(string)
	if oppID == "" {
		t.Fatalf("missing opportunityId: %v", withOpp)
	}
	rr = testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/sobjects/Opportunity/"+oppID, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("opportunity: %d %s", rr.Code, rr.Body.String())
	}
	var opp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &opp)
	if opp["StageName"] != "Prospecting" {
		t.Fatalf("stage=%v", opp["StageName"])
	}
	if opp["AccountId"] == nil && opp["ContactId"] == nil {
		t.Fatal("opportunity missing party")
	}
}

func TestQuoteAcceptHappyPathAndGates(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()
	if _, err := seed.EnablePackage(ctx, d.Meta, "catalog"); err != nil {
		t.Fatalf("enable catalog: %v", err)
	}
	if _, err := seed.EnablePackage(ctx, d.Meta, "sales"); err != nil {
		t.Fatalf("enable sales: %v", err)
	}
	_, _ = seed.DisablePackage(ctx, d.Meta, "billing")
	t.Cleanup(func() {
		_, _ = seed.DisablePackage(ctx, d.Meta, "billing")
		_, _ = seed.DisablePackage(ctx, d.Meta, "sales")
		_, _ = seed.DisablePackage(ctx, d.Meta, "catalog")
	})
	if _, err := d.Pool.Exec(ctx, `UPDATE metadata_automations SET active=false WHERE object_api_name = ANY($1::text[])`,
		[]string{"Quote", "QuoteLine", "Order", "OrderLine", "Opportunity", "Account", "Contact", "Product"}); err != nil {
		t.Fatalf("disable leftover automations: %v", err)
	}
	_, _ = d.Pool.Exec(ctx, `UPDATE metadata_cache_epoch SET epoch = epoch + 1 WHERE id = 1`)

	srv := testutil.NewTestServer(t, d, testutil.ServerOptions{
		APIKeys: "admin-key+admin,client-key:client",
	})

	rr := testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/actions/quote.accept", "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("describe: %d %s", rr.Code, rr.Body.String())
	}

	acct := mustCreateSObject(t, srv, "Account", map[string]any{
		"Name": "Accept Co", "BillingStreet": "1 Main", "BillingCity": "Austin", "BillingState": "TX",
		"BillingPostalCode": "78701", "BillingCountry": "US",
	})
	product := mustCreateSObject(t, srv, "Product", map[string]any{"Name": "Widget", "ProductType": "Good"})

	quoteBody := map[string]any{
		"Name": "Q-Accept", "Status": "Draft", "AccountId": acct["Id"],
		"CurrencyCode": "USD", "Subtotal": 100.0, "TaxAmount": 8.0, "ShippingAmount": 7.0, "TotalAmount": 115.0,
		"BillingStreet": "1 Main", "BillingCity": "Austin", "BillingState": "TX",
		"BillingPostalCode": "78701", "BillingCountry": "US",
		"ShippingStreet": "2 Dock", "ShippingCity": "Houston", "ShippingState": "TX",
		"ShippingPostalCode": "77002", "ShippingCountry": "US",
		"Description": "commercial snapshot",
	}
	quoteA := mustCreateSObject(t, srv, "Quote", quoteBody)
	lineA := mustCreateSObject(t, srv, "QuoteLine", map[string]any{
		"QuoteId": quoteA["Id"], "ProductId": product["Id"], "LineNumber": 1.0,
		"Quantity": 2.0, "ListPrice": 50.0, "UnitPrice": 50.0, "Amount": 100.0,
		"Description": "two widgets", "PriceSource": "Manual",
	})

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/quote.accept", "admin-key", map[string]any{
		"input": map[string]any{"quoteId": quoteA["Id"]},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("accept without billing: %d %s", rr.Code, rr.Body.String())
	}
	var accepted map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &accepted)
	if accepted["alreadyAccepted"] != false || accepted["orderId"] != nil {
		t.Fatalf("status-only accept=%v", accepted)
	}
	gotQuote := mustGetSObject(t, srv, "Quote", quoteA["Id"].(string))
	if gotQuote["Status"] != "Accepted" || gotQuote["AcceptedAt"] == nil {
		t.Fatalf("quote after accept=%v", gotQuote)
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/quote.accept", "admin-key", map[string]any{
		"input": map[string]any{"quoteId": quoteA["Id"]},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("idempotent without order: %d %s", rr.Code, rr.Body.String())
	}
	var again map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &again)
	if again["alreadyAccepted"] != true || again["orderId"] != nil {
		t.Fatalf("already accepted without order=%v", again)
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/quote.accept", "admin-key", map[string]any{
		"input": map[string]any{"quoteId": quoteA["Id"], "createOrder": true},
	})
	if rr.Code != http.StatusConflict {
		t.Fatalf("createOrder without billing: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "PACKAGE_NOT_ENABLED")
	var packErr map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &packErr)
	details, _ := packErr["details"].(map[string]any)
	if details["packageName"] != "billing" || details["option"] != "createOrder" {
		t.Fatalf("details=%v", details)
	}

	if _, err := seed.EnablePackage(ctx, d.Meta, "billing"); err != nil {
		t.Fatalf("enable billing: %v", err)
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/quote.accept", "admin-key", map[string]any{
		"input": map[string]any{"quoteId": quoteA["Id"]},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("upgrade createOrder: %d %s", rr.Code, rr.Body.String())
	}
	var upgraded map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &upgraded)
	orderID, _ := upgraded["orderId"].(string)
	if orderID == "" || upgraded["alreadyAccepted"] != true {
		t.Fatalf("upgrade=%v", upgraded)
	}

	order := mustGetSObject(t, srv, "Order", orderID)
	if order["Status"] != "Activated" || order["ActivatedAt"] == nil {
		t.Fatalf("order=%v", order)
	}
	if order["QuoteId"] != quoteA["Id"] {
		t.Fatalf("order.QuoteId=%v", order["QuoteId"])
	}
	on, _ := order["OrderNumber"].(string)
	if len(on) < 4 || on[:4] != "ORD-" {
		t.Fatalf("OrderNumber=%v", order["OrderNumber"])
	}
	for _, field := range []string{
		"AccountId", "CurrencyCode", "Subtotal", "TaxAmount", "ShippingAmount", "TotalAmount",
		"BillingStreet", "BillingCity", "BillingState", "BillingPostalCode", "BillingCountry",
		"ShippingStreet", "ShippingCity", "ShippingState", "ShippingPostalCode", "ShippingCountry",
		"Description",
	} {
		if fmtSprint(order[field]) != fmtSprint(quoteBody[field]) && fmtSprint(order[field]) != fmtSprint(gotQuote[field]) {
			t.Fatalf("snapshot %s order=%v quote=%v", field, order[field], gotQuote[field])
		}
	}

	gotQuote = mustGetSObject(t, srv, "Quote", quoteA["Id"].(string))
	if gotQuote["OrderId"] != orderID {
		t.Fatalf("Quote.OrderId=%v want %s", gotQuote["OrderId"], orderID)
	}

	qraw := map[string]any{
		"object":  "OrderLine",
		"filters": []map[string]any{{"field": "OrderId", "op": "eq", "value": orderID}},
		"limit":   10,
	}
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/query", "admin-key", qraw)
	if rr.Code != http.StatusOK {
		t.Fatalf("query lines: %d %s", rr.Code, rr.Body.String())
	}
	var qres map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &qres)
	olines := asSlice(qres["records"])
	if len(olines) != 1 {
		t.Fatalf("order lines=%v", qres)
	}
	oline, _ := olines[0].(map[string]any)
	if oline["QuoteLineId"] != lineA["Id"] || oline["ProductId"] != product["Id"] {
		t.Fatalf("order line refs=%v", oline)
	}
	for _, field := range []string{"Quantity", "ListPrice", "UnitPrice", "Amount", "Description", "PriceSource"} {
		if fmtSprint(oline[field]) != fmtSprint(lineA[field]) {
			t.Fatalf("line snapshot %s got=%v want=%v", field, oline[field], lineA[field])
		}
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/quote.accept", "admin-key", map[string]any{
		"input": map[string]any{"quoteId": quoteA["Id"]},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("idempotent with order: %d %s", rr.Code, rr.Body.String())
	}
	var idem map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &idem)
	if idem["alreadyAccepted"] != true || idem["orderId"] != orderID {
		t.Fatalf("idempotent=%v", idem)
	}

	olineID, _ := oline["Id"].(string)
	rr = testutil.AuthRequest(srv.Handler, http.MethodPatch, "/client/v1/sobjects/OrderLine/"+olineID, "admin-key", map[string]any{
		"Quantity": 9.0,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("activated line patch: %d %s", rr.Code, rr.Body.String())
	}

	quoteB := mustCreateSObject(t, srv, "Quote", map[string]any{
		"Name": "Q-NoOrder", "Status": "Presented", "AccountId": acct["Id"], "TotalAmount": 10.0,
	})
	mustCreateSObject(t, srv, "QuoteLine", map[string]any{
		"QuoteId": quoteB["Id"], "ProductId": product["Id"], "Quantity": 1.0, "Amount": 10.0,
	})
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/quote.accept", "admin-key", map[string]any{
		"input": map[string]any{"quoteId": quoteB["Id"], "createOrder": false},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("explicit no order: %d %s", rr.Code, rr.Body.String())
	}
	var noOrd map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &noOrd)
	if noOrd["orderId"] != nil {
		t.Fatalf("createOrder false=%v", noOrd)
	}

	quoteNone := mustCreateSObject(t, srv, "Quote", map[string]any{
		"Name": "Q-Empty", "Status": "Draft", "AccountId": acct["Id"],
	})
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/quote.accept", "admin-key", map[string]any{
		"input": map[string]any{"quoteId": quoteNone["Id"], "createOrder": false},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("no lines: %d %s", rr.Code, rr.Body.String())
	}
	assertErrorCode(t, rr.Body.Bytes(), "VALIDATION_FAILED")

	quoteExp := mustCreateSObject(t, srv, "Quote", map[string]any{
		"Name": "Q-Exp", "Status": "Expired", "AccountId": acct["Id"],
	})
	mustCreateSObject(t, srv, "QuoteLine", map[string]any{
		"QuoteId": quoteExp["Id"], "ProductId": product["Id"], "Quantity": 1.0,
	})
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/quote.accept", "admin-key", map[string]any{
		"input": map[string]any{"quoteId": quoteExp["Id"], "createOrder": false},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expired: %d %s", rr.Code, rr.Body.String())
	}

	quoteParty := mustCreateSObject(t, srv, "Quote", map[string]any{
		"Name": "Q-Party", "Status": "Draft", "AccountId": acct["Id"],
	})
	mustCreateSObject(t, srv, "QuoteLine", map[string]any{
		"QuoteId": quoteParty["Id"], "ProductId": product["Id"], "Quantity": 1.0,
	})
	if _, err := d.Pool.Exec(ctx, `UPDATE records SET data = data - 'AccountId' - 'ContactId' WHERE id = $1::uuid`, quoteParty["Id"]); err != nil {
		t.Fatalf("clear party: %v", err)
	}
	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/quote.accept", "admin-key", map[string]any{
		"input": map[string]any{"quoteId": quoteParty["Id"], "createOrder": false},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing party: %d %s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/actions/quote.accept", "client-key", map[string]any{
		"input": map[string]any{"quoteId": quoteA["Id"]},
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("FLS deny: %d %s", rr.Code, rr.Body.String())
	}
}

func mustCreateSObject(t *testing.T, srv *testutil.TestServer, object string, body map[string]any) map[string]any {
	t.Helper()
	rr := testutil.AuthRequest(srv.Handler, http.MethodPost, "/client/v1/sobjects/"+object, "admin-key", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %s: %d %s", object, rr.Code, rr.Body.String())
	}
	var rec map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &rec)
	return rec
}

func mustGetSObject(t *testing.T, srv *testutil.TestServer, object, id string) map[string]any {
	t.Helper()
	rr := testutil.AuthRequest(srv.Handler, http.MethodGet, "/client/v1/sobjects/"+object+"/"+id, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get %s/%s: %d %s", object, id, rr.Code, rr.Body.String())
	}
	var rec map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &rec)
	return rec
}

func fmtSprint(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(jsonNumber(v))
}

func jsonNumber(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return strings.Trim(string(b), `"`)
}

func assertErrorCode(t *testing.T, body []byte, want string) {
	t.Helper()
	var env map[string]any
	_ = json.Unmarshal(body, &env)
	if env["error"] != want {
		t.Fatalf("error code=%v want %s body=%s", env["error"], want, string(body))
	}
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	if s == nil {
		return []any{}
	}
	return s
}
