package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// DataRole is a row from data_roles.
type DataRole struct {
	ID               string
	APIName          string
	Label            string
	ParentDataRoleID *string
	IsSystem         bool
}

// DataRoleStore persists data roles for record sharing.
type DataRoleStore struct {
	pool *Pool
}

// NewDataRoleStore constructs a data role store.
func NewDataRoleStore(pool *Pool) *DataRoleStore {
	return &DataRoleStore{pool: pool}
}

// ListDataRoles returns all data roles.
func (s *DataRoleStore) ListDataRoles(ctx context.Context) ([]DataRole, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, api_name, label, parent_data_role_id::text, is_system
FROM data_roles ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DataRole
	for rows.Next() {
		var r DataRole
		var parent *string
		if err := rows.Scan(&r.ID, &r.APIName, &r.Label, &parent, &r.IsSystem); err != nil {
			return nil, err
		}
		r.ParentDataRoleID = parent
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetDataRoleByAPIName loads a data role by api name.
func (s *DataRoleStore) GetDataRoleByAPIName(ctx context.Context, apiName string) (*DataRole, error) {
	return s.scanOne(ctx, `SELECT id::text, api_name, label, parent_data_role_id::text, is_system
FROM data_roles WHERE api_name = $1`, apiName)
}

// GetDataRoleByID loads a data role by id.
func (s *DataRoleStore) GetDataRoleByID(ctx context.Context, id string) (*DataRole, error) {
	return s.scanOne(ctx, `SELECT id::text, api_name, label, parent_data_role_id::text, is_system
FROM data_roles WHERE id = $1::uuid`, id)
}

func (s *DataRoleStore) scanOne(ctx context.Context, q string, args ...any) (*DataRole, error) {
	var r DataRole
	var parent *string
	err := s.pool.QueryRow(ctx, q, args...).Scan(&r.ID, &r.APIName, &r.Label, &parent, &r.IsSystem)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.ParentDataRoleID = parent
	return &r, nil
}

// CreateDataRole inserts a customer data role.
func (s *DataRoleStore) CreateDataRole(ctx context.Context, apiName, label string, parentID *string) (*DataRole, error) {
	apiName = strings.TrimSpace(apiName)
	if apiName == "" || label == "" {
		return nil, fmt.Errorf("%w: apiName and label required", ErrValidation)
	}
	if parentID != nil && *parentID != "" {
		if _, err := s.GetDataRoleByID(ctx, *parentID); err != nil {
			return nil, err
		}
	}
	row := s.pool.QueryRow(ctx, `
INSERT INTO data_roles (api_name, label, parent_data_role_id)
VALUES ($1, $2, $3::uuid)
RETURNING id::text, api_name, label, parent_data_role_id::text, is_system`,
		apiName, label, parentID,
	)
	return s.scanOneRow(row)
}

// UpdateDataRole patches label and/or parent.
func (s *DataRoleStore) UpdateDataRole(ctx context.Context, apiName, label string, parentID *string, clearParent bool) (*DataRole, error) {
	existing, err := s.GetDataRoleByAPIName(ctx, apiName)
	if err != nil {
		return nil, err
	}
	if existing.IsSystem {
		return nil, fmt.Errorf("%w: system data role is immutable", ErrValidation)
	}
	if label == "" {
		label = existing.Label
	}
	var parent any
	if clearParent {
		parent = nil
	} else if parentID != nil {
		if *parentID == existing.ID {
			return nil, fmt.Errorf("%w: data role cannot be its own parent", ErrValidation)
		}
		if err := s.assertNoHierarchyCycle(ctx, existing.ID, *parentID); err != nil {
			return nil, err
		}
		parent = *parentID
	} else if existing.ParentDataRoleID != nil {
		parent = *existing.ParentDataRoleID
	}
	row := s.pool.QueryRow(ctx, `
UPDATE data_roles SET label=$2, parent_data_role_id=$3::uuid
WHERE api_name=$1 AND is_system=false
RETURNING id::text, api_name, label, parent_data_role_id::text, is_system`,
		apiName, label, parent,
	)
	return s.scanOneRow(row)
}

// assertNoHierarchyCycle rejects parent assignments that would introduce a cycle.
func (s *DataRoleStore) assertNoHierarchyCycle(ctx context.Context, roleID, newParentID string) error {
	if newParentID == "" {
		return nil
	}
	hierarchy, err := s.LoadDataRoleHierarchy(ctx)
	if err != nil {
		return err
	}
	// Walk from proposed parent upward; if we hit roleID, roleID is already an ancestor of parent → cycle.
	cur := newParentID
	seen := map[string]struct{}{}
	for cur != "" {
		if cur == roleID {
			return fmt.Errorf("%w: data role hierarchy cycle detected", ErrValidation)
		}
		if _, ok := seen[cur]; ok {
			return fmt.Errorf("%w: data role hierarchy cycle detected", ErrValidation)
		}
		seen[cur] = struct{}{}
		parent := hierarchy[cur]
		if parent == nil {
			break
		}
		cur = *parent
	}
	return nil
}

// DeleteDataRole removes a non-system role with no assignments.
func (s *DataRoleStore) DeleteDataRole(ctx context.Context, apiName string) error {
	role, err := s.GetDataRoleByAPIName(ctx, apiName)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return fmt.Errorf("%w: cannot delete system data role", ErrValidation)
	}
	var assigned int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE data_role_id = $1::uuid`, role.ID).Scan(&assigned); err != nil {
		return err
	}
	if assigned > 0 {
		return fmt.Errorf("%w: data role is assigned to users", ErrValidation)
	}
	var children int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM data_roles WHERE parent_data_role_id = $1::uuid`, role.ID).Scan(&children); err != nil {
		return err
	}
	if children > 0 {
		return fmt.Errorf("%w: data role has child roles", ErrValidation)
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM data_roles WHERE id = $1::uuid AND is_system=false`, role.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *DataRoleStore) scanOneRow(row pgx.Row) (*DataRole, error) {
	var r DataRole
	var parent *string
	err := row.Scan(&r.ID, &r.APIName, &r.Label, &parent, &r.IsSystem)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.ParentDataRoleID = parent
	return &r, nil
}

// LoadDataRoleHierarchy returns id -> parent id map for all data roles.
func (s *DataRoleStore) LoadDataRoleHierarchy(ctx context.Context) (map[string]*string, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, parent_data_role_id::text FROM data_roles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*string{}
	for rows.Next() {
		var id string
		var parent *string
		if err := rows.Scan(&id, &parent); err != nil {
			return nil, err
		}
		out[id] = parent
	}
	return out, rows.Err()
}

// ListSubordinateDataRoleIDs returns role id and all descendant role ids.
func (s *DataRoleStore) ListSubordinateDataRoleIDs(ctx context.Context, rootRoleID string) ([]string, error) {
	hierarchy, err := s.LoadDataRoleHierarchy(ctx)
	if err != nil {
		return nil, err
	}
	children := map[string][]string{}
	for id, parent := range hierarchy {
		if parent != nil {
			children[*parent] = append(children[*parent], id)
		}
	}
	out := []string{rootRoleID}
	queue := []string{rootRoleID}
	seen := map[string]struct{}{rootRoleID: {}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, ch := range children[cur] {
			if _, ok := seen[ch]; ok {
				continue
			}
			seen[ch] = struct{}{}
			out = append(out, ch)
			queue = append(queue, ch)
		}
	}
	return out, nil
}

// ListUserIDsInDataRoles returns user ids assigned to any of the given data roles.
func (s *DataRoleStore) ListUserIDsInDataRoles(ctx context.Context, roleIDs []string) ([]string, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
SELECT id::text FROM users WHERE data_role_id = ANY($1::uuid[])`, roleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// GetUserDataRoleID returns the user's data role id if set.
func (s *DataRoleStore) GetUserDataRoleID(ctx context.Context, userID string) (*string, error) {
	var roleID *string
	err := s.pool.QueryRow(ctx, `SELECT data_role_id::text FROM users WHERE id = $1::uuid`, userID).Scan(&roleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return roleID, err
}

// SetUserDataRole assigns or clears a user's data role.
func (s *DataRoleStore) SetUserDataRole(ctx context.Context, userID string, dataRoleID *string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET data_role_id = $2::uuid, updated_at = now() WHERE id = $1::uuid`, userID, dataRoleID)
	return err
}

// GetOwnerDataRoleID loads the owner's data role for hierarchy checks.
func (s *DataRoleStore) GetOwnerDataRoleID(ctx context.Context, ownerUserID string) (*string, error) {
	if ownerUserID == "" {
		return nil, nil
	}
	return s.GetUserDataRoleID(ctx, ownerUserID)
}
