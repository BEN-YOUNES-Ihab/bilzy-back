package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bilzy/bilzy-back/internal/domain"
)

const defaultShopColor = "#FF6B47"

func (s *Store) ListShops(ctx context.Context, ownerID uuid.UUID) ([]domain.Shop, error) {
	const q = `
		select id, name, address, color
		from shops
		where owner_id = $1
		order by name asc
	`
	rows, err := s.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Shop, 0)
	for rows.Next() {
		var sh domain.Shop
		if err := rows.Scan(&sh.ID, &sh.Name, &sh.Address, &sh.Color); err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

type CreateShopInput struct {
	Name    string
	Address *string
	Color   *string // nil → server default
}

func (s *Store) CreateShop(ctx context.Context, ownerID uuid.UUID, in CreateShopInput) (*domain.Shop, error) {
	color := defaultShopColor
	if in.Color != nil && *in.Color != "" {
		color = *in.Color
	}
	const q = `
		insert into shops (owner_id, name, address, color)
		values ($1, $2, $3, $4)
		returning id, name, address, color
	`
	var sh domain.Shop
	if err := s.pool.QueryRow(ctx, q, ownerID, in.Name, in.Address, color).
		Scan(&sh.ID, &sh.Name, &sh.Address, &sh.Color); err != nil {
		return nil, err
	}
	return &sh, nil
}

type UpdateShopInput struct {
	Name    *string
	Address *string // pointer to empty string clears it
	Color   *string
}

// UpdateShop patches a shop owned by ownerID. Returns (nil, nil) if the
// shop doesn't exist or isn't owned by the caller — handlers map this to 404.
func (s *Store) UpdateShop(ctx context.Context, ownerID, shopID uuid.UUID, in UpdateShopInput) (*domain.Shop, error) {
	const q = `
		update shops
		set
		  name    = coalesce($3, name),
		  address = case when $4::int = 1 then $5::text else address end,
		  color   = coalesce($6, color)
		where id = $1 and owner_id = $2
		returning id, name, address, color
	`
	var addrSet int
	var addrVal *string
	if in.Address != nil {
		addrSet = 1
		v := *in.Address
		if v == "" {
			addrVal = nil
		} else {
			addrVal = &v
		}
	}

	var sh domain.Shop
	err := s.pool.QueryRow(ctx, q,
		shopID, ownerID,
		in.Name,
		addrSet, addrVal,
		in.Color,
	).Scan(&sh.ID, &sh.Name, &sh.Address, &sh.Color)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sh, nil
}
