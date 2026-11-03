package posts

import "time"

type PostStatus string

var (
	PostStatusDraft     PostStatus = "draft"
	PostStatusPublished PostStatus = "published"
	PostStatusArchived  PostStatus = "archived"
)

type Post struct {
	ID          string     `db:"id"`
	Title       string     `db:"title"`
	Slug        string     `db:"slug"`
	Content     string     `db:"content"`
	Status      PostStatus `db:"status"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	PublishedAt *time.Time `db:"published_at"`
	ArchivedAt  *time.Time `db:"archived_at"`
}
