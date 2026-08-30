package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/MajestaNet/ide/internal/seed"
	"github.com/MajestaNet/ide/internal/testutil"
)

func TestActivityFeedComposesActivities(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ctx := t.Context()

	if _, err := seed.EnablePackage(ctx, d.Meta, "activities"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = seed.DisablePackage(ctx, d.Meta, "activities")
		_, _ = d.Pool.Exec(ctx, `DELETE FROM records WHERE object_api_name IN ('Task','Appointment','PhoneCall','Email','Account')`)
	})

	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{APIKeys: "admin-key+admin"})

	rr := testutil.AuthRequest(ts.Handler, http.MethodPost, "/client/v1/sobjects/Account", "admin-key", map[string]any{
		"Name": "Feed Parent Co",
	})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("create account status=%d body=%s", rr.Code, rr.Body.String())
	}
	var acct map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &acct)
	acctID, _ := acct["Id"].(string)
	if acctID == "" {
		t.Fatalf("account=%v", acct)
	}

	rr = testutil.AuthRequest(ts.Handler, http.MethodPost, "/client/v1/sobjects/Task", "admin-key", map[string]any{
		"Subject":            "Follow up",
		"Status":             "Open",
		"RegardingAccountId": acctID,
	})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("create task status=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = testutil.AuthRequest(ts.Handler, http.MethodGet,
		"/client/v1/activity-feed?parentType=Account&parentId="+acctID, "admin-key", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("feed status=%d body=%s", rr.Code, rr.Body.String())
	}
	var feed struct {
		Items []struct {
			Kind          string `json:"kind"`
			ObjectAPIName string `json:"objectApiName"`
			Subject       string `json:"subject"`
		} `json:"items"`
		TotalSize int `json:"totalSize"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &feed); err != nil {
		t.Fatal(err)
	}
	if feed.TotalSize < 1 {
		t.Fatalf("expected Task in feed, got %+v", feed)
	}
	sawTask := false
	for _, it := range feed.Items {
		if it.Kind == "activity" && it.ObjectAPIName == "Task" && it.Subject == "Follow up" {
			sawTask = true
		}
	}
	if !sawTask {
		t.Fatalf("missing Task in feed: items=%+v", feed.Items)
	}
}

func TestActivityFeedRequiresParentParams(t *testing.T) {
	d := testutil.RequireDatabase(t)
	testutil.BootstrapCore(t, d, testutil.BootstrapOptions{})
	ts := testutil.NewTestServer(t, d, testutil.ServerOptions{APIKeys: "admin-key+admin"})

	rr := testutil.AuthRequest(ts.Handler, http.MethodGet, "/client/v1/activity-feed", "admin-key", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}
