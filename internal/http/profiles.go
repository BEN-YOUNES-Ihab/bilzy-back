package http

import (
	"encoding/json"
	"net/http"

	"github.com/bilzy/bilzy-back/internal/db/store"
)

type ProfileHandler struct {
	store *store.Store
}

func NewProfileHandler(st *store.Store) *ProfileHandler {
	return &ProfileHandler{store: st}
}

func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := MustUserID(r.Context())
	p, err := h.store.GetProfileByID(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}
	if p == nil {
		WriteError(w, ErrNotFound("profile"))
		return
	}
	WriteJSON(w, http.StatusOK, p)
}

type updateProfileBody struct {
	Email     *string `json:"email"`
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
	Birthdate *string `json:"birthdate"`
}

func (h *ProfileHandler) Put(w http.ResponseWriter, r *http.Request) {
	var body updateProfileBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, ErrValidation("invalid json"))
		return
	}
	id := MustUserID(r.Context())
	p, err := h.store.UpdateProfile(r.Context(), id, store.UpdateProfileInput{
		Email:     body.Email,
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Birthdate: body.Birthdate,
	})
	if err != nil {
		WriteError(w, err)
		return
	}
	if p == nil {
		WriteError(w, ErrNotFound("profile"))
		return
	}
	WriteJSON(w, http.StatusOK, p)
}
