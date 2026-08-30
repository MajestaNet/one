package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
)

// activityFeedItem is one normalized row from GET /client/v1/activity-feed.
type activityFeedItem struct {
	Kind          string `json:"kind"`
	ObjectAPIName string `json:"objectApiName"`
	ID            string `json:"id"`
	OccurredAt    string `json:"occurredAt"`
	Subject       string `json:"subject,omitempty"`
	Summary       string `json:"summary,omitempty"`
	Channel       string `json:"channel,omitempty"`
	Direction     string `json:"direction,omitempty"`
	Status        string `json:"status,omitempty"`
}

var activityFeedObjects = []string{"Task", "Appointment", "PhoneCall", "Email"}

// handleActivityFeed composes optional activities for a parent record.
// Read-only view; writes stay on concrete record object apiNames.
func (s *Server) handleActivityFeed(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.meta == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	parentType := strings.TrimSpace(r.URL.Query().Get("parentType"))
	parentID := strings.TrimSpace(r.URL.Query().Get("parentId"))
	if parentType == "" || parentID == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "parentType and parentId are required")
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}

	actor := ActorFromContext(r.Context())
	viewAll, err := s.objectAz.GetViewAllObjects(r.Context(), actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	perSource := limit
	if perSource < 50 {
		perSource = 50
	}
	if perSource > 100 {
		perSource = 100
	}

	items := make([]activityFeedItem, 0, limit)
	items = append(items, s.activityFeedWorkItems(r.Context(), actor, viewAll, parentType, parentID, perSource)...)

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].OccurredAt > items[j].OccurredAt
	})
	if len(items) > limit {
		items = items[:limit]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"parentType": parentType,
		"parentId":   parentID,
		"items":      items,
		"totalSize":  len(items),
	})
}

func (s *Server) activityFeedWorkItems(
	ctx context.Context,
	actor *authz.Actor,
	viewAll map[string]struct{},
	parentType, parentID string,
	limit int,
) []activityFeedItem {
	regardingField := ""
	switch parentType {
	case "Account":
		regardingField = "RegardingAccountId"
	case "Contact":
		regardingField = "RegardingContactId"
	default:
		return nil
	}
	out := make([]activityFeedItem, 0)
	for _, apiName := range activityFeedObjects {
		if !s.objectExists(ctx, apiName) {
			continue
		}
		if err := s.objectAz.AssertObjectAccess(ctx, actor, apiName, authz.ActionRead); err != nil {
			continue
		}
		raw, _ := json.Marshal(map[string]any{
			"object": apiName,
			"filters": []map[string]any{
				{"field": regardingField, "op": "eq", "value": parentID},
			},
			"sort":  []map[string]any{{"field": "CreatedAt", "direction": "desc"}},
			"limit": limit,
		})
		for _, rec := range s.queryFeedObject(ctx, actor, viewAll, apiName, raw) {
			occurred := timeField(rec, "ScheduledStart")
			if occurred == "" {
				occurred = timeField(rec, "CreatedAt")
			}
			summary := stringField(rec, "Description")
			if summary == "" {
				summary = stringField(rec, "Status")
			}
			out = append(out, activityFeedItem{
				Kind:          "activity",
				ObjectAPIName: apiName,
				ID:            stringField(rec, "Id"),
				OccurredAt:    occurred,
				Subject:       stringField(rec, "Subject"),
				Summary:       summary,
				Direction:     stringField(rec, "Direction"),
				Status:        stringField(rec, "Status"),
			})
		}
	}
	return out
}

func (s *Server) objectExists(ctx context.Context, apiName string) bool {
	_, err := s.meta.GetObject(ctx, apiName)
	return err == nil
}

func (s *Server) queryFeedObject(
	ctx context.Context,
	actor *authz.Actor,
	viewAll map[string]struct{},
	objectAPIName string,
	raw []byte,
) []dataengine.SObjectRecord {
	vis, err := s.buildQueryVisibility(ctx, actor, objectAPIName, viewAll)
	if err != nil {
		return nil
	}
	result, err := s.data.Query(ctx, raw, vis)
	if err != nil {
		return nil
	}
	records := result.Records
	if s.fieldAz != nil {
		flsCtx := authz.ContextWithFLSCache(ctx)
		stripped := make([]dataengine.SObjectRecord, 0, len(records))
		for _, rec := range records {
			outRec, err := s.fieldAz.StripUnreadableFields(flsCtx, actor, objectAPIName, rec)
			if err != nil {
				continue
			}
			stripped = append(stripped, outRec)
		}
		records = stripped
	}
	return records
}

func stringField(rec dataengine.SObjectRecord, key string) string {
	v, ok := rec[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return strings.Trim(string(b), `"`)
	}
}

func timeField(rec dataengine.SObjectRecord, key string) string {
	v, ok := rec[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	default:
		return stringField(rec, key)
	}
}
