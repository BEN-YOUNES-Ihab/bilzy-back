package http

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bilzy/bilzy-back/internal/db/store"
)

type CategoryHandler struct {
	store *store.Store
}

func NewCategoryHandler(st *store.Store) *CategoryHandler { return &CategoryHandler{store: st} }

var allowedKinds = map[string]struct{}{
	"revenue": {},
	"expense": {},
}
var allowedColors = map[string]struct{}{
	"brand": {}, "accent": {}, "warn": {}, "info": {}, "ink3": {},
}

func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if _, ok := allowedKinds[kind]; !ok {
		WriteError(w, ErrValidation("kind must be 'revenue' or 'expense'"))
		return
	}
	includeArchived := r.URL.Query().Get("include_archived") == "true"

	uid := MustUserID(r.Context())
	cats, err := h.store.ListCategories(r.Context(), uid, kind, includeArchived)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, cats)
}

type createCategoryBody struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body createCategoryBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, ErrValidation("invalid json"))
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if _, ok := allowedKinds[body.Kind]; !ok {
		WriteError(w, ErrValidation("invalid kind"))
		return
	}
	if body.Name == "" {
		WriteError(w, ErrValidation("name is required"))
		return
	}
	if body.Icon == "" {
		WriteError(w, ErrValidation("icon is required"))
		return
	}
	if _, ok := allowedColors[body.Color]; !ok {
		WriteError(w, ErrValidation("invalid color"))
		return
	}

	uid := MustUserID(r.Context())
	cat, err := h.store.CreateCategory(r.Context(), uid, store.CreateCategoryInput{
		Kind:  body.Kind,
		Slug:  randomCustomSlug(),
		Name:  body.Name,
		Icon:  body.Icon,
		Color: body.Color,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			WriteError(w, ErrDuplicate("slug already in use"))
			return
		}
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, cat)
}

func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	catID, err := uuid.Parse(idParam)
	if err != nil {
		WriteError(w, ErrValidation("invalid category id"))
		return
	}

	// Decode into a map so we can detect whether `name` was sent (vs absent).
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		WriteError(w, ErrValidation("invalid json"))
		return
	}

	patch := store.UpdateCategoryPatch{}

	if rawName, ok := raw["name"]; ok {
		patch.NameSet = true
		// null in the JSON → clear the override (NameValue stays nil).
		if string(rawName) != "null" {
			var s string
			if err := json.Unmarshal(rawName, &s); err != nil {
				WriteError(w, ErrValidation("invalid name"))
				return
			}
			s = strings.TrimSpace(s)
			if s != "" {
				patch.NameValue = &s
			}
		}
	}
	if rawIcon, ok := raw["icon"]; ok {
		var s string
		if err := json.Unmarshal(rawIcon, &s); err != nil || s == "" {
			WriteError(w, ErrValidation("invalid icon"))
			return
		}
		patch.Icon = &s
	}
	if rawColor, ok := raw["color"]; ok {
		var s string
		if err := json.Unmarshal(rawColor, &s); err != nil {
			WriteError(w, ErrValidation("invalid color"))
			return
		}
		if _, ok := allowedColors[s]; !ok {
			WriteError(w, ErrValidation("invalid color"))
			return
		}
		patch.Color = &s
	}

	uid := MustUserID(r.Context())
	cat, err := h.store.UpdateCategory(r.Context(), uid, catID, patch)
	if err != nil {
		WriteError(w, err)
		return
	}
	if cat == nil {
		WriteError(w, ErrNotFound("category"))
		return
	}
	WriteJSON(w, http.StatusOK, cat)
}

func (h *CategoryHandler) Archive(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	catID, err := uuid.Parse(idParam)
	if err != nil {
		WriteError(w, ErrValidation("invalid category id"))
		return
	}
	uid := MustUserID(r.Context())
	ok, err := h.store.ArchiveCategory(r.Context(), uid, catID)
	if err != nil {
		WriteError(w, err)
		return
	}
	if !ok {
		WriteError(w, ErrNotFound("category"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type reorderBody struct {
	Kind string   `json:"kind"`
	IDs  []string `json:"ids"`
}

func (h *CategoryHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	var body reorderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, ErrValidation("invalid json"))
		return
	}
	if _, ok := allowedKinds[body.Kind]; !ok {
		WriteError(w, ErrValidation("invalid kind"))
		return
	}
	ids := make([]uuid.UUID, 0, len(body.IDs))
	for _, s := range body.IDs {
		id, err := uuid.Parse(s)
		if err != nil {
			WriteError(w, ErrValidation("invalid id in list"))
			return
		}
		ids = append(ids, id)
	}

	uid := MustUserID(r.Context())
	if err := h.store.ReorderCategories(r.Context(), uid, body.Kind, ids); err != nil {
		WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// randomCustomSlug mirrors the frontend's `cust_<8 random>` pattern. We could
// generate it client-side too, but server-side keeps the shape uniform.
func randomCustomSlug() string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return "cust_" + string(buf)
}
