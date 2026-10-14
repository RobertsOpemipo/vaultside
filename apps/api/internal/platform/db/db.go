package db

import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
    *pgxpool.Pool
}

func New(ctx context.Context, url string) (*Pool, error) {
    cfg, err := pgxpool.ParseConfig(url)
    if err != nil {
        return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
    }
    cfg.MaxConns = 20
    cfg.MinConns = 2
    cfg.MaxConnLifetime = time.Hour
    cfg.MaxConnIdleTime = 10 * time.Minute

    p, err := pgxpool.NewWithConfig(ctx, cfg)
    if err != nil {
        return nil, fmt.Errorf("open pool: %w", err)
    }
    if err := p.Ping(ctx); err != nil {
        p.Close()
        return nil, fmt.Errorf("ping: %w", err)
    }
    return &Pool{Pool: p}, nil
}