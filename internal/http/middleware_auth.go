package http

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/bilzy/bilzy-back/internal/db/store"
	"github.com/bilzy/bilzy-back/internal/firebase"
)

// NewAuthMiddleware returns middleware that:
//
//  1. Extracts the bearer token from the Authorization header.
//  2. Verifies it with the Firebase Admin SDK.
//  3. Resolves the corresponding profiles.id (creating the row + seeding
//     default categories on first request — i.e. replacing the old Supabase
//     on_auth_user_created triggers).
//  4. Puts the internal user uuid on the request context.
//
// Any failure in steps 1–3 yields a 401.
func NewAuthMiddleware(fb *firebase.Client, st *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := bearerFrom(r.Header.Get("Authorization"))
			if tok == "" {
				WriteError(w, ErrUnauthorized("missing bearer token"))
				return
			}

			ctx := r.Context()
			claims, err := fb.Verify(ctx, tok)
			if err != nil {
				slog.Debug("verify token", "err", err)
				WriteError(w, ErrUnauthorized("invalid token"))
				return
			}

			profile, err := st.GetProfileByFirebaseUID(ctx, claims.UID)
			if err != nil {
				WriteError(w, ErrInternal("lookup profile: "+err.Error()))
				return
			}

			if profile == nil {
				first, last := splitDisplayName(claims.DisplayName)
				profile, err = st.BootstrapProfile(ctx, claims.UID, claims.Email, first, last)
				if err != nil {
					WriteError(w, ErrInternal("bootstrap profile: "+err.Error()))
					return
				}
				slog.Info("bootstrapped profile", "user_id", profile.ID, "firebase_uid", claims.UID)
			} else if claims.Email != "" && profile.Email != claims.Email {
				// Sync email when Firebase reports a change (verifyBeforeUpdateEmail
				// flow). Cheap UPDATE that only fires on divergence — usually a no-op.
				if err := st.SyncProfileEmail(ctx, profile.ID, claims.Email); err != nil {
					slog.Warn("sync email", "err", err, "user_id", profile.ID)
				} else {
					profile.Email = claims.Email
				}
			}

			ctx = WithUserID(ctx, profile.ID)
			ctx = WithFirebaseUID(ctx, claims.UID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerFrom(h string) string {
	const p = "Bearer "
	if len(h) <= len(p) || !strings.EqualFold(h[:len(p)], p) {
		return ""
	}
	return strings.TrimSpace(h[len(p):])
}

// splitDisplayName splits "First Last" into pointers used by BootstrapProfile.
// Returns (nil, nil) when displayName is empty so we don't insert empty strings.
func splitDisplayName(name string) (*string, *string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	parts := strings.SplitN(name, " ", 2)
	first := parts[0]
	if len(parts) == 1 {
		return &first, nil
	}
	last := strings.TrimSpace(parts[1])
	if last == "" {
		return &first, nil
	}
	return &first, &last
}
