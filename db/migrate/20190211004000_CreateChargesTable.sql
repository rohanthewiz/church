
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Holds local record of payment charges
create table charges
(
	id bigserial not null
		constraint charges_pkey
			primary key,
	created_at timestamp with time zone,
	updated_at timestamp with time zone,
	customer_id text,
	customer_name text not null,
	description text,
	receipt_number text,
	receipt_url text,
	payment_token text not null,
	captured boolean default false,
	paid boolean default false,
	amount_paid bigint,
	refunded boolean default false,
	amount_refunded bigint,
	meta text
);

alter table charges
  owner to devuser; -- be sure to change to the owner of the production DB

create index idx_created_at
  on charges (created_at);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE charges;

