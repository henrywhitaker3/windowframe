// Package postgres
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

func Open(ctx context.Context, url string, tracingEnabled bool) (*sql.DB, error) {
	var db *sql.DB
	var err error
	if tracingEnabled {
		db, err = otelsql.Open(
			"pgx",
			url,
			otelsql.WithSpanOptions(otelsql.SpanOptions{
				OmitConnResetSession: true,
				OmitConnectorConnect: true,
			}),
		)
	} else {
		db, err = sql.Open("pgx", url)
	}

	if err != nil {
		return nil, err
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return db, nil
}

func OpenPgx(ctx context.Context, url string, tracingEnabled bool) (*pgxpool.Pool, error) {
	conf, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	conf.ConnConfig.Tracer = otelpgx.NewTracer()
	conf.ConnConfig.ConnectTimeout = time.Second * 10
	// conf.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgxpool.NewWithConfig(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("create pool from config: %w", err)
	}

	if tracingEnabled {
		if err := otelpgx.RecordStats(conn); err != nil {
			return nil, fmt.Errorf("insturment pgx tracer: %w", err)
		}
	}

	return conn, nil
}

func OpenStd(pool *pgxpool.Pool) (*sql.DB, error) {
	db := stdlib.OpenDBFromPool(pool)
	return db, nil
}
