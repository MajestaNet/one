package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
)

func (s *Server) registerIngestRoutes(prefix string, client func(http.HandlerFunc) http.Handler) {
	s.mux.Handle("POST "+prefix+"/jobs/ingest", client(s.handleCreateIngestJob))
	s.mux.Handle("GET "+prefix+"/jobs/ingest/{id}", client(s.handleGetIngestJob))
	s.mux.Handle("PUT "+prefix+"/jobs/ingest/{id}/batches", client(s.handleIngestBatch))
	s.mux.Handle("PATCH "+prefix+"/jobs/ingest/{id}", client(s.handlePatchIngestJob))
	s.mux.Handle("DELETE "+prefix+"/jobs/ingest/{id}", client(s.handleAbortIngestJob))
	s.mux.Handle("GET "+prefix+"/jobs/ingest/{id}/successfulResults", client(s.handleIngestSuccessResults))
	s.mux.Handle("GET "+prefix+"/jobs/ingest/{id}/failedResults", client(s.handleIngestFailedResults))
}

func (s *Server) handleCreateIngestJob(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	var body struct {
		Object          string `json:"object"`
		Operation       string `json:"operation"`
		ExternalIDField string `json:"externalIdField"`
		ContentType     string `json:"contentType"`
		AllOrNone       bool   `json:"allOrNone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	action := authz.ActionCreate
	switch strings.ToLower(body.Operation) {
	case dataengine.IngestOpUpdate, dataengine.IngestOpUpsert:
		// Upsert may create; still require update capability when field present — create checked per-row.
		action = authz.ActionUpdate
	case dataengine.IngestOpDelete:
		action = authz.ActionDelete
	}
	if body.Operation == dataengine.IngestOpInsert || body.Operation == dataengine.IngestOpUpsert {
		if err := s.objectAz.AssertObjectAccess(r.Context(), actor, body.Object, authz.ActionCreate); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	if action != authz.ActionCreate {
		if err := s.objectAz.AssertObjectAccess(r.Context(), actor, body.Object, action); err != nil {
			writeAPIError(w, err)
			return
		}
	}
	job, err := s.data.CreateIngestJob(r.Context(), actor, dataengine.CreateIngestJobInput{
		ObjectAPIName:   body.Object,
		Operation:       body.Operation,
		ExternalIDField: body.ExternalIDField,
		ContentType:     body.ContentType,
		AllOrNone:       body.AllOrNone,
	})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) handleGetIngestJob(w http.ResponseWriter, r *http.Request) {
	if s.data == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	job, err := s.data.GetIngestJob(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if job.ActorID != actor.ID && (actor == nil || !actor.IsAdmin) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Not job owner")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleIngestBatch(w http.ResponseWriter, r *http.Request) {
	if s.data == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	raw, err := readBodyLimited(r.Body, dataengine.IngestMaxUploadBytes)
	if err != nil {
		if requestBodyTooLarge(err) {
			writeErr(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "batch too large")
			return
		}
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unable to read body")
		return
	}
	job, err := s.data.AppendIngestBatch(r.Context(), r.PathValue("id"), actor.ID, raw)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handlePatchIngestJob(w http.ResponseWriter, r *http.Request) {
	if s.data == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	var body struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body.State != dataengine.IngestStateUploadComplete {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "state must be UploadComplete")
		return
	}
	job, err := s.data.CloseIngestJob(r.Context(), r.PathValue("id"), actor.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleAbortIngestJob(w http.ResponseWriter, r *http.Request) {
	if s.data == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	job, err := s.data.AbortIngestJob(r.Context(), r.PathValue("id"), actor.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleIngestSuccessResults(w http.ResponseWriter, r *http.Request) {
	s.writeIngestResults(w, r, false)
}

func (s *Server) handleIngestFailedResults(w http.ResponseWriter, r *http.Request) {
	s.writeIngestResults(w, r, true)
}

func (s *Server) writeIngestResults(w http.ResponseWriter, r *http.Request, failed bool) {
	if s.data == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	raw, err := s.data.IngestJobResults(r.Context(), r.PathValue("id"), actor.ID, failed)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
