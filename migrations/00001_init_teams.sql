-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS teams (
    team_name VARCHAR(100) PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_teams_created_at ON teams(created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS teams;
-- +goose StatementEnd
