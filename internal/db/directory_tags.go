package db

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const directoryTagMemberCap = 200

var directoryTagAPINamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// DirectoryTag is a non-AuthZ grouping label (SCIM Group).
type DirectoryTag struct {
	ID          string
	APIName     string
	DisplayName string
	ExternalID  *string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	MemberCount int
}

// DirectoryTagMember is a principal assigned to a directory tag.
type DirectoryTagMember struct {
	UserID        string
	DisplayName   string
	PrincipalType string
}

// DirectoryTagGroupRef is the SCIM User.groups projection (id + displayName).
type DirectoryTagGroupRef struct {
	ID          string
	DisplayName string
}

// CreateDirectoryTagInput creates a directory tag.
type CreateDirectoryTagInput struct {
	APIName     string
	DisplayName string
	ExternalID  string
	Description string
	// AutoAPIName allocates api_name from DisplayName (suffix 2… on collision).
	AutoAPIName bool
}

// UpdateDirectoryTagInput patches mutable tag fields. api_name is immutable.
type UpdateDirectoryTagInput struct {
	DisplayName *string
	ExternalID  *string
	Description *string
}

// ListDirectoryTagsFilter is the R1 SCIM Group list filter (eq only).
type ListDirectoryTagsFilter struct {
	DisplayName string
	ExternalID  string
}

// DirectoryTagStore persists directory_tags and user_directory_tags.
type DirectoryTagStore struct {
	pool *Pool
}

// NewDirectoryTagStore constructs a directory tag store.
func NewDirectoryTagStore(pool *Pool) *DirectoryTagStore {
	return &DirectoryTagStore{pool: pool}
}

const directoryTagSelectCols = `id::text, api_name, display_name, external_id, description, created_at, updated_at,
  (SELECT COUNT(*) FROM user_directory_tags udt WHERE udt.tag_id = directory_tags.id)`

func scanDirectoryTag(row interface{ Scan(dest ...any) error }) (*DirectoryTag, error) {
	var t DirectoryTag
	err := row.Scan(&t.ID, &t.APIName, &t.DisplayName, &t.ExternalID, &t.Description, &t.CreatedAt, &t.UpdatedAt, &t.MemberCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ValidateDirectoryTagAPIName reports whether apiName matches Client identifier rules.
func ValidateDirectoryTagAPIName(apiName string) error {
	apiName = strings.TrimSpace(apiName)
	if apiName == "" {
		return fmt.Errorf("%w: apiName is required", ErrValidation)
	}
	if !directoryTagAPINamePattern.MatchString(apiName) {
		return fmt.Errorf("%w: apiName must match [A-Za-z][A-Za-z0-9_]*", ErrValidation)
	}
	return nil
}

// DeriveDirectoryTagAPIName slugs a display name into PascalCase [A-Za-z][A-Za-z0-9]*.
func DeriveDirectoryTagAPIName(displayName string) string {
	var words []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		words = append(words, cur.String())
		cur.Reset()
	}
	for _, r := range displayName {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	var b strings.Builder
	for _, w := range words {
		rs := []rune(w)
		if len(rs) == 0 {
			continue
		}
		rs[0] = unicode.ToUpper(rs[0])
		b.WriteString(string(rs))
	}
	name := b.String()
	if name == "" {
		return "Tag"
	}
	first, _ := utf8.DecodeRuneInString(name)
	if !unicode.IsLetter(first) {
		name = "T" + name
	}
	if err := ValidateDirectoryTagAPIName(name); err != nil {
		return "Tag"
	}
	return name
}

// Create inserts a directory tag.
func (s *DirectoryTagStore) Create(ctx context.Context, in CreateDirectoryTagInput) (*DirectoryTag, error) {
	display := strings.TrimSpace(in.DisplayName)
	if display == "" {
		return nil, fmt.Errorf("%w: displayName/label is required", ErrValidation)
	}
	apiName := strings.TrimSpace(in.APIName)
	if in.AutoAPIName || apiName == "" {
		base := apiName
		if base == "" {
			base = DeriveDirectoryTagAPIName(display)
		}
		if err := ValidateDirectoryTagAPIName(base); err != nil {
			return nil, err
		}
		allocated, err := s.allocateAPIName(ctx, base)
		if err != nil {
			return nil, err
		}
		apiName = allocated
	} else if err := ValidateDirectoryTagAPIName(apiName); err != nil {
		return nil, err
	}
	t, err := scanDirectoryTag(s.pool.QueryRow(ctx, `
INSERT INTO directory_tags (api_name, display_name, external_id, description)
VALUES ($1, $2, $3, $4)
RETURNING `+directoryTagSelectCols,
		apiName, display, nilIfBlank(in.ExternalID), nilIfBlank(in.Description),
	))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: directory tag unique field already exists", ErrConflict)
		}
		return nil, err
	}
	return t, nil
}

func (s *DirectoryTagStore) allocateAPIName(ctx context.Context, base string) (string, error) {
	candidate := base
	for n := 2; n < 10000; n++ {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM directory_tags WHERE api_name = $1)`, candidate).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s%d", base, n)
	}
	return "", fmt.Errorf("%w: could not allocate unique apiName", ErrConflict)
}

// GetByID loads a tag by UUID.
func (s *DirectoryTagStore) GetByID(ctx context.Context, id string) (*DirectoryTag, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrNotFound
	}
	return scanDirectoryTag(s.pool.QueryRow(ctx, `SELECT `+directoryTagSelectCols+` FROM directory_tags WHERE id = $1::uuid`, id))
}

// GetByAPIName loads a tag by Client api_name.
func (s *DirectoryTagStore) GetByAPIName(ctx context.Context, apiName string) (*DirectoryTag, error) {
	apiName = strings.TrimSpace(apiName)
	if apiName == "" {
		return nil, ErrNotFound
	}
	return scanDirectoryTag(s.pool.QueryRow(ctx, `SELECT `+directoryTagSelectCols+` FROM directory_tags WHERE api_name = $1`, apiName))
}

// List returns tags matching the optional eq filters.
func (s *DirectoryTagStore) List(ctx context.Context, f ListDirectoryTagsFilter) ([]DirectoryTag, error) {
	q := `SELECT ` + directoryTagSelectCols + ` FROM directory_tags WHERE 1=1`
	args := []any{}
	if strings.TrimSpace(f.DisplayName) != "" {
		args = append(args, strings.TrimSpace(f.DisplayName))
		q += fmt.Sprintf(` AND display_name = $%d`, len(args))
	}
	if strings.TrimSpace(f.ExternalID) != "" {
		args = append(args, strings.TrimSpace(f.ExternalID))
		q += fmt.Sprintf(` AND external_id = $%d`, len(args))
	}
	q += ` ORDER BY created_at ASC`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DirectoryTag
	for rows.Next() {
		t, err := scanDirectoryTag(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// Update patches display_name, external_id, and description. api_name is never renamed.
func (s *DirectoryTagStore) Update(ctx context.Context, id string, in UpdateDirectoryTagInput) (*DirectoryTag, error) {
	var display any
	if in.DisplayName != nil {
		v := strings.TrimSpace(*in.DisplayName)
		if v == "" {
			return nil, fmt.Errorf("%w: displayName/label cannot be empty", ErrValidation)
		}
		display = v
	}
	var externalID any
	if in.ExternalID != nil {
		externalID = nilIfBlank(*in.ExternalID)
	}
	var description any
	if in.Description != nil {
		description = nilIfBlank(*in.Description)
	}
	if in.DisplayName == nil && in.ExternalID == nil && in.Description == nil {
		return s.GetByID(ctx, id)
	}
	tag, err := scanDirectoryTag(s.pool.QueryRow(ctx, `
UPDATE directory_tags SET
  display_name = CASE WHEN $2 THEN $3 ELSE display_name END,
  external_id = CASE WHEN $4 THEN $5 ELSE external_id END,
  description = CASE WHEN $6 THEN $7 ELSE description END,
  updated_at = now()
WHERE id = $1::uuid
RETURNING `+directoryTagSelectCols,
		id,
		in.DisplayName != nil, display,
		in.ExternalID != nil, externalID,
		in.Description != nil, description,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: directory tag unique field already exists", ErrConflict)
		}
		return nil, err
	}
	return tag, nil
}

// Delete removes a tag and its memberships (users stay).
func (s *DirectoryTagStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM directory_tags WHERE id = $1::uuid`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountMembers returns the unfiltered membership count.
func (s *DirectoryTagStore) CountMembers(ctx context.Context, tagID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_directory_tags WHERE tag_id = $1::uuid`, tagID).Scan(&n)
	return n, err
}

// ListMembers returns a page of members (cap 200) plus the unfiltered total.
func (s *DirectoryTagStore) ListMembers(ctx context.Context, tagID string, limit, offset int) ([]DirectoryTagMember, int, error) {
	total, err := s.CountMembers(ctx, tagID)
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > directoryTagMemberCap {
		limit = directoryTagMemberCap
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(ctx, `
SELECT u.id::text, u.display_name, COALESCE(u.principal_type, 'user')
FROM user_directory_tags udt
JOIN users u ON u.id = udt.user_id
WHERE udt.tag_id = $1::uuid
ORDER BY udt.created_at ASC, u.id ASC
LIMIT $2 OFFSET $3`, tagID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []DirectoryTagMember
	for rows.Next() {
		var m DirectoryTagMember
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.PrincipalType); err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	if out == nil {
		out = []DirectoryTagMember{}
	}
	return out, total, rows.Err()
}

// ListAllMembers returns the complete membership set for mutation semantics.
// SCIM representations remain capped separately, but PATCH/PUT must never treat
// the first response page as the whole group and silently delete later members.
func (s *DirectoryTagStore) ListAllMembers(ctx context.Context, tagID string) ([]DirectoryTagMember, error) {
	rows, err := s.pool.Query(ctx, `
SELECT u.id::text, u.display_name, COALESCE(u.principal_type, 'user')
FROM user_directory_tags udt
JOIN users u ON u.id = udt.user_id
WHERE udt.tag_id = $1::uuid
ORDER BY udt.created_at ASC, u.id ASC`, tagID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DirectoryTagMember{}
	for rows.Next() {
		var m DirectoryTagMember
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.PrincipalType); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Assign is idempotent membership insert.
func (s *DirectoryTagStore) Assign(ctx context.Context, userID, tagID string) error {
	if _, err := NewUserStore(s.pool).GetByID(ctx, userID); err != nil {
		return err
	}
	if _, err := s.GetByID(ctx, tagID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO user_directory_tags (user_id, tag_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT (user_id, tag_id) DO NOTHING`, userID, tagID)
	return err
}

// Unassign is idempotent membership delete (missing pair is success).
func (s *DirectoryTagStore) Unassign(ctx context.Context, userID, tagID string) error {
	_, err := s.pool.Exec(ctx, `
DELETE FROM user_directory_tags WHERE user_id = $1::uuid AND tag_id = $2::uuid`, userID, tagID)
	return err
}

// ReplaceMembers sets the tag's membership to userIDs (full replace).
func (s *DirectoryTagStore) ReplaceMembers(ctx context.Context, tagID string, userIDs []string) error {
	if _, err := s.GetByID(ctx, tagID); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM user_directory_tags WHERE tag_id = $1::uuid`, tagID); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, id := range userIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		var exists string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id = $1::uuid`, id).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: user %s", ErrNotFound, id)
			}
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO user_directory_tags (user_id, tag_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT (user_id, tag_id) DO NOTHING`, id, tagID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ListAPINamesForUser returns directory tag apiNames for a principal (ordered).
func (s *DirectoryTagStore) ListAPINamesForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
SELECT t.api_name
FROM user_directory_tags udt
JOIN directory_tags t ON t.id = udt.tag_id
WHERE udt.user_id = $1::uuid
ORDER BY t.api_name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// ListGroupRefsForUser returns SCIM Group refs for a principal.
func (s *DirectoryTagStore) ListGroupRefsForUser(ctx context.Context, userID string) ([]DirectoryTagGroupRef, error) {
	rows, err := s.pool.Query(ctx, `
SELECT t.id::text, t.display_name
FROM user_directory_tags udt
JOIN directory_tags t ON t.id = udt.tag_id
WHERE udt.user_id = $1::uuid
ORDER BY t.display_name ASC, t.id ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DirectoryTagGroupRef
	for rows.Next() {
		var ref DirectoryTagGroupRef
		if err := rows.Scan(&ref.ID, &ref.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	if out == nil {
		out = []DirectoryTagGroupRef{}
	}
	return out, rows.Err()
}

// GetIDsByAPINames resolves tag apiNames; unknown names return ErrNotFound.
func (s *DirectoryTagStore) GetIDsByAPINames(ctx context.Context, apiNames []string) ([]string, error) {
	ids := make([]string, 0, len(apiNames))
	seen := map[string]struct{}{}
	for _, name := range apiNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		t, err := s.GetByAPIName(ctx, name)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("%w: DIRECTORY_TAG_NOT_FOUND %s", ErrNotFound, name)
			}
			return nil, err
		}
		ids = append(ids, t.ID)
	}
	return ids, nil
}

// ReplaceUserTagsByAPINames replace-on-PATCH membership for a principal.
func (s *DirectoryTagStore) ReplaceUserTagsByAPINames(ctx context.Context, userID string, apiNames []string) error {
	if _, err := NewUserStore(s.pool).GetByID(ctx, userID); err != nil {
		return err
	}
	ids, err := s.GetIDsByAPINames(ctx, apiNames)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM user_directory_tags WHERE user_id = $1::uuid`, userID); err != nil {
		return err
	}
	for _, tagID := range ids {
		if _, err := tx.Exec(ctx, `
INSERT INTO user_directory_tags (user_id, tag_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT (user_id, tag_id) DO NOTHING`, userID, tagID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
