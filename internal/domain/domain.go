// Package domain holds the data types exchanged between the store, HTTP
// handlers, and (via JSON) the frontend. Names and JSON tags are aligned
// with the existing client types in `bilzy-front/lib/data/types.ts`.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// ---- profile ----------------------------------------------------------------

type Profile struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FirstName *string   `json:"firstName"`
	LastName  *string   `json:"lastName"`
	Birthdate *string   `json:"birthdate"` // YYYY-MM-DD
}

// ---- shop -------------------------------------------------------------------

type Shop struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Address *string   `json:"address"`
	Color   string    `json:"color"`
}

// ---- category ---------------------------------------------------------------

type Category struct {
	ID         uuid.UUID `json:"id"`
	Kind       string    `json:"kind"` // 'revenue' | 'expense'
	Slug       string    `json:"slug"`
	Name       *string   `json:"name"`
	I18nKey    *string   `json:"i18nKey"`
	Icon       string    `json:"icon"`
	Color      string    `json:"color"`
	Position   int32     `json:"position"`
	IsSystem   bool      `json:"isSystem"`
	ArchivedAt *string   `json:"archivedAt"` // ISO timestamp; nil = active
}

// ---- closing ----------------------------------------------------------------

type RevenueLine struct {
	Method string  `json:"method"`
	Amount float64 `json:"amount"`
}

type ExpenseLine struct {
	Category string  `json:"category"`
	Label    *string `json:"label"`
	Amount   float64 `json:"amount"`
}

// Closing is the wire shape. The client computes aggregates (revenue,
// expenses, net, diff) locally because the cash-slug invariant lives there.
type Closing struct {
	ID         uuid.UUID     `json:"id"`
	ShopID     uuid.UUID     `json:"shopId"`
	Date       string        `json:"date"` // YYYY-MM-DD
	FloatOpen  float64       `json:"floatOpen"`
	FloatClose float64       `json:"floatClose"`
	Note       *string       `json:"note"`
	Revenues   []RevenueLine `json:"revenues"`
	Expenses   []ExpenseLine `json:"expenses"`
}

// FormatDate formats a calendar date as YYYY-MM-DD.
func FormatDate(t time.Time) string { return t.Format("2006-01-02") }

// ParseDate parses a YYYY-MM-DD string. Used at HTTP boundaries.
func ParseDate(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }
