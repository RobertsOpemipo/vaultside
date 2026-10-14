package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    log.Println("worker up (no jobs registered yet)")

    // Block until SIGINT (Ctrl+C) or SIGTERM, then exit cleanly.
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    <-ctx.Done()

    log.Println("worker shutting down")
    // Placeholder; Phase 1 replaces this with River's Stop().
    time.Sleep(100 * time.Millisecond)
}