package antivirus

import (
    "context"
    "log"
)

type Result struct {
    Status string
    Threat string
}

type Scanner interface {
    Scan(ctx context.Context, key string, sizeBytes int64) (Result, error)
}

type NoopScanner struct{}

func (NoopScanner) Scan(_ context.Context, key string, sizeBytes int64) (Result, error) {
    log.Printf("antivirus[noop]: scanning %s (%d bytes) -> clean", key, sizeBytes)
    return Result{Status: "clean"}, nil
}