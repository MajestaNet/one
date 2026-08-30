package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/customerrepo"
	"github.com/MajestaNet/ide/internal/dataengine"
	"github.com/MajestaNet/ide/internal/deploy"
)

func (s *Server) registerDeployRoutes() {
	wrap := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeDeploy, h))
	}
	mutate := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeDeploy, s.requireCapability(authz.CapDeployPromote, h)))
	}
	s.mux.Handle("GET /deploy/v1/environment", wrap(s.handleDeployEnvironment))
	s.mux.Handle("POST /deploy/v1/packages/pack", mutate(s.handlePackagePack))
	s.mux.Handle("POST /deploy/v1/packages/validate-local", mutate(s.handlePackageValidateLocal))
	s.mux.Handle("GET /deploy/v1/packages/export", wrap(s.handlePackageExport))
	s.mux.Handle("POST /deploy/v1/packages/initialize-repo", s.requireAuth(s.requireScope(authz.ScopeDeploy, s.requireAdmin(s.requireCapability(authz.CapDeployPromote, http.HandlerFunc(s.handleInitializeRepo))))))
	s.mux.Handle("POST /deploy/v1/bundles", mutate(s.handleCreateBundle))
	s.mux.Handle("GET /deploy/v1/bundles", wrap(s.handleListBundles))
	s.mux.Handle("GET /deploy/v1/bundles/{id}", wrap(s.handleGetBundle))
	s.mux.Handle("GET /deploy/v1/bundles/{id}/artifact", wrap(s.handleGetBundleArtifact))
	s.mux.Handle("POST /deploy/v1/bundles/{id}/validate", mutate(s.handleValidateBundle))
	s.mux.Handle("POST /deploy/v1/promotions", mutate(s.handlePromotions))
	s.mux.Handle("GET /deploy/v1/promotions/{id}", wrap(s.handleGetPromotion))
	s.mux.Handle("GET /deploy/v1/work/{jobId}", wrap(s.handleGetDeployWork))
	s.mux.Handle("POST /deploy/v1/peers", mutate(s.handleUpsertPeer))
	s.mux.Handle("GET /deploy/v1/peers", wrap(s.handleListPeers))
	s.mux.Handle("POST /deploy/v1/tests", mutate(s.handleUpsertTest))
	s.mux.Handle("GET /deploy/v1/tests", wrap(s.handleListTests))
	s.mux.Handle("GET /deploy/v1/tests/{apiName}", wrap(s.handleGetTest))
	s.mux.Handle("POST /deploy/v1/tests/runs", mutate(s.handleStartTestRun))
	s.mux.Handle("GET /deploy/v1/tests/runs", wrap(s.handleListTestRuns))
	s.mux.Handle("GET /deploy/v1/tests/runs/{id}", wrap(s.handleGetTestRun))
}

func (s *Server) handleComposite(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	var body struct {
		CompositeRequest []dataengine.CompositeSubrequest `json:"compositeRequest"`
		Requests         []dataengine.CompositeSubrequest `json:"requests"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	reqs := body.CompositeRequest
	if len(reqs) == 0 {
		reqs = body.Requests
	}
	az := &dataengine.CompositeAuthz{
		AssertObjectAccess:  s.objectAz.AssertObjectAccess,
		CanViewRecord:       s.canViewRecord,
		CanModifyRecord:     s.canModifyRecord,
		GetViewAllObjects:   s.objectAz.GetViewAllObjects,
		GetModifyAllObjects: s.objectAz.GetModifyAllObjects,
	}
	if s.fieldAz != nil {
		az.AssertEditableFields = s.fieldAz.AssertEditableFields
		az.StripUnreadableFields = s.fieldAz.StripUnreadableFields
	}
	result, err := s.data.Composite(r.Context(), reqs, actor, az)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleBulk(w http.ResponseWriter, r *http.Request) {
	if s.data == nil || s.objectAz == nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "data-engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	object := r.PathValue("object")
	if err := s.objectAz.AssertObjectAccess(r.Context(), actor, object, authz.ActionCreate); err != nil {
		writeAPIError(w, err)
		return
	}
	raw, err := readBodyLimited(r.Body, 8<<20)
	if err != nil {
		if requestBodyTooLarge(err) {
			writeErr(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request body too large")
			return
		}
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Unable to read body")
		return
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		var wrapped struct {
			Records []map[string]any `json:"records"`
		}
		if err2 := json.Unmarshal(raw, &wrapped); err2 != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Expected array or {records:[]}")
			return
		}
		rows = wrapped.Records
	}
	result, err := s.data.BulkInsert(r.Context(), object, rows, actor)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) requireDeploy() bool {
	return s.deploy != nil
}

func (s *Server) handleDeployEnvironment(w http.ResponseWriter, _ *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	writeJSON(w, http.StatusOK, s.deploy.GetEnvironment())
}

func (s *Server) handlePackagePack(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	const maxArchive = 32 << 20
	raw, err := customerrepo.ReadAllLimited(r.Body, maxArchive)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if len(raw) == 0 {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "empty archive body")
		return
	}
	label := r.URL.Query().Get("label")
	var labelPtr *string
	if label != "" {
		labelPtr = &label
	}
	actor := ActorFromContext(r.Context())
	var createdBy *string
	if actor != nil && actor.ID != "" {
		createdBy = &actor.ID
	}
	env := s.deploy.GetEnvironment()
	art, _, err := customerrepo.PackArchive(customerrepo.BytesReaderAt{B: raw}, int64(len(raw)), r.Header.Get("Content-Type"), customerrepo.PackOptions{
		CustomerIDOverride: env.CustomerID,
		SourceInstallID:    env.InstallID,
		SourceInstallRole:  env.InstallRole,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	row, err := s.deploy.CreateBundleFromArtifact(r.Context(), struct {
		Artifact  any
		Label     *string
		CreatedBy *string
		Origin    string
		Signature *string
	}{
		Artifact:  art,
		Label:     labelPtr,
		CreatedBy: createdBy,
		Origin:    "customer-package",
	})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (s *Server) handlePackageValidateLocal(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	var createdBy *string
	if actor != nil && actor.ID != "" {
		createdBy = &actor.ID
	}
	label := r.URL.Query().Get("label")
	var labelPtr *string
	if label != "" {
		labelPtr = &label
	}

	const maxBody = 32 << 20
	raw, err := customerrepo.ReadAllLimited(r.Body, maxBody)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	isJSON := strings.Contains(ct, "json") || (len(raw) > 0 && raw[0] == '{')
	bundleIDQuery := r.URL.Query().Get("bundleId")

	if isJSON || (len(raw) == 0 && bundleIDQuery != "") {
		var body struct {
			BundleID string  `json:"bundleId"`
			Artifact any     `json:"artifact"`
			Label    *string `json:"label"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
				return
			}
		}
		if bundleIDQuery != "" && body.BundleID == "" {
			body.BundleID = bundleIDQuery
		}
		if body.Label != nil {
			labelPtr = body.Label
		}
		if body.BundleID == "" && body.Artifact == nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "bundleId, artifact, or zip body is required")
			return
		}
		result, queued, err := s.deploy.EnqueueValidate(r.Context(), struct {
			Artifact  any
			BundleID  string
			Label     *string
			CreatedBy *string
		}{Artifact: body.Artifact, BundleID: body.BundleID, Label: labelPtr, CreatedBy: createdBy}, int64(len(raw)))
		if err != nil {
			writeAPIError(w, err)
			return
		}
		if queued != nil {
			writeJSON(w, http.StatusAccepted, queued)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	if len(raw) == 0 {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "empty archive body (or pass JSON {bundleId})")
		return
	}
	env := s.deploy.GetEnvironment()
	art, _, err := customerrepo.PackArchive(customerrepo.BytesReaderAt{B: raw}, int64(len(raw)), ct, customerrepo.PackOptions{
		CustomerIDOverride: env.CustomerID,
		SourceInstallID:    env.InstallID,
		SourceInstallRole:  env.InstallRole,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	result, queued, err := s.deploy.EnqueueValidate(r.Context(), struct {
		Artifact  any
		BundleID  string
		Label     *string
		CreatedBy *string
	}{Artifact: art, Label: labelPtr, CreatedBy: createdBy}, int64(len(raw)))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if queued != nil {
		writeJSON(w, http.StatusAccepted, queued)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePackageExport(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	row, err := s.deploy.CreateBundleFromSnapshot(r.Context(), struct {
		Label               *string
		CreatedBy           *string
		ProductVersionRange string
	}{})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	var art deploy.BundleArtifact
	if err := json.Unmarshal(row.Artifact, &art); err != nil {
		writeAPIError(w, err)
		return
	}
	env := s.deploy.GetEnvironment()
	man := customerrepo.Manifest{
		CustomerID:          env.CustomerID,
		PackageName:         deploy.DefaultCustomerPackage,
		ProductVersionRange: row.ProductVersionRange,
		RepoFormat:          customerrepo.RepoFormat,
	}
	var buf bytes.Buffer
	if err := customerrepo.WriteZipArchive(&buf, &art, man); err != nil {
		writeAPIError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="one-export.zip"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) handleInitializeRepo(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	var body struct {
		Confirm bool `json:"confirm"`
		Force   bool `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if !body.Confirm {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "confirm must be true")
		return
	}
	art, versionRange, err := s.deploy.ExportRepoArtifact(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	tmpdir, err := os.MkdirTemp("", "one-init-*")
	if err != nil {
		writeAPIError(w, err)
		return
	}
	defer func() { _ = os.RemoveAll(tmpdir) }()
	env := s.deploy.GetEnvironment()
	man := customerrepo.Manifest{
		CustomerID:          env.CustomerID,
		PackageName:         deploy.DefaultCustomerPackage,
		ProductVersionRange: versionRange,
		RepoFormat:          customerrepo.RepoFormat,
	}
	if err := customerrepo.UnpackToDir(tmpdir, art, man); err != nil {
		writeAPIError(w, err)
		return
	}
	sha, err := s.deploy.SeedCustomerRepoDir(r.Context(), tmpdir, body.Force)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.deploy.BuildInitializeRepoResult(art, sha, body.Force))
}

func (s *Server) handleCreateBundle(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	var body struct {
		Label               *string `json:"label"`
		Artifact            any     `json:"artifact"`
		ProductVersionRange string  `json:"productVersionRange"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	var createdBy *string
	if actor != nil {
		createdBy = &actor.ID
	}
	var bundle *deploy.BundleRow
	var err error
	if body.Artifact != nil {
		bundle, err = s.deploy.CreateBundleFromArtifact(r.Context(), struct {
			Artifact  any
			Label     *string
			CreatedBy *string
			Origin    string
			Signature *string
		}{Artifact: body.Artifact, Label: body.Label, CreatedBy: createdBy, Origin: "local"})
	} else {
		bundle, err = s.deploy.CreateBundleFromSnapshot(r.Context(), struct {
			Label               *string
			CreatedBy           *string
			ProductVersionRange string
		}{Label: body.Label, CreatedBy: createdBy, ProductVersionRange: body.ProductVersionRange})
	}
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": bundle.ID, "label": bundle.Label, "customerId": bundle.CustomerID,
		"sourceInstallId": bundle.SourceInstallID, "sourceInstallRole": bundle.SourceInstallRole,
		"origin": bundle.Origin, "productVersion": bundle.ProductVersion,
		"productVersionRange": bundle.ProductVersionRange, "checksum": bundle.Checksum,
		"signature": bundle.Signature, "status": bundle.Status, "createdAt": bundle.CreatedAt,
	})
}

func (s *Server) handleListBundles(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	bundles, err := s.deploy.ListBundles(r.Context(), limit)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"bundles": bundles})
}

func (s *Server) handleGetBundle(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	bundle, err := s.deploy.GetBundle(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": bundle.ID, "label": bundle.Label, "customerId": bundle.CustomerID,
		"sourceInstallId": bundle.SourceInstallID, "sourceInstallRole": bundle.SourceInstallRole,
		"origin": bundle.Origin, "productVersion": bundle.ProductVersion,
		"productVersionRange": bundle.ProductVersionRange, "checksum": bundle.Checksum,
		"signature": bundle.Signature, "status": bundle.Status, "createdAt": bundle.CreatedAt,
		"createdBy": bundle.CreatedBy,
	})
}

func (s *Server) handleGetBundleArtifact(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	bundle, err := s.deploy.GetBundle(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	var artifact any
	_ = json.Unmarshal(bundle.Artifact, &artifact)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": bundle.ID, "customerId": bundle.CustomerID, "checksum": bundle.Checksum,
		"signature": bundle.Signature, "artifact": artifact,
	})
}

func (s *Server) handleValidateBundle(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	result, queued, err := s.deploy.EnqueueValidateBundle(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if queued != nil {
		writeJSON(w, http.StatusAccepted, queued)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePromotions(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	var createdBy *string
	if actor != nil {
		createdBy = &actor.ID
	}
	var body struct {
		BundleID string `json:"bundleId"`
		Artifact any    `json:"artifact"`
		DryRun   bool   `json:"dryRun"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	if body.Artifact != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "inbound artifact promote removed; pack locally then POST promotions with bundleId (repo→org DX)")
		return
	}
	if body.BundleID == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "bundleId is required")
		return
	}
	result, queued, err := s.deploy.EnqueuePromote(r.Context(), struct {
		BundleID  string
		DryRun    bool
		CreatedBy *string
	}{BundleID: body.BundleID, DryRun: body.DryRun, CreatedBy: createdBy}, false)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if queued != nil {
		writeJSON(w, http.StatusAccepted, queued)
		return
	}
	status := http.StatusCreated
	if result.Promotion != nil && result.Promotion.Status == "failed" {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, result)
}

func (s *Server) handleGetPromotion(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	promo, err := s.deploy.GetPromotion(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, promo)
}

func (s *Server) handleGetDeployWork(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	work, err := s.deploy.GetDeployWork(r.Context(), r.PathValue("jobId"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, work)
}

func (s *Server) handleUpsertPeer(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	var body struct {
		InstallID   string  `json:"installId"`
		Label       *string `json:"label"`
		InstallRole *string `json:"installRole"`
		BaseURL     *string `json:"baseUrl"`
		Active      *bool   `json:"active"`
		CustomerID  *string `json:"customerId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InstallID == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "installId is required")
		return
	}
	peer, err := s.deploy.UpsertPeer(r.Context(), struct {
		InstallID   string
		Label       *string
		InstallRole *string
		BaseURL     *string
		Active      *bool
		CustomerID  *string
	}{InstallID: body.InstallID, Label: body.Label, InstallRole: body.InstallRole, BaseURL: body.BaseURL, Active: body.Active, CustomerID: body.CustomerID})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, peer)
}

func (s *Server) handleListPeers(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	peers, err := s.deploy.ListPeers(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"peers": peers})
}

func (s *Server) handleUpsertTest(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	var input deploy.TestSuiteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return
	}
	suite, err := s.deploy.UpsertTestSuite(r.Context(), &input)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, suite)
}

func (s *Server) handleListTests(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	suites, err := s.deploy.ListTestSuites(r.Context())
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suites": suites})
}

func (s *Server) handleGetTest(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	suite, err := s.deploy.GetTestSuite(r.Context(), r.PathValue("apiName"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, suite)
}

func (s *Server) handleStartTestRun(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	actor := ActorFromContext(r.Context())
	var body struct {
		SuiteAPIName string `json:"suiteApiName"`
		Async        bool   `json:"async"`
		Trigger      string `json:"trigger"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SuiteAPIName == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "suiteApiName is required")
		return
	}
	result, err := s.deploy.StartTestRun(r.Context(), struct {
		SuiteAPIName string
		Actor        *authz.Actor
		Async        bool
		Trigger      string
	}{SuiteAPIName: body.SuiteAPIName, Actor: actor, Async: body.Async, Trigger: body.Trigger})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	status := http.StatusCreated
	if result.Mode == "async" {
		status = http.StatusAccepted
	}
	writeJSON(w, status, result)
}

func (s *Server) handleListTestRuns(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, err := s.deploy.ListTestRuns(r.Context(), limit)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (s *Server) handleGetTestRun(w http.ResponseWriter, r *http.Request) {
	if !s.requireDeploy() {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "deploy engine not configured")
		return
	}
	run, err := s.deploy.GetTestRun(r.Context(), r.PathValue("id"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}
