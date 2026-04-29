package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bilzy/bilzy-back/internal/domain"
)

// loadClosingLines loads revenues + expenses for a set of closing IDs,
// indexed by closing_id. Used by both the list and the detail loaders.
func (s *Store) loadClosingLines(ctx context.Context, closingIDs []uuid.UUID) (map[uuid.UUID][]domain.RevenueLine, map[uuid.UUID][]domain.ExpenseLine, error) {
	revs := make(map[uuid.UUID][]domain.RevenueLine, len(closingIDs))
	exps := make(map[uuid.UUID][]domain.ExpenseLine, len(closingIDs))
	if len(closingIDs) == 0 {
		return revs, exps, nil
	}

	{
		rows, err := s.pool.Query(ctx,
			`select closing_id, method, amount
			   from closing_revenues
			  where closing_id = any($1)`,
			closingIDs,
		)
		if err != nil {
			return nil, nil, err
		}
		for rows.Next() {
			var cid uuid.UUID
			var line domain.RevenueLine
			if err := rows.Scan(&cid, &line.Method, &line.Amount); err != nil {
				rows.Close()
				return nil, nil, err
			}
			revs[cid] = append(revs[cid], line)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
	}

	{
		rows, err := s.pool.Query(ctx,
			`select closing_id, category, label, amount
			   from closing_expenses
			  where closing_id = any($1)`,
			closingIDs,
		)
		if err != nil {
			return nil, nil, err
		}
		for rows.Next() {
			var cid uuid.UUID
			var line domain.ExpenseLine
			if err := rows.Scan(&cid, &line.Category, &line.Label, &line.Amount); err != nil {
				rows.Close()
				return nil, nil, err
			}
			exps[cid] = append(exps[cid], line)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
	}

	return revs, exps, nil
}

// ListClosings returns all closings owned by the user, optionally filtered
// to one shop, newest first. Each closing includes its line breakdowns —
// the client computes revenue/expense/diff aggregates locally because the
// `'cash'` slug invariant lives there.
func (s *Store) ListClosings(ctx context.Context, ownerID uuid.UUID, shopID *uuid.UUID) ([]domain.Closing, error) {
	q := `
		select c.id, c.shop_id, c.date, c.float_open, c.float_close, c.note
		from closings c
		join shops s on s.id = c.shop_id
		where s.owner_id = $1
	`
	args := []any{ownerID}
	if shopID != nil {
		q += ` and c.shop_id = $2`
		args = append(args, *shopID)
	}
	q += ` order by c.date desc`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	out := make([]domain.Closing, 0)
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var c domain.Closing
		var date time.Time
		if err := rows.Scan(&c.ID, &c.ShopID, &date, &c.FloatOpen, &c.FloatClose, &c.Note); err != nil {
			rows.Close()
			return nil, err
		}
		c.Date = domain.FormatDate(date)
		out = append(out, c)
		ids = append(ids, c.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	revs, exps, err := s.loadClosingLines(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Revenues = revs[out[i].ID]
		out[i].Expenses = exps[out[i].ID]
		if out[i].Revenues == nil {
			out[i].Revenues = []domain.RevenueLine{}
		}
		if out[i].Expenses == nil {
			out[i].Expenses = []domain.ExpenseLine{}
		}
	}
	return out, nil
}

// GetClosingByDate returns the closing for a specific (shop, date), or
// (nil, nil) if none exists. The shop must be owned by `ownerID`.
func (s *Store) GetClosingByDate(ctx context.Context, ownerID, shopID uuid.UUID, date time.Time) (*domain.Closing, error) {
	const q = `
		select c.id, c.shop_id, c.date, c.float_open, c.float_close, c.note
		from closings c
		join shops s on s.id = c.shop_id
		where s.owner_id = $1 and c.shop_id = $2 and c.date = $3
	`
	var c domain.Closing
	var d time.Time
	err := s.pool.QueryRow(ctx, q, ownerID, shopID, date).
		Scan(&c.ID, &c.ShopID, &d, &c.FloatOpen, &c.FloatClose, &c.Note)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Date = domain.FormatDate(d)
	revs, exps, err := s.loadClosingLines(ctx, []uuid.UUID{c.ID})
	if err != nil {
		return nil, err
	}
	c.Revenues = revs[c.ID]
	c.Expenses = exps[c.ID]
	if c.Revenues == nil {
		c.Revenues = []domain.RevenueLine{}
	}
	if c.Expenses == nil {
		c.Expenses = []domain.ExpenseLine{}
	}
	return &c, nil
}

// GetClosingByID returns a single closing, scoped to the owner.
func (s *Store) GetClosingByID(ctx context.Context, ownerID, closingID uuid.UUID) (*domain.Closing, error) {
	const q = `
		select c.id, c.shop_id, c.date, c.float_open, c.float_close, c.note
		from closings c
		join shops s on s.id = c.shop_id
		where s.owner_id = $1 and c.id = $2
	`
	var c domain.Closing
	var d time.Time
	err := s.pool.QueryRow(ctx, q, ownerID, closingID).
		Scan(&c.ID, &c.ShopID, &d, &c.FloatOpen, &c.FloatClose, &c.Note)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Date = domain.FormatDate(d)
	revs, exps, err := s.loadClosingLines(ctx, []uuid.UUID{c.ID})
	if err != nil {
		return nil, err
	}
	c.Revenues = revs[c.ID]
	c.Expenses = exps[c.ID]
	if c.Revenues == nil {
		c.Revenues = []domain.RevenueLine{}
	}
	if c.Expenses == nil {
		c.Expenses = []domain.ExpenseLine{}
	}
	return &c, nil
}

// SaveClosingInput is what the handler passes to the store.
type SaveClosingInput struct {
	ShopID     uuid.UUID
	Date       time.Time
	FloatOpen  float64
	FloatClose float64
	Note       *string
	Revenues   []domain.RevenueLine
	Expenses   []domain.ExpenseLine
}

// SaveClosing upserts the closing on (shop_id, date) and replaces all child
// rows. Idempotent: re-saving the same date does not accumulate duplicates.
//
// Verifies shop ownership at the top — if the shop doesn't belong to the
// caller, returns (uuid.Nil, nil, ErrShopNotOwned) without touching anything.
func (s *Store) SaveClosing(ctx context.Context, ownerID uuid.UUID, in SaveClosingInput) (uuid.UUID, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Ownership check. If the shop isn't owned by the caller, the upsert
	// would either succeed (creating an orphaned closing) or fail awkwardly
	// — explicit lookup makes the 404 clean.
	var ownerCheck uuid.UUID
	err = tx.QueryRow(ctx,
		`select owner_id from shops where id = $1`,
		in.ShopID,
	).Scan(&ownerCheck)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && ownerCheck != ownerID) {
		return uuid.Nil, ErrShopNotOwned
	}
	if err != nil {
		return uuid.Nil, err
	}

	// 1) upsert the closing row.
	const upsert = `
		insert into closings (shop_id, entered_by, date, float_open, float_close, note)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (shop_id, date) do update
		  set float_open  = excluded.float_open,
		      float_close = excluded.float_close,
		      note        = excluded.note,
		      entered_by  = excluded.entered_by
		returning id
	`
	var closingID uuid.UUID
	if err := tx.QueryRow(ctx, upsert,
		in.ShopID, ownerID, in.Date, in.FloatOpen, in.FloatClose, in.Note,
	).Scan(&closingID); err != nil {
		return uuid.Nil, err
	}

	// 2) replace revenue lines.
	if _, err := tx.Exec(ctx, `delete from closing_revenues where closing_id = $1`, closingID); err != nil {
		return uuid.Nil, err
	}
	if len(in.Revenues) > 0 {
		batch := &pgx.Batch{}
		for _, r := range in.Revenues {
			batch.Queue(
				`insert into closing_revenues (closing_id, method, amount) values ($1, $2, $3)`,
				closingID, r.Method, r.Amount,
			)
		}
		br := tx.SendBatch(ctx, batch)
		for range in.Revenues {
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return uuid.Nil, fmt.Errorf("insert revenue: %w", err)
			}
		}
		if err := br.Close(); err != nil {
			return uuid.Nil, err
		}
	}

	// 3) replace expense lines.
	if _, err := tx.Exec(ctx, `delete from closing_expenses where closing_id = $1`, closingID); err != nil {
		return uuid.Nil, err
	}
	if len(in.Expenses) > 0 {
		batch := &pgx.Batch{}
		for _, e := range in.Expenses {
			batch.Queue(
				`insert into closing_expenses (closing_id, category, label, amount) values ($1, $2, $3, $4)`,
				closingID, e.Category, e.Label, e.Amount,
			)
		}
		br := tx.SendBatch(ctx, batch)
		for range in.Expenses {
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return uuid.Nil, fmt.Errorf("insert expense: %w", err)
			}
		}
		if err := br.Close(); err != nil {
			return uuid.Nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return closingID, nil
}

// ErrShopNotOwned is returned when SaveClosing is called against a shop_id
// that doesn't belong to the authenticated user.
var ErrShopNotOwned = errors.New("shop not owned by caller")
