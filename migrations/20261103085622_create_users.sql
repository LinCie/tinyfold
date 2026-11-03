-- +goose Up
CREATE TABLE users (
	id TEXT PRIMARY KEY NOT NULL,
	username TEXT NOT NULL UNIQUE CHECK (length(trim(username)) > 0),
	email TEXT NOT NULL UNIQUE CHECK (length(trim(email)) > 0),
	display_name TEXT NOT NULL CHECK (length(trim(display_name)) > 0),
	bio TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('admin', 'editor', 'author')),
	status TEXT NOT NULL CHECK (status IN ('invited', 'active', 'disabled')),
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE users;
