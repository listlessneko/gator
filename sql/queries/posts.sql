-- name: CreatePost :one
INSERT INTO posts(id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  $7,
  $8
)
RETURNING *;

-- name: GetPostsForUser :many
WITH user_feeds AS (
  SELECT
    feeds.id AS feed_id
  FROM feeds
  WHERE user_id = $1
)

SELECT posts.*
FROM posts
INNER JOIN user_feeds ON user_feeds.feed_id = posts.feed_id
ORDER BY posts.published_at DESC
LIMIT $2;
