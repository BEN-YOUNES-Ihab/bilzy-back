package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bilzy/bilzy-back/internal/db/store"
)

type ShopHandler struct {
	store *store.Store
}

func NewShopHandler(st *store.Store) *ShopHandler { return &ShopHandler{store: st} }

func (h *ShopHandler) List(w http.ResponseWriter, r *http.Request) {
	uid := MustUserID(r.Context())
	shops, err := h.store.ListShops(r.Context(), uid)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, shops)
}

type createShopBody struct {
	Name    string  `json:"name"`
	Address *string `json:"address"`
	Color   *string `json:"color"`
}

func (h *ShopHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body createShopBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, ErrValidation("invalid json"))
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		WriteError(w, ErrValidation("name is required"))
		return
	}
	if len(body.Name) > 80 {
		WriteError(w, ErrValidation("name too long"))
		return
	}

	uid := MustUserID(r.Context())
	sh, err := h.store.CreateShop(r.Context(), uid, store.CreateShopInput{
		Name:    body.Name,
		Address: body.Address,
		Color:   body.Color,
	})
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, sh)
}

type updateShopBody struct {
	Name    *string `json:"name"`
	Address *string `json:"address"`
	Color   *string `json:"color"`
}

func (h *ShopHandler) Update(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	shopID, err := uuid.Parse(idParam)
	if err != nil {
		WriteError(w, ErrValidation("invalid shop id"))
		return
	}

	var body updateShopBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, ErrValidation("invalid json"))
		return
	}
	if body.Name != nil {
		trimmed := strings.TrimSpace(*body.Name)
		if trimmed == "" {
			WriteError(w, ErrValidation("name cannot be empty"))
			return
		}
		if len(trimmed) > 80 {
			WriteError(w, ErrValidation("name too long"))
			return
		}
		body.Name = &trimmed
	}

	uid := MustUserID(r.Context())
	sh, err := h.store.UpdateShop(r.Context(), uid, shopID, store.UpdateShopInput{
		Name:    body.Name,
		Address: body.Address,
		Color:   body.Color,
	})
	if err != nil {
		WriteError(w, err)
		return
	}
	if sh == nil {
		WriteError(w, ErrNotFound("shop"))
		return
	}
	WriteJSON(w, http.StatusOK, sh)
}
