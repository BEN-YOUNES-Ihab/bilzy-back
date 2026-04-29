package http

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyFirebaseUID
)

// WithUserID stores the internal profiles.id (uuid) on the request context.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, id)
}

// UserIDFrom returns the internal profiles.id from a request context.
// Handlers can rely on this being set when wired behind the auth middleware.
func UserIDFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return id, ok
}

// MustUserID is the convenience accessor for handlers that run behind the
// auth middleware — it panics if the context is missing the user, which is
// a programming error.
func MustUserID(ctx context.Context) uuid.UUID {
	id, ok := UserIDFrom(ctx)
	if !ok {
		panic("no user id on context (missing auth middleware?)")
	}
	return id
}

func WithFirebaseUID(ctx context.Context, uid string) context.Context {
	return context.WithValue(ctx, ctxKeyFirebaseUID, uid)
}

func FirebaseUIDFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyFirebaseUID).(string)
	return v, ok
}
