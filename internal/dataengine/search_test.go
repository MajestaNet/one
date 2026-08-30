package dataengine_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

func TestBuildSearchDocumentPhoneDigits(t *testing.T) {
	fields := []metadata.FieldDefinition{
		{APIName: "Name", FieldType: metadata.FieldTypeText, Searchable: true},
		{APIName: "Phone", FieldType: metadata.FieldTypePhone, Searchable: true},
		{APIName: "Description", FieldType: metadata.FieldTypeTextarea, Searchable: false},
	}
	doc, title, subtitle := dataengine.BuildSearchDocument(fields, map[string]any{
		"Name":        "Acme",
		"Phone":       "(415) 555-0100",
		"Description": "secret",
	})
	if title != "Acme" {
		t.Fatalf("title=%q", title)
	}
	if subtitle != "(415) 555-0100" {
		t.Fatalf("subtitle=%q", subtitle)
	}
	if !strings.Contains(doc, "acme") || !strings.Contains(doc, "4155550100") {
		t.Fatalf("document=%q", doc)
	}
	if strings.Contains(doc, "secret") {
		t.Fatalf("textarea leaked into document: %q", doc)
	}
}

func TestNormalizeSearchQuery(t *testing.T) {
	n, _, err := dataengine.NormalizeSearchQuery("  ACM  ")
	if err != nil || n != "acm" {
		t.Fatalf("n=%q err=%v", n, err)
	}
	if _, _, err := dataengine.NormalizeSearchQuery("a"); err == nil {
		t.Fatal("expected short q error")
	}
	if _, _, err := dataengine.NormalizeSearchQuery("41"); err == nil {
		t.Fatal("expected short digits error")
	}
	if _, _, err := dataengine.NormalizeSearchQuery(""); err == nil {
		t.Fatal("expected empty q error")
	}
	if _, _, err := dataengine.NormalizeSearchQuery("%"); err == nil {
		t.Fatal("wildcard-only q should fail")
	}
	n, d, err := dataengine.NormalizeSearchQuery("415555")
	if err != nil || n != "415555" || d != "415555" {
		t.Fatalf("digits n=%q d=%q err=%v", n, d, err)
	}
}

func TestSearchNameEmailPhoneCrossObject(t *testing.T) {
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

	ownerID := "00000000-0000-4000-8000-000000000001"
	otherID := "00000000-0000-4000-8000-000000000077"
	store := db.NewUserStore(pool)
	if _, err := store.EnsureBootstrapAdmin(ctx, ownerID, "admin@one.local", "Admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnsureBootstrapAdmin(ctx, otherID, "other-search@one.local", "Other"); err != nil {
		t.Fatal(err)
	}

	acct := "SearchAcct" + time.Now().Format("150405")
	contact := "SearchCtc" + time.Now().Format("150405")
	hvObj := "SearchHV" + time.Now().Format("150405")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name = ANY($1)`, []string{acct, contact})
		_, _ = pool.Exec(ctx, `DELETE FROM records_hv WHERE object_api_name = $1`, hvObj)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name = ANY($1)`, []string{acct, contact, hvObj})
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name = ANY($1)`, []string{acct, contact, hvObj})
	})

	meta := metadata.NewService(pool)
	if _, err := meta.InsertObject(ctx, metadata.ObjectDefinition{
		APIName: acct, Label: "Search Account", PluralLabel: "Search Accounts", StorageMode: "flexible",
	}, metadata.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := meta.InsertObject(ctx, metadata.ObjectDefinition{
		APIName: contact, Label: "Search Contact", PluralLabel: "Search Contacts", StorageMode: "flexible",
	}, metadata.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := meta.InsertObject(ctx, metadata.ObjectDefinition{
		APIName: hvObj, Label: "Search HV", PluralLabel: "Search HV", StorageMode: db.StorageModeHighVolume,
	}, metadata.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []metadata.FieldDefinition{
		{ObjectAPIName: acct, APIName: "Name", Label: "Name", FieldType: "text", Searchable: true, Filterable: true},
		{ObjectAPIName: acct, APIName: "Phone", Label: "Phone", FieldType: "phone", Searchable: true, Filterable: true},
		{ObjectAPIName: contact, APIName: "FirstName", Label: "First", FieldType: "text", Searchable: true, Filterable: true},
		{ObjectAPIName: contact, APIName: "LastName", Label: "Last", FieldType: "text", Searchable: true, Filterable: true},
		{ObjectAPIName: contact, APIName: "Email", Label: "Email", FieldType: "email", Searchable: true, Filterable: true},
		{ObjectAPIName: hvObj, APIName: "Title", Label: "Title", FieldType: "text", Searchable: true, Filterable: true},
	} {
		if _, err := meta.InsertField(ctx, f, metadata.CreateOptions{}); err != nil {
			t.Fatalf("insert field %s.%s: %v", f.ObjectAPIName, f.APIName, err)
		}
	}

	svc := dataengine.NewService(pool, meta)
	actor := &authz.Actor{ID: ownerID, IsAdmin: true, Scopes: []authz.Scope{authz.ScopeClient}}
	if _, err := svc.Create(ctx, acct, map[string]any{"Name": "Acme SearchCo", "Phone": "(415) 555-0100"}, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, contact, map[string]any{"FirstName": "Jane", "LastName": "Doe", "Email": "jane@search.test"}, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, hvObj, map[string]any{"Title": "Note needle HV"}, actor); err != nil {
		t.Fatal(err)
	}

	scopes := []dataengine.SearchScope{
		{ObjectAPIName: acct, StorageMode: "flexible"},
		{ObjectAPIName: contact, StorageMode: "flexible"},
		{ObjectAPIName: hvObj, StorageMode: db.StorageModeHighVolume},
	}

	prefix, err := svc.Search(ctx, dataengine.SearchRequest{Query: "acm"}, scopes)
	if err != nil {
		t.Fatal(err)
	}
	if !searchHasObject(prefix, acct) || prefix.Hits[0].Title != "Acme SearchCo" {
		t.Fatalf("prefix hits=%#v", prefix.Hits)
	}

	email, err := svc.Search(ctx, dataengine.SearchRequest{Query: "jane@"}, scopes)
	if err != nil {
		t.Fatal(err)
	}
	if !searchHasObject(email, contact) {
		t.Fatalf("email hits=%#v", email.Hits)
	}

	phone, err := svc.Search(ctx, dataengine.SearchRequest{Query: "415555"}, scopes)
	if err != nil {
		t.Fatal(err)
	}
	if !searchHasObject(phone, acct) {
		t.Fatalf("phone hits=%#v", phone.Hits)
	}

	cross, err := svc.Search(ctx, dataengine.SearchRequest{Query: "search"}, scopes)
	if err != nil {
		t.Fatal(err)
	}
	if !searchHasObject(cross, acct) || !searchHasObject(cross, contact) {
		t.Fatalf("cross-object hits=%#v", cross.Hits)
	}

	filtered, err := svc.Search(ctx, dataengine.SearchRequest{Query: "search"}, []dataengine.SearchScope{
		{ObjectAPIName: contact, StorageMode: "flexible"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if searchHasObject(filtered, acct) || !searchHasObject(filtered, contact) {
		t.Fatalf("object filter hits=%#v", filtered.Hits)
	}

	hv, err := svc.Search(ctx, dataengine.SearchRequest{Query: "needle"}, scopes)
	if err != nil {
		t.Fatal(err)
	}
	if !searchHasObject(hv, hvObj) {
		t.Fatalf("hv hits=%#v", hv.Hits)
	}

	hidden := dataengine.QueryVisibility{Mode: dataengine.VisibilityOwnerCreator, UserID: otherID, HasObjectRead: true}
	denied, err := svc.Search(ctx, dataengine.SearchRequest{Query: "acm"}, []dataengine.SearchScope{
		{ObjectAPIName: acct, StorageMode: "flexible", Visibility: hidden},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(denied.Hits) != 0 {
		t.Fatalf("non-owner should not see hits, got %#v", denied.Hits)
	}

	if _, err := svc.Search(ctx, dataengine.SearchRequest{Query: "a"}, scopes); err == nil {
		t.Fatal("short q should fail")
	}
}

func searchHasObject(res *dataengine.SearchResult, object string) bool {
	if res == nil {
		return false
	}
	for _, h := range res.Hits {
		if h.Object == object {
			return true
		}
	}
	return false
}
