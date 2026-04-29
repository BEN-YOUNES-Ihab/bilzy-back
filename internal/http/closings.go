package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bilzy/bilzy-back/internal/db/store"
	"github.com/bilzy/bilzy-back/internal/domain"
)

type ClosingHandler struct {
	store *store.Store
}

func NewClosingHandler(st *store.Store) *ClosingHandler { return &ClosingHandler{store: st} }

func (h *ClosingHandler) List(w http.ResponseWriter, r *http.Request) {
	uid := MustUserID(r.Context())

	var shopID *uuid.UUID
	if s := r.URL.Query().Get("shop_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			WriteError(w, ErrValidation("invalid shop_id"))
			return
		}
		shopID = &id
	}

	out, err := h.store.ListClosings(r.Context(), uid, shopID)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

func (h *ClosingHandler) ByDate(w http.ResponseWriter, r *http.Request) {
	uid := MustUserID(r.Context())

	shopParam := r.URL.Query().Get("shop_id")
	dateParam := r.URL.Query().Get("date")
	if shopParam == "" || dateParam == "" {
		WriteError(w, ErrValidation("shop_id and date are required"))
		return
	}
	shopID, err := uuid.Parse(shopParam)
	if err != nil {
		WriteError(w, ErrValidation("invalid shop_id"))
		return
	}
	date, err := domain.ParseDate(dateParam)
	if err != nil {
		WriteError(w, ErrValidation("invalid date (expected YYYY-MM-DD)"))
		return
	}

	c, err := h.store.GetClosingByDate(r.Context(), uid, shopID, date)
	if err != nil {
		WriteError(w, err)
		return
	}
	if c == nil {
		WriteJSON(w, http.StatusOK, nil)
		return
	}
	WriteJSON(w, http.StatusOK, c)
}

func (h *ClosingHandler) Get(w http.ResponseWriter, r *http.Request) {
	uid := MustUserID(r.Context())
	idParam := chi.URLParam(r, "id")
	closingID, err := uuid.Parse(idParam)
	if err != nil {
		WriteError(w, ErrValidation("invalid closing id"))
		return
	}
	c, err := h.store.GetClosingByID(r.Context(), uid, closingID)
	if err != nil {
		WriteError(w, err)
		return
	}
	if c == nil {
		WriteError(w, ErrNotFound("closing"))
		return
	}
	WriteJSON(w, http.StatusOK, c)
}

type saveClosingBody struct {
	ShopID     string                `json:"shopId"`
	Date       string                `json:"date"`
	FloatOpen  float64               `json:"floatOpen"`
	FloatClose float64               `json:"floatClose"`
	Note       *string               `json:"note"`
	Revenues   []domain.RevenueLine  `json:"revenues"`
	Expenses   []domain.ExpenseLine  `json:"expenses"`
}

func (h *ClosingHandler) Save(w http.ResponseWriter, r *http.Request) {
	var body saveClosingBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, ErrValidation("invalid json"))
		return
	}
	shopID, err := uuid.Parse(body.ShopID)
	if err != nil {
		WriteError(w, ErrValidation("invalid shopId"))
		return
	}
	date, err := domain.ParseDate(body.Date)
	if err != nil {
		WriteError(w, ErrValidation("invalid date (expected YYYY-MM-DD)"))
		return
	}
	if body.FloatOpen < 0 || body.FloatClose < 0 {
		WriteError(w, ErrValidation("floats must be >= 0"))
		return
	}
	for _, r := range body.Revenues {
		if r.Method == "" || r.Amount < 0 {
			WriteError(w, ErrValidation("invalid revenue line"))
			return
		}
	}
	for _, e := range body.Expenses {
		if e.Category == "" || e.Amount < 0 {
			WriteError(w, ErrValidation("invalid expense line"))
			return
		}
	}
	if body.Note != nil {
		v := strings.TrimSpace(*body.Note)
		if v == "" {
			body.Note = nil
		} else {
			body.Note = &v
		}
	}

	uid := MustUserID(r.Context())
	id, err := h.store.SaveClosing(r.Context(), uid, store.SaveClosingInput{
		ShopID:     shopID,
		Date:       date,
		FloatOpen:  body.FloatOpen,
		FloatClose: body.FloatClose,
		Note:       body.Note,
		Revenues:   body.Revenues,
		Expenses:   body.Expenses,
	})
	if errors.Is(err, store.ErrShopNotOwned) {
		WriteError(w, ErrNotFound("shop"))
		return
	}
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"id": id.String()})
}
