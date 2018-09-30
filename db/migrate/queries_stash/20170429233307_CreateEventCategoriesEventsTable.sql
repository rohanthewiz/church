
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
create table event_categories_events (
  event_category_id BIGINT NOT NULL,
  event_id BIGINT NOT NULL,
  PRIMARY KEY (event_category_id, event_id),
  FOREIGN KEY (event_category_id) REFERENCES event_categories (id),
  FOREIGN KEY (event_id) REFERENCES events (id)
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE event_categories_events;
