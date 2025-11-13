-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS pull_requests (
    pull_request_id VARCHAR(100) PRIMARY KEY,
    pull_request_name VARCHAR(500) NOT NULL,
    author_id VARCHAR(100) NOT NULL REFERENCES users(user_id),
    status VARCHAR(20) NOT NULL CHECK (status IN ('OPEN', 'MERGED')),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    merged_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pr_status ON pull_requests(status);
CREATE INDEX IF NOT EXISTS idx_pr_author ON pull_requests(author_id);
CREATE INDEX IF NOT EXISTS idx_pr_created_at ON pull_requests(created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pull_requests;
-- +goose StatementEnd
