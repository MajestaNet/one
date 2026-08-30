package dataengine

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

const (
	searchMinChars     = 2
	searchMinDigits    = 3
	searchDefaultLimit = 20
	searchMaxLimit     = 50
	searchReindexPage  = 500
)

// SearchRequest is POST /client/v1/search.
type SearchRequest struct {
	Query   string   `json:"q"`
	Objects []string `json:"objects,omitempty"`
	Limit   int      `json:"limit,omitempty"`
}

// SearchHit is one ranked find result.
type SearchHit struct {
	ID        string  `json:"id"`
	Object    string  `json:"object"`
	Title     string  `json:"title"`
	Subtitle  string  `json:"subtitle,omitempty"`
	UpdatedAt string  `json:"updatedAt"`
	Score     float64 `json:"score"`
}

// SearchResult is the Client search response.
type SearchResult struct {
	Hits  []SearchHit `json:"hits"`
	Query string      `json:"query"`
}

// SearchScope is one object partition to probe, with that object's visibility.
type SearchScope struct {
	ObjectAPIName string
	StorageMode   string
	Visibility    QueryVisibility
}

type searchRow struct {
	ID        string
	Object    string
	Title     string
	Subtitle  string
	UpdatedAt time.Time
	Score     float64
}

var searchTitleFields = []string{"Name", "Subject", "Title"}
var searchSubtitleFields = []string{"Email", "Phone", "MobilePhone", "AccountNumber", "SerialNumber", "ProductCode"}

// NormalizeSearchQuery lowercases, collapses whitespace, and strips LIKE wildcards.
func NormalizeSearchQuery(q string) (normalized string, digits string, err error) {
	q = strings.TrimSpace(strings.ToLower(q))
	q = strings.Join(strings.Fields(q), " ")
	q = strings.ReplaceAll(q, "%", "")
	q = strings.ReplaceAll(q, "_", "")
	q = strings.Join(strings.Fields(q), " ")
	if q == "" {
		return "", "", validationErrorf("q is required")
	}
	digits = digitsOnly(q)
	if isDigitsOnlyQuery(q) {
		if len(digits) < searchMinDigits {
			return "", "", validationErrorf("q must be at least %d digits", searchMinDigits)
		}
	} else if len([]rune(q)) < searchMinChars {
		return "", "", validationErrorf("q must be at least %d characters", searchMinChars)
	}
	return q, digits, nil
}

func isDigitsOnlyQuery(q string) bool {
	if q == "" {
		return false
	}
	for _, r := range q {
		if unicode.IsSpace(r) {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return digitsOnly(q) != ""
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func coerceSearchString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

func normalizeSearchToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Join(strings.Fields(s), " ")
}

func HasSearchableField(fields []metadata.FieldDefinition) bool {
	for _, f := range fields {
		if f.Searchable {
			return true
		}
	}
	return false
}

// BuildSearchDocument concatenates normalized searchable values plus title/subtitle.
func BuildSearchDocument(fields []metadata.FieldDefinition, data map[string]any) (document, title, subtitle string) {
	if data == nil {
		data = map[string]any{}
	}
	tokens := make([]string, 0, len(fields)+4)
	byName := make(map[string]string, len(fields))
	for _, f := range fields {
		if !f.Searchable {
			continue
		}
		raw := coerceSearchString(data[f.APIName])
		if raw == "" {
			continue
		}
		token := normalizeSearchToken(raw)
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
		if f.FieldType == metadata.FieldTypePhone {
			if d := digitsOnly(raw); d != "" && d != token {
				tokens = append(tokens, d)
			}
		}
		byName[f.APIName] = raw
	}
	title = pickSearchTitle(byName)
	subtitle = pickSearchSubtitle(byName, title)
	document = strings.Join(tokens, " ")
	return document, title, subtitle
}

func pickSearchTitle(byName map[string]string) string {
	for _, key := range searchTitleFields {
		if v := strings.TrimSpace(byName[key]); v != "" {
			return v
		}
	}
	first := strings.TrimSpace(byName["FirstName"])
	last := strings.TrimSpace(byName["LastName"])
	if first != "" && last != "" {
		return strings.TrimSpace(first + " " + last)
	}
	if last != "" {
		return last
	}
	if first != "" {
		return first
	}
	return ""
}

func pickSearchSubtitle(byName map[string]string, title string) string {
	for _, key := range searchSubtitleFields {
		v := strings.TrimSpace(byName[key])
		if v == "" || v == title {
			continue
		}
		return v
	}
	return ""
}

func clampSearchLimit(limit int) int {
	if limit <= 0 {
		return searchDefaultLimit
	}
	if limit > searchMaxLimit {
		return searchMaxLimit
	}
	return limit
}

func visSignature(v QueryVisibility) string {
	return fmt.Sprintf("%s|%s|%v|%s|%s", v.Mode, v.DefaultAccess, v.HasObjectRead, v.UserID, strings.Join(v.SubordinateDataRoleIDs, ","))
}

// Search ranks records across the given object scopes (AuthZ already applied by the caller).
func (s *Service) Search(ctx context.Context, req SearchRequest, scopes []SearchScope) (*SearchResult, error) {
	normalized, _, err := NormalizeSearchQuery(req.Query)
	if err != nil {
		return nil, err
	}
	limit := clampSearchLimit(req.Limit)
	out := &SearchResult{Query: normalized, Hits: []SearchHit{}}
	if len(scopes) == 0 {
		return out, nil
	}

	type group struct {
		table string
		vis   QueryVisibility
		names []string
	}
	groups := map[string]*group{}
	order := make([]string, 0)
	for _, sc := range scopes {
		if strings.TrimSpace(sc.ObjectAPIName) == "" {
			continue
		}
		if err := rejectKernelStorage(sc.ObjectAPIName, sc.StorageMode); err != nil {
			continue
		}
		table := db.RecordsTableForStorageMode(sc.StorageMode)
		key := table + "\x00" + visSignature(sc.Visibility)
		g, ok := groups[key]
		if !ok {
			g = &group{table: table, vis: sc.Visibility}
			groups[key] = g
			order = append(order, key)
		}
		g.names = append(g.names, sc.ObjectAPIName)
	}

	rows := make([]searchRow, 0, limit*2)
	for _, key := range order {
		g := groups[key]
		got, err := s.searchStore(ctx, g.table, g.names, g.vis, normalized, limit)
		if err != nil {
			return nil, err
		}
		rows = append(rows, got...)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		if !rows[i].UpdatedAt.Equal(rows[j].UpdatedAt) {
			return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
		}
		return rows[i].ID > rows[j].ID
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	hits := make([]SearchHit, 0, len(rows))
	for _, r := range rows {
		hits = append(hits, SearchHit{
			ID:        r.ID,
			Object:    r.Object,
			Title:     r.Title,
			Subtitle:  r.Subtitle,
			UpdatedAt: r.UpdatedAt.UTC().Format(time.RFC3339Nano),
			Score:     r.Score,
		})
	}
	out.Hits = hits
	return out, nil
}

func (s *Service) searchStore(ctx context.Context, table string, objects []string, vis QueryVisibility, q string, limit int) ([]searchRow, error) {
	quoted, err := quoteTable(table)
	if err != nil {
		return nil, err
	}
	if len(objects) == 0 {
		return nil, nil
	}
	prefix := q + "%"
	contains := "%" + q + "%"
	args := []any{}
	where := []string{}

	args = append(args, objects)
	objParam := fmt.Sprintf("$%d", len(args))
	where = append(where, fmt.Sprintf("r.object_api_name = ANY(%s)", objParam))

	args = append(args, q)
	qParam := fmt.Sprintf("$%d", len(args))
	args = append(args, prefix)
	prefixParam := fmt.Sprintf("$%d", len(args))
	args = append(args, contains)
	containsParam := fmt.Sprintf("$%d", len(args))

	where = append(where, fmt.Sprintf(`(
		r.search_title ILIKE %s
		OR r.search_document ILIKE %s
		OR r.search_document %% %s
	)`, prefixParam, containsParam, qParam))

	AppendSharingVisibility("r", &where, &args, vis)

	args = append(args, limit)
	limitParam := fmt.Sprintf("$%d", len(args))

	sql := fmt.Sprintf(`
SELECT r.id::text, r.object_api_name, r.search_title, r.search_subtitle, r.updated_at,
  CASE
    WHEN lower(r.search_title) = %s THEN 4.0
    WHEN r.search_title ILIKE %s THEN 3.0
    ELSE GREATEST(similarity(r.search_title, %s), similarity(r.search_document, %s))
  END AS score
FROM %s r
WHERE %s
ORDER BY
  (lower(r.search_title) = %s) DESC,
  (r.search_title ILIKE %s) DESC,
  similarity(r.search_title, %s) DESC,
  similarity(r.search_document, %s) DESC,
  r.updated_at DESC,
  r.id DESC
LIMIT %s`,
		qParam, prefixParam, qParam, qParam,
		quoted,
		strings.Join(where, " AND "),
		qParam, prefixParam, qParam, qParam,
		limitParam,
	)

	rs, err := s.querier(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []searchRow
	for rs.Next() {
		var row searchRow
		if err := rs.Scan(&row.ID, &row.Object, &row.Title, &row.Subtitle, &row.UpdatedAt, &row.Score); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rs.Err()
}

// ReindexSearch rebuilds search_document columns for one object or every searchable object.
func (s *Service) ReindexSearch(ctx context.Context, objectAPIName string) error {
	objects := []string{}
	if name := strings.TrimSpace(objectAPIName); name != "" {
		objects = []string{name}
	} else {
		listed, err := s.meta.ListObjects(ctx)
		if err != nil {
			return err
		}
		for _, o := range listed {
			if db.IsKernelStorage(o.StorageMode) {
				continue
			}
			fields, err := s.meta.GetFields(ctx, o.APIName)
			if err != nil {
				return err
			}
			if HasSearchableField(fields) {
				objects = append(objects, o.APIName)
			}
		}
	}
	for _, name := range objects {
		if err := s.reindexObject(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) reindexObject(ctx context.Context, objectAPIName string) error {
	obj, err := s.meta.GetObject(ctx, objectAPIName)
	if err != nil {
		return err
	}
	if err := rejectKernelStorage(objectAPIName, obj.StorageMode); err != nil {
		return nil
	}
	fields, err := s.meta.GetFields(ctx, objectAPIName)
	if err != nil {
		return err
	}
	table, err := quoteTable(db.RecordsTableForStorageMode(obj.StorageMode))
	if err != nil {
		return err
	}
	var (
		cursorTime = time.Time{}
		cursorID   = "00000000-0000-0000-0000-000000000000"
	)
	for {
		rows, err := s.querier(ctx).Query(ctx, fmt.Sprintf(`
SELECT id::text, created_at, data
FROM %s
WHERE object_api_name = $1
  AND (created_at, id) > ($2, $3::uuid)
ORDER BY created_at, id
LIMIT $4`, table), objectAPIName, cursorTime, cursorID, searchReindexPage)
		if err != nil {
			return err
		}
		type pageRow struct {
			id        string
			createdAt time.Time
			raw       []byte
		}
		var page []pageRow
		for rows.Next() {
			var r pageRow
			if err := rows.Scan(&r.id, &r.createdAt, &r.raw); err != nil {
				rows.Close()
				return err
			}
			page = append(page, r)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if len(page) == 0 {
			return nil
		}
		for _, r := range page {
			data, err := decodeJSONBMap(r.raw)
			if err != nil {
				return err
			}
			doc, title, subtitle := BuildSearchDocument(fields, data)
			if _, err := s.querier(ctx).Exec(ctx, fmt.Sprintf(`
UPDATE %s
SET search_document = $1, search_title = $2, search_subtitle = $3
WHERE id = $4::uuid AND object_api_name = $5`, table),
				doc, title, subtitle, r.id, objectAPIName); err != nil {
				return err
			}
			cursorTime = r.createdAt
			cursorID = r.id
		}
		if len(page) < searchReindexPage {
			return nil
		}
	}
}
