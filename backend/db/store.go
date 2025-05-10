package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func Init(db_url string, ctx *context.Context) (*Queries, error) {
	conn, err := pgx.Connect(*ctx, db_url)
	if err != nil {
		return nil, err
	}

	queries := New(conn)

	return  queries, nil
}
