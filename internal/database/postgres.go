package database

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/newrelic/go-agent/v3/integrations/nrpq"
)

type PostgresDB struct {
	*sqlx.DB
}

func NewPostgres(databaseURL string, maxConns, maxIdleConns int) (*PostgresDB, error) {
	// Use nrpq driver for New Relic instrumentation
	db, err := sqlx.Connect("nrpostgres", databaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return &PostgresDB{DB: db}, nil
}

func (p *PostgresDB) Close() error {
	return p.DB.Close()
}

func (p *PostgresDB) Health(ctx context.Context) error {
	return p.PingContext(ctx)
}
