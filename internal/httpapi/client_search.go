package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil || s.meta == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	raw, err := readBodyLimited(r.Body, 1<<20)
	if err != nil {
		if requestBodyTooLarge(err) {
			writeErr(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request body too large")
			return
		}
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unable to read body")
		return
	}
	var req dataengine.SearchRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	named := len(req.Objects) > 0
	scopes, err := s.resolveSearchScopes(r.Context(), actor, req.Objects)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if named && len(scopes) == 0 {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "no searchable objects in request")
		return
	}
	result, err := s.data.Search(r.Context(), req, scopes)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	hits := result.Hits
	if s.fieldAz != nil {
		stripped := make([]dataengine.SearchHit, 0, len(hits))
		flsCtx := authz.ContextWithFLSCache(r.Context())
		for _, hit := range hits {
			out, err := s.stripSearchHitFLS(flsCtx, actor, hit)
			if err != nil {
				writeAPIError(w, err)
				return
			}
			stripped = append(stripped, out)
		}
		hits = stripped
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query": result.Query,
		"hits":  hits,
	})
}

func (s *Server) resolveSearchScopes(ctx context.Context, actor *authz.Actor, requested []string) ([]dataengine.SearchScope, error) {
	named := len(requested) > 0
	var names []string
	if named {
		for _, n := range requested {
			n = strings.TrimSpace(n)
			if n != "" {
				names = append(names, n)
			}
		}
		if len(names) == 0 {
			return nil, &dataengine.ValidationError{Message: "objects must not be empty"}
		}
	} else {
		listed, err := s.meta.ListObjects(ctx)
		if err != nil {
			return nil, err
		}
		for _, o := range listed {
			names = append(names, o.APIName)
		}
	}

	viewAll, err := s.objectAz.GetViewAllObjects(ctx, actor)
	if err != nil {
		return nil, err
	}

	var scopes []dataengine.SearchScope
	for _, name := range names {
		obj, err := s.meta.GetObject(ctx, name)
		if err != nil {
			if named && errors.Is(err, metadata.ErrNotFound) {
				return nil, &dataengine.ValidationError{Message: "unknown object: " + name}
			}
			if named {
				return nil, err
			}
			continue
		}
		if db.IsKernelStorage(obj.StorageMode) {
			if named {
				return nil, &dataengine.ValidationError{Message: name + " is not a flexible object"}
			}
			continue
		}
		fields, err := s.meta.GetFields(ctx, name)
		if err != nil {
			return nil, err
		}
		if !dataengine.HasSearchableField(fields) {
			if named {
				return nil, &dataengine.ValidationError{Message: "object has no searchable fields: " + name}
			}
			continue
		}
		if err := s.objectAz.AssertObjectAccess(ctx, actor, name, authz.ActionRead); err != nil {
			if named {
				return nil, &dataengine.ValidationError{Message: "cannot read object: " + name}
			}
			continue
		}
		vis, err := s.buildQueryVisibility(ctx, actor, name, viewAll)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, dataengine.SearchScope{
			ObjectAPIName: name,
			StorageMode:   obj.StorageMode,
			Visibility:    vis,
		})
	}
	return scopes, nil
}

func (s *Server) stripSearchHitFLS(ctx context.Context, actor *authz.Actor, hit dataengine.SearchHit) (dataengine.SearchHit, error) {
	titleReadable := false
	for _, field := range []string{"Name", "Subject", "Title", "FirstName", "LastName"} {
		ok, err := s.fieldAz.FieldReadable(ctx, actor, hit.Object, field)
		if err != nil {
			return hit, err
		}
		if ok {
			titleReadable = true
			break
		}
	}
	if !titleReadable {
		hit.Title = ""
	}
	subReadable := false
	for _, field := range []string{"Email", "Phone", "MobilePhone", "AccountNumber", "SerialNumber", "ProductCode"} {
		ok, err := s.fieldAz.FieldReadable(ctx, actor, hit.Object, field)
		if err != nil {
			return hit, err
		}
		if ok {
			subReadable = true
			break
		}
	}
	if !subReadable {
		hit.Subtitle = ""
	}
	return hit, nil
}
