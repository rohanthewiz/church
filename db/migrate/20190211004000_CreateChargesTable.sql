
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Holds local record of payment charges
create table charges
(
  id              bigserial not null
    constraint charges_pkey
      primary key,
  created_at      timestamp with time zone,
  updated_at      timestamp with time zone,
  customer_id     bigint,
  customer_name   text      not null,
  description     text      not null,
  receipt_number  text,
  receipt_url     text,
  payment_token   text      not null,
  captured        boolean default false,
  paid            boolean default false,
  refunded        boolean default false,
  amount_refunded integer,
  meta            text
);

alter table charges
  owner to devuser;

create unique index idx_created_at
  on charges (created_at);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE charges;
