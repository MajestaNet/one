package rungraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/MajestaNet/ide/internal/db"
)

var ErrRevisionConflict = errors.New("run graph revision conflict")

type Store struct {
	pool *db.Pool
}

func NewStore(pool *db.Pool) *Store {
	return &Store{pool: pool}
}

func EmptyDocument(graphKey string) json.RawMessage {
	doc := RunGraphDocument{
		APIVersion: DocumentAPIVersion,
		ID:         graphKey,
		Title:      "My graph",
		Nodes:      []RunGraphNode{},
		Edges:      []RunGraphEdge{},
	}
	raw, _ := json.Marshal(doc)
	return raw
}

func (s *Store) Get(ctx context.Context, principalID, graphKey string) (*Graph, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("run graph store is not configured")
	}
	var graph Graph
	err := s.pool.QueryRow(ctx, `
SELECT id::text, principal_id::text, graph_key, title, document, revision, created_at, updated_at
FROM principal_run_graphs
WHERE principal_id=$1::uuid AND graph_key=$2`, principalID, graphKey).Scan(
		&graph.ID, &graph.PrincipalID, &graph.GraphKey, &graph.Title, &graph.Document,
		&graph.Revision, &graph.CreatedAt, &graph.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &graph, nil
}

func (s *Store) GetOrCreateHome(ctx context.Context, principalID string) (*Graph, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("run graph store is not configured")
	}
	doc := EmptyDocument(HomeGraphKey)
	_, err := s.pool.Exec(ctx, `
INSERT INTO principal_run_graphs (principal_id, graph_key, title, document)
VALUES ($1::uuid, $2, 'My graph', $3::jsonb)
ON CONFLICT (principal_id, graph_key) DO NOTHING`, principalID, HomeGraphKey, string(doc))
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, principalID, HomeGraphKey)
}

func (s *Store) Upsert(ctx context.Context, principalID, graphKey, title string, document json.RawMessage) (*Graph, error) {
	return s.UpsertIfRevision(ctx, principalID, graphKey, title, document, nil)
}

// UpsertIfRevision conditionally replaces an existing graph. A non-nil
// expectedRevision never inserts and fails when another writer won the race.
func (s *Store) UpsertIfRevision(ctx context.Context, principalID, graphKey, title string, document json.RawMessage, expectedRevision *int64) (*Graph, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("run graph store is not configured")
	}
	var graph Graph
	if expectedRevision != nil {
		err := s.pool.QueryRow(ctx, `
UPDATE principal_run_graphs
SET title=$3, document=$4::jsonb, revision=revision + 1, updated_at=now()
WHERE principal_id=$1::uuid AND graph_key=$2 AND revision=$5
RETURNING id::text, principal_id::text, graph_key, title, document, revision, created_at, updated_at`,
			principalID, graphKey, title, string(document), *expectedRevision,
		).Scan(
			&graph.ID, &graph.PrincipalID, &graph.GraphKey, &graph.Title, &graph.Document,
			&graph.Revision, &graph.CreatedAt, &graph.UpdatedAt,
		)
		if err == pgx.ErrNoRows {
			return nil, ErrRevisionConflict
		}
		if err != nil {
			return nil, err
		}
		return &graph, nil
	}
	err := s.pool.QueryRow(ctx, `
INSERT INTO principal_run_graphs (principal_id, graph_key, title, document)
VALUES ($1::uuid, $2, $3, $4::jsonb)
ON CONFLICT (principal_id, graph_key) DO UPDATE
SET title=EXCLUDED.title,
    document=EXCLUDED.document,
    revision=principal_run_graphs.revision + 1,
    updated_at=now()
RETURNING id::text, principal_id::text, graph_key, title, document, revision, created_at, updated_at`,
		principalID, graphKey, title, string(document),
	).Scan(
		&graph.ID, &graph.PrincipalID, &graph.GraphKey, &graph.Title, &graph.Document,
		&graph.Revision, &graph.CreatedAt, &graph.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &graph, nil
}

func IsNotFound(err error) bool {
	return err == pgx.ErrNoRows
}

func IsRevisionConflict(err error) bool {
	return errors.Is(err, ErrRevisionConflict)
}
