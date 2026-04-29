-- =============================================================================
-- Bilzy initial schema.
--
-- Consolidates the previous Supabase migrations 0002+0003+0004 into a single
-- Neon-friendly migration with three structural changes:
--
--   1. No RLS. Authorization is enforced in the Go server.
--   2. No `default auth.uid()` on owner columns. Server passes them explicitly.
--   3. No FKs to `auth.users`. Profiles owns identity via `firebase_uid`,
--      and other tables FK to profiles(id).
--
-- The on-signup triggers from Supabase are gone; the Go middleware bootstraps
-- a profile + seeds default categories on the user's first authenticated
-- request.
--
-- The five `categories.color` slots, the `'cash'` revenue slug, and the
-- `closings(shop_id, date)` UNIQUE constraint are load-bearing for the client
-- and stay verbatim.
-- =============================================================================

-- +goose Up
-- +goose StatementBegin

create extension if not exists "pgcrypto";

-- ---- profiles ---------------------------------------------------------------

create table profiles (
  id            uuid primary key default gen_random_uuid(),
  firebase_uid  text not null unique,
  email         text not null,
  first_name    text,
  last_name     text,
  birthdate     date,
  created_at    timestamptz not null default now(),
  updated_at    timestamptz not null default now()
);

create index profiles_firebase_uid_idx on profiles(firebase_uid);

-- ---- shops ------------------------------------------------------------------

create table shops (
  id          uuid primary key default gen_random_uuid(),
  owner_id    uuid not null references profiles(id) on delete cascade,
  name        text not null check (length(name) between 1 and 80),
  address     text,
  color       text not null default '#FF6B47',
  created_at  timestamptz not null default now()
);

create index shops_owner_id_idx on shops(owner_id);

-- ---- closings ---------------------------------------------------------------

create table closings (
  id          uuid primary key default gen_random_uuid(),
  shop_id     uuid not null references shops(id) on delete cascade,
  entered_by  uuid not null references profiles(id),
  date        date not null,
  float_open  numeric not null default 0 check (float_open  >= 0),
  float_close numeric not null default 0 check (float_close >= 0),
  note        text,
  closed_at   timestamptz not null default now(),
  unique (shop_id, date)
);

create index closings_shop_date_idx on closings(shop_id, date desc);

-- ---- closing_revenues -------------------------------------------------------
-- `method` is a category slug (free-form string referencing categories.slug).
-- No FK / CHECK enum: the client filters historical rows against the user's
-- (possibly archived) category set.

create table closing_revenues (
  id          uuid primary key default gen_random_uuid(),
  closing_id  uuid not null references closings(id) on delete cascade,
  method      text not null,
  amount      numeric not null default 0 check (amount >= 0),
  unique (closing_id, method)
);

create index closing_revenues_closing_id_idx on closing_revenues(closing_id);

-- ---- closing_expenses -------------------------------------------------------

create table closing_expenses (
  id          uuid primary key default gen_random_uuid(),
  closing_id  uuid not null references closings(id) on delete cascade,
  category    text not null,
  label       text,
  amount      numeric not null default 0 check (amount >= 0)
);

create index closing_expenses_closing_id_idx on closing_expenses(closing_id);

-- ---- categories -------------------------------------------------------------

create table categories (
  id          uuid primary key default gen_random_uuid(),
  owner_id    uuid not null references profiles(id) on delete cascade,
  kind        text not null check (kind in ('revenue','expense')),
  slug        text not null check (slug ~ '^[a-z0-9_]+$' and length(slug) between 1 and 64),
  name        text,
  i18n_key    text,
  icon        text not null,
  color       text not null check (color in ('brand','accent','warn','info','ink3')),
  position    int  not null default 0,
  is_system   boolean not null default false,
  archived_at timestamptz,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now(),
  unique (owner_id, kind, slug)
);

create index categories_owner_kind_pos_idx
  on categories (owner_id, kind, position);

-- ---- updated_at triggers ----------------------------------------------------

-- +goose StatementEnd

-- +goose StatementBegin
create or replace function touch_profile_updated_at()
returns trigger
language plpgsql
as $$
begin
  new.updated_at := now();
  return new;
end;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
create trigger profiles_updated_at
  before update on profiles
  for each row execute function touch_profile_updated_at();
-- +goose StatementEnd

-- +goose StatementBegin
create or replace function touch_category_updated_at()
returns trigger
language plpgsql
as $$
begin
  new.updated_at := now();
  return new;
end;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
create trigger categories_updated_at
  before update on categories
  for each row execute function touch_category_updated_at();
-- +goose StatementEnd

-- ---- seed_default_categories ------------------------------------------------
-- Called by the Go middleware during first-login bootstrap. Idempotent.
-- The 'cash' revenue slug is load-bearing for the client float-diff math:
-- diff = floatClose - (floatOpen + cashRevenue - expenses).

-- +goose StatementBegin
create or replace function seed_default_categories(p_owner uuid)
returns void
language sql
as $$
  insert into categories
    (owner_id, kind, slug, i18n_key, icon, color, position, is_system)
  values
    (p_owner, 'revenue', 'cash',     'closing.paymentMethods.cash',     'cash',      'accent', 0, true),
    (p_owner, 'revenue', 'card',     'closing.paymentMethods.card',     'card',      'info',   1, true),
    (p_owner, 'revenue', 'check',    'closing.paymentMethods.check',    'check_doc', 'warn',   2, true),
    (p_owner, 'revenue', 'transfer', 'closing.paymentMethods.transfer', 'transfer',  'brand',  3, true),
    (p_owner, 'revenue', 'other',    'closing.paymentMethods.other',    'package',   'ink3',   4, true),
    (p_owner, 'expense', 'supplier',  'closing.expenseCategories.supplier',  'box',       'brand',  0, true),
    (p_owner, 'expense', 'salary',    'closing.expenseCategories.salary',    'users',     'info',   1, true),
    (p_owner, 'expense', 'rent',      'closing.expenseCategories.rent',      'home_shop', 'warn',   2, true),
    (p_owner, 'expense', 'utilities', 'closing.expenseCategories.utilities', 'bolt',      'accent', 3, true),
    (p_owner, 'expense', 'other',     'closing.expenseCategories.other',     'package',   'ink3',   4, true)
  on conflict (owner_id, kind, slug) do nothing;
$$;
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
drop function if exists seed_default_categories(uuid);
drop function if exists touch_category_updated_at() cascade;
drop function if exists touch_profile_updated_at() cascade;
drop table if exists categories cascade;
drop table if exists closing_expenses cascade;
drop table if exists closing_revenues cascade;
drop table if exists closings cascade;
drop table if exists shops cascade;
drop table if exists profiles cascade;
-- +goose StatementEnd
