-- +goose Up
CREATE TABLE posts (
	id TEXT PRIMARY KEY NOT NULL,
	title TEXT NOT NULL CHECK (length(trim(title)) > 0),
	slug TEXT NOT NULL UNIQUE CHECK (length(trim(slug)) > 0),
	content TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('draft', 'published', 'archived')),
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	published_at TIMESTAMP NULL,
	archived_at TIMESTAMP NULL
);

-- +goose Down
DROP TABLE posts;
