// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: addfeed.sql

package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const addFeed = `-- name: AddFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id) 
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at, updated_at, name, url, user_id
`

type AddFeedParams struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	Url       string
	UserID    uuid.UUID
}

type AddFeedRow struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	Url       string
	UserID    uuid.UUID
}

func (q *Queries) AddFeed(ctx context.Context, arg AddFeedParams) (AddFeedRow, error) {
	row := q.db.QueryRowContext(ctx, addFeed,
		arg.ID,
		arg.CreatedAt,
		arg.UpdatedAt,
		arg.Name,
		arg.Url,
		arg.UserID,
	)
	var i AddFeedRow
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Name,
		&i.Url,
		&i.UserID,
	)
	return i, err
}
