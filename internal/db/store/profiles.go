package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bilzy/bilzy-back/internal/domain"
)

// GetProfileByFirebaseUID returns the profile associated with a Firebase UID,
// or (nil, nil) if no row exists yet.
func (s *Store) GetProfileByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.Profile, error) {
	const q = `
		select id, email, first_name, last_name, birthdate
		from profiles
		where firebase_uid = $1
	`
	var p domain.Profile
	var birthdate *time.Time
	err := s.pool.QueryRow(ctx, q, firebaseUID).
		Scan(&p.ID, &p.Email, &p.FirstName, &p.LastName, &birthdate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if birthdate != nil {
		s := domain.FormatDate(*birthdate)
		p.Birthdate = &s
	}
	return &p, nil
}

// GetProfileByID returns the profile by internal uuid (for /api/profile reads).
func (s *Store) GetProfileByID(ctx context.Context, id uuid.UUID) (*domain.Profile, error) {
	const q = `
		select id, email, first_name, last_name, birthdate
		from profiles
		where id = $1
	`
	var p domain.Profile
	var birthdate *time.Time
	err := s.pool.QueryRow(ctx, q, id).
		Scan(&p.ID, &p.Email, &p.FirstName, &p.LastName, &birthdate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if birthdate != nil {
		s := domain.FormatDate(*birthdate)
		p.Birthdate = &s
	}
	return &p, nil
}

// BootstrapProfile creates a profile row for a freshly-authenticated Firebase
// user and seeds the 10 default categories. Single transaction. Idempotent —
// if a row already exists for the firebase_uid, returns the existing row
// without re-seeding (categories' ON CONFLICT covers the seed half).
func (s *Store) BootstrapProfile(ctx context.Context, firebaseUID, email string, firstName, lastName *string) (*domain.Profile, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertProfile = `
		insert into profiles (firebase_uid, email, first_name, last_name)
		values ($1, $2, $3, $4)
		on conflict (firebase_uid) do update
		  set email = excluded.email
		returning id, email, first_name, last_name, birthdate
	`
	var p domain.Profile
	var birthdate *time.Time
	if err := tx.QueryRow(ctx, insertProfile, firebaseUID, email, firstName, lastName).
		Scan(&p.ID, &p.Email, &p.FirstName, &p.LastName, &birthdate); err != nil {
		return nil, err
	}
	if birthdate != nil {
		ds := domain.FormatDate(*birthdate)
		p.Birthdate = &ds
	}

	if _, err := tx.Exec(ctx, `select seed_default_categories($1)`, p.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &p, nil
}

// SyncProfileEmail updates profiles.email when Firebase reports a different
// email for the user. Called from the auth middleware on every request where
// it diverges (cheap UPDATE; almost always a no-op).
func (s *Store) SyncProfileEmail(ctx context.Context, id uuid.UUID, email string) error {
	_, err := s.pool.Exec(ctx,
		`update profiles set email = $1 where id = $2 and email is distinct from $1`,
		email, id,
	)
	return err
}

// UpdateProfileInput patches first_name / last_name / birthdate / email.
// Pointers distinguish "leave unchanged" (nil) from "set to NULL"
// (pointer to empty string for nullable text, ParseDate error for birthdate).
// Email is a string (always required to be non-empty if provided).
type UpdateProfileInput struct {
	Email     *string
	FirstName *string
	LastName  *string
	Birthdate *string // YYYY-MM-DD; pointer to empty string clears it
}

// UpdateProfile applies a patch and returns the updated row.
func (s *Store) UpdateProfile(ctx context.Context, id uuid.UUID, in UpdateProfileInput) (*domain.Profile, error) {
	// COALESCE-style update: keep current value when arg is null.
	const q = `
		update profiles
		set
		  email      = coalesce($2, email),
		  first_name = case when $3::int = 1 then $4::text else first_name end,
		  last_name  = case when $5::int = 1 then $6::text else last_name  end,
		  birthdate  = case when $7::int = 1 then $8::date else birthdate  end
		where id = $1
		returning id, email, first_name, last_name, birthdate
	`

	var fnSet int
	var fnVal *string
	if in.FirstName != nil {
		fnSet = 1
		v := *in.FirstName
		if v == "" {
			fnVal = nil
		} else {
			fnVal = &v
		}
	}
	var lnSet int
	var lnVal *string
	if in.LastName != nil {
		lnSet = 1
		v := *in.LastName
		if v == "" {
			lnVal = nil
		} else {
			lnVal = &v
		}
	}
	var bdSet int
	var bdVal *time.Time
	if in.Birthdate != nil {
		bdSet = 1
		if *in.Birthdate != "" {
			t, err := domain.ParseDate(*in.Birthdate)
			if err != nil {
				return nil, err
			}
			bdVal = &t
		}
	}

	var p domain.Profile
	var birthdate *time.Time
	err := s.pool.QueryRow(ctx, q,
		id, in.Email,
		fnSet, fnVal,
		lnSet, lnVal,
		bdSet, bdVal,
	).Scan(&p.ID, &p.Email, &p.FirstName, &p.LastName, &birthdate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if birthdate != nil {
		ds := domain.FormatDate(*birthdate)
		p.Birthdate = &ds
	}
	return &p, nil
}
