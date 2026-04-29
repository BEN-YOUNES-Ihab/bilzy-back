package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/bilzy/bilzy-back/internal/http/middleware"
)

// Deps bundles everything handlers need. Keep it small — anything that needs
// to be used by more than one handler goes here.
type Deps struct {
	AuthMiddleware func(http.Handler) http.Handler
	CORSOrigins    []string

	Profile    *ProfileHandler
	Shops      *ShopHandler
	Categories *CategoryHandler
	Closings   *ClosingHandler
}

func NewRouter(d *Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recover)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(d.CORSOrigins))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(d.AuthMiddleware)

		r.Get("/me", func(w http.ResponseWriter, req *http.Request) {
			uid, _ := UserIDFrom(req.Context())
			WriteJSON(w, http.StatusOK, map[string]string{"user_id": uid.String()})
		})

		r.Get("/profile", d.Profile.Get)
		r.Put("/profile", d.Profile.Put)

		r.Get("/shops", d.Shops.List)
		r.Post("/shops", d.Shops.Create)
		r.Patch("/shops/{id}", d.Shops.Update)

		r.Get("/categories", d.Categories.List)
		r.Post("/categories", d.Categories.Create)
		r.Patch("/categories/{id}", d.Categories.Update)
		r.Post("/categories/{id}/archive", d.Categories.Archive)
		r.Post("/categories/reorder", d.Categories.Reorder)

		r.Get("/closings", d.Closings.List)
		r.Get("/closings/by-date", d.Closings.ByDate)
		r.Get("/closings/{id}", d.Closings.Get)
		r.Put("/closings", d.Closings.Save)
	})

	return r
}
