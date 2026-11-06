-- +goose Up
CREATE TABLE refresh_tokens (
	token_id TEXT PRIMARY KEY NOT NULL,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at TIMESTAMP NOT NULL,
	revoked_at TIMESTAMP
);

CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens(user_id);

-- +goose Down
DROP TABLE refresh_tokens;
