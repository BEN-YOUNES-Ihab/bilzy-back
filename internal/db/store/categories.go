package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bilzy/bilzy-back/internal/domain"
)

// ListCategories returns categories for an owner + kind, ordered by position.
// includeArchived=false filters out soft-deleted rows (the picker default).
func (s *Store) ListCategories(ctx context.Context, ownerID uuid.UUID, kind string, includeArchived bool) ([]domain.Category, error) {
	q := `
		select id, kind, slug, name, i18n_key, icon, color, position, is_system, archived_at
		from categories
		where owner_id = $1 and kind = $2
	`
	if !includeArchived {
		q += ` and archived_at is null`
	}
	q += ` order by position asc`

	rows, err := s.pool.Query(ctx, q, ownerID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Category, 0)
	for rows.Next() {
		var c domain.Category
		var archivedAt *time.Time
		if err := rows.Scan(
			&c.ID, &c.Kind, &c.Slug, &c.Name, &c.I18nKey,
			&c.Icon, &c.Color, &c.Position, &c.IsSystem, &archivedAt,
		); err != nil {
			return nil, err
		}
		if archivedAt != nil {
			s := archivedAt.UTC().Format(time.RFC3339)
			c.ArchivedAt = &s
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type CreateCategoryInput struct {
	Kind  string
	Slug  string
	Name  string
	Icon  string
	Color string
}

// CreateCategory inserts a custom category at the next-highest position
// within (owner, kind). is_system is false; i18n_key is null.
func (s *Store) CreateCategory(ctx context.Context, ownerID uuid.UUID, in CreateCategoryInput) (*domain.Category, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var maxPos *int32
	if err := tx.QueryRow(ctx,
		`select max(position) from categories where owner_id = $1 and kind = $2`,
		ownerID, in.Kind,
	).Scan(&maxPos); err != nil {
		return nil, err
	}
	nextPos := int32(0)
	if maxPos != nil {
		nextPos = *maxPos + 1
	}

	const insert = `
		insert into categories
		  (owner_id, kind, slug, name, icon, color, position, is_system)
		values ($1, $2, $3, $4, $5, $6, $7, false)
		returning id, kind, slug, name, i18n_key, icon, color, position, is_system, archived_at
	`
	var c domain.Category
	var archivedAt *time.Time
	if err := tx.QueryRow(ctx, insert,
		ownerID, in.Kind, in.Slug, in.Name, in.Icon, in.Color, nextPos,
	).Scan(&c.ID, &c.Kind, &c.Slug, &c.Name, &c.I18nKey,
		&c.Icon, &c.Color, &c.Position, &c.IsSystem, &archivedAt); err != nil {
		return nil, err
	}
	if archivedAt != nil {
		s := archivedAt.UTC().Format(time.RFC3339)
		c.ArchivedAt = &s
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateCategoryPatch carries the optional fields. To clear a name override
// on a system seed, pass NameSet=true with NameValue=nil.
type UpdateCategoryPatch struct {
	NameSet   bool
	NameValue *string
	Icon      *string
	Color     *string
}

func (s *Store) UpdateCategory(ctx context.Context, ownerID, id uuid.UUID, patch UpdateCategoryPatch) (*domain.Category, error) {
	const q = `
		update categories
		set
		  name  = case when $3::int = 1 then $4::text else name  end,
		  icon  = coalesce($5, icon),
		  color = coalesce($6, color)
		where id = $1 and owner_id = $2
		returning id, kind, slug, name, i18n_key, icon, color, position, is_system, archived_at
	`
	nameSet := 0
	if patch.NameSet {
		nameSet = 1
	}

	var c domain.Category
	var archivedAt *time.Time
	err := s.pool.QueryRow(ctx, q,
		id, ownerID,
		nameSet, patch.NameValue,
		patch.Icon, patch.Color,
	).Scan(&c.ID, &c.Kind, &c.Slug, &c.Name, &c.I18nKey,
		&c.Icon, &c.Color, &c.Position, &c.IsSystem, &archivedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if archivedAt != nil {
		ds := archivedAt.UTC().Format(time.RFC3339)
		c.ArchivedAt = &ds
	}
	return &c, nil
}

// ArchiveCategory soft-deletes a row by stamping archived_at. Returns
// (false, nil) when the row doesn't exist or isn't owned by the caller.
func (s *Store) ArchiveCategory(ctx context.Context, ownerID, id uuid.UUID) (bool, error) {
	const q = `
		update categories
		set archived_at = now()
		where id = $1 and owner_id = $2 and archived_at is null
	`
	tag, err := s.pool.Exec(ctx, q, id, ownerID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ReorderCategories assigns position by index in `orderedIDs` for the given
// (owner, kind). Runs in a single transaction so a partial failure can't leave
// positions inconsistent.
func (s *Store) ReorderCategories(ctx context.Context, ownerID uuid.UUID, kind string, orderedIDs []uuid.UUID) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for i, id := range orderedIDs {
		if _, err := tx.Exec(ctx,
			`update categories set position = $1
			   where id = $2 and owner_id = $3 and kind = $4`,
			int32(i), id, ownerID, kind,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
