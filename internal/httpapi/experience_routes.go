package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

func (s *Server) registerExperienceRoutes(prefix string) {
	meta := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, h))
	}
	capMeta := func(cap string, h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeMetadata, s.requireCapability(cap, h)))
	}
	s.mux.Handle("GET "+prefix+"/experiences", meta(s.handleListExperiences))
	s.mux.Handle("GET "+prefix+"/experiences/{apiName}", meta(s.handleGetExperience))
	s.mux.Handle("POST "+prefix+"/experiences", capMeta(authz.CapMetadataBuild, s.handleCreateExperience))
	s.mux.Handle("PATCH "+prefix+"/experiences/{apiName}", capMeta(authz.CapMetadataBuild, s.handlePatchExperience))
	s.mux.Handle("DELETE "+prefix+"/experiences/{apiName}", capMeta(authz.CapMetadataBuild, s.handleDeleteExperience))
}

func (s *Server) handleListExperiences(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	rows, err := pool.Query(r.Context(), `
SELECT api_name, label, description, home_url, connected_app_api_name, allowed_origins,
       active, ownership, package_name, created_at, updated_at
FROM metadata_experiences
WHERE ownership = 'custom'
ORDER BY api_name`)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		m, err := scanExperienceRow(rows)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		list = append(list, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"experiences": list})
}

func (s *Server) handleGetExperience(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	m, err := loadExperience(r.Context(), pool, apiName)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Experience not found: "+apiName)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleCreateExperience(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	var body struct {
		APIName             string   `json:"apiName"`
		Label               string   `json:"label"`
		Description         string   `json:"description"`
		HomeURL             string   `json:"homeUrl"`
		ConnectedAppAPIName string   `json:"connectedAppApiName"`
		AllowedOrigins      []string `json:"allowedOrigins"`
		Active              *bool    `json:"active"`
		PackageName         string   `json:"packageName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	apiName := strings.TrimSpace(body.APIName)
	label := strings.TrimSpace(body.Label)
	if apiName == "" || label == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "apiName and label are required")
		return
	}
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	pkg := strings.TrimSpace(body.PackageName)
	if pkg == "" {
		pkg = "customer.default"
	}
	origins := body.AllowedOrigins
	if origins == nil {
		origins = []string{}
	}
	originsJSON, _ := json.Marshal(origins)
	var id string
	err := pool.QueryRow(r.Context(), `
INSERT INTO metadata_experiences (
  api_name, label, description, home_url, connected_app_api_name, allowed_origins, active, ownership, package_name
)
VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,'custom',$8)
RETURNING id::text`,
		apiName, label, body.Description, strings.TrimSpace(body.HomeURL),
		strings.TrimSpace(body.ConnectedAppAPIName), string(originsJSON), active, pkg,
	).Scan(&id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	m, err := loadExperience(r.Context(), pool, apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handlePatchExperience(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	var ownership string
	err := pool.QueryRow(r.Context(), `SELECT ownership FROM metadata_experiences WHERE api_name=$1`, apiName).Scan(&ownership)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "Experience not found: "+apiName)
			return
		}
		writeAPIError(w, err)
		return
	}
	if ownership != "custom" {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "cannot modify managed experience")
		return
	}
	var body struct {
		Label               *string   `json:"label"`
		Description         *string   `json:"description"`
		HomeURL             *string   `json:"homeUrl"`
		ConnectedAppAPIName *string   `json:"connectedAppApiName"`
		AllowedOrigins      *[]string `json:"allowedOrigins"`
		Active              *bool     `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	sets := []string{"updated_at = now()"}
	args := []any{apiName}
	n := 2
	if body.Label != nil {
		sets = append(sets, "label=$"+strconv.Itoa(n))
		args = append(args, *body.Label)
		n++
	}
	if body.Description != nil {
		sets = append(sets, "description=$"+strconv.Itoa(n))
		args = append(args, *body.Description)
		n++
	}
	if body.HomeURL != nil {
		sets = append(sets, "home_url=$"+strconv.Itoa(n))
		args = append(args, *body.HomeURL)
		n++
	}
	if body.ConnectedAppAPIName != nil {
		sets = append(sets, "connected_app_api_name=$"+strconv.Itoa(n))
		args = append(args, *body.ConnectedAppAPIName)
		n++
	}
	if body.AllowedOrigins != nil {
		originsJSON, _ := json.Marshal(*body.AllowedOrigins)
		sets = append(sets, "allowed_origins=$"+strconv.Itoa(n)+"::jsonb")
		args = append(args, string(originsJSON))
		n++
	}
	if body.Active != nil {
		sets = append(sets, "active=$"+strconv.Itoa(n))
		args = append(args, *body.Active)
	}
	if len(sets) == 1 {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "no fields to update")
		return
	}
	_, err = pool.Exec(r.Context(), `UPDATE metadata_experiences SET `+strings.Join(sets, ",")+` WHERE api_name=$1`, args...)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	m, err := loadExperience(r.Context(), pool, apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteExperience(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	apiName := r.PathValue("apiName")
	var ownership string
	err := pool.QueryRow(r.Context(), `SELECT ownership FROM metadata_experiences WHERE api_name=$1`, apiName).Scan(&ownership)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", "Experience not found: "+apiName)
			return
		}
		writeAPIError(w, err)
		return
	}
	if ownership != "custom" {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "cannot delete managed experience")
		return
	}
	_, err = pool.Exec(r.Context(), `DELETE FROM metadata_experiences WHERE api_name=$1 AND ownership='custom'`, apiName)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "apiName": apiName})
}

func loadExperience(ctx context.Context, pool *db.Pool, apiName string) (map[string]any, error) {
	row := pool.QueryRow(ctx, `
SELECT api_name, label, description, home_url, connected_app_api_name, allowed_origins,
       active, ownership, package_name, created_at, updated_at
FROM metadata_experiences WHERE api_name=$1`, apiName)
	return scanExperienceRow(row)
}

type experienceScanner interface {
	Scan(dest ...any) error
}

func scanExperienceRow(row experienceScanner) (map[string]any, error) {
	var apiName, label, description, homeURL, connApp, ownership, pkg string
	var origins []byte
	var active bool
	var createdAt, updatedAt time.Time
	if err := row.Scan(&apiName, &label, &description, &homeURL, &connApp, &origins, &active, &ownership, &pkg, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var originsVal []string
	_ = json.Unmarshal(origins, &originsVal)
	if originsVal == nil {
		originsVal = []string{}
	}
	return map[string]any{
		"apiName":             apiName,
		"label":               label,
		"description":         description,
		"homeUrl":             homeURL,
		"connectedAppApiName": connApp,
		"allowedOrigins":      originsVal,
		"active":              active,
		"ownership":           ownership,
		"packageName":         pkg,
		"createdAt":           createdAt,
		"updatedAt":           updatedAt,
	}, nil
}
