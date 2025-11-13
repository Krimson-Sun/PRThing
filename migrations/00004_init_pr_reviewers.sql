-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS pr_reviewers (
    pull_request_id VARCHAR(100) NOT NULL REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE,
    user_id VARCHAR(100) NOT NULL REFERENCES users(user_id),
    assigned_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (pull_request_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_pr_reviewers_user_id ON pr_reviewers(user_id);
CREATE INDEX IF NOT EXISTS idx_pr_reviewers_assigned_at ON pr_reviewers(assigned_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pr_reviewers;
-- +goose StatementEnd
