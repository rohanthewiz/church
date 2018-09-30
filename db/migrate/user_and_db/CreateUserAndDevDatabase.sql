
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE USER myuser with password 'secret' CREATEDB;
CREATE DATABASE church_development WITH OWNER myuser;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP DATABASE church_development;
