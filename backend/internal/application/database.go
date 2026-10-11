package application

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/Najah7/task2todaytodo/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newPool(ctx context.Context, database config.DatabaseConfig) (*pgxpool.Pool, error) {
	cfg, err := postgresPoolConfig(database)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection pool")
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to connect to database")
	}

	return pool, nil
}

func postgresPoolConfig(database config.DatabaseConfig) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(postgresConnectionString(database))
	if err != nil {
		return nil, fmt.Errorf("invalid database connection settings")
	}

	cfg.MaxConns = 10
	cfg.MinConns = 2
	return cfg, nil
}

func postgresConnectionString(database config.DatabaseConfig) string {
	postgresURL := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			database.User,
			database.Password,
		),
		Host: net.JoinHostPort(
			database.Host,
			database.Port,
		),
		Path: "/" + database.Name,
	}

	query := postgresURL.Query()
	query.Set("sslmode", database.SSLMode)
	postgresURL.RawQuery = query.Encode()

	return postgresURL.String()
}
