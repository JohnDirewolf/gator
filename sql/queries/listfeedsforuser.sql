-- name: ListFeedsForUser :many
SELECT feed_follows.feed_id, feeds.name AS feed_name, feeds.url AS feed_url FROM feed_follows 
INNER JOIN feeds 
ON  feed_follows.feed_id = feeds.id
WHERE feed_follows.user_id = $1;