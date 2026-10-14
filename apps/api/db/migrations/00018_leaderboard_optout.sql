-- +goose Up

ALTER TABLE users ADD COLUMN leaderboard_optout boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS leaderboard_optout;