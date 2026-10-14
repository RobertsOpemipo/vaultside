package server

import (
    "encoding/json"
    "log/slog"
    "net/http"
    "strconv"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/vaultside/vaultside/apps/api/internal/auth"
    "github.com/vaultside/vaultside/apps/api/internal/xapi"
)

// XAPIHandlers serves POST and GET /v1/xapi/statements.
type XAPIHandlers struct {
    pool *pgxpool.Pool
}

// NewXAPIHandlers builds an XAPIHandlers.
func NewXAPIHandlers(pool *pgxpool.Pool) *XAPIHandlers {
    return &XAPIHandlers{pool: pool}
}

// Post accepts a single statement or an array of statements, per the spec.
// Returns 200 with the array of statement IDs that were stored.
func (h *XAPIHandlers) Post(w http.ResponseWriter, r *http.Request) {
    claims, ok := auth.FromContext(r.Context())
    if !ok {
        scormHTTPError(w, http.StatusUnauthorized, "unauthenticated")
        return
    }

    var raw json.RawMessage
    if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
        scormHTTPError(w, http.StatusBadRequest, "invalid json")
        return
    }

    var stmts []xapi.Statement
    if len(raw) > 0 && raw[0] == '[' {
        if err := json.Unmarshal(raw, &stmts); err != nil {
            scormHTTPError(w, http.StatusBadRequest, "invalid statement array")
            return
        }
    } else {
        var one xapi.Statement
        if err := json.Unmarshal(raw, &one); err != nil {
            scormHTTPError(w, http.StatusBadRequest, "invalid statement")
            return
        }
        stmts = []xapi.Statement{one}
    }
    if len(stmts) == 0 {
        scormHTTPError(w, http.StatusBadRequest, "no statements")
        return
    }

    now := time.Now().UTC()
    for i := range stmts {
        if err := stmts[i].Validate(); err != nil {
            scormHTTPError(w, http.StatusBadRequest, err.Error())
            return
        }
        stmts[i].Normalize(now)
    }

    ctx := r.Context()
    tx, err := h.pool.Begin(ctx)
    if err != nil {
        slog.Error("xapi post: begin", "err", err)
        scormHTTPError(w, http.StatusServiceUnavailable, "db unavailable")
        return
    }
    defer tx.Rollback(ctx)
    if err := scormSetTenant(ctx, tx, claims.TenantID, claims.UserID); err != nil {
        slog.Error("xapi post: set tenant", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "tenant setup failed")
        return
    }

    ids := make([]string, 0, len(stmts))
    for i := range stmts {
        sid, err := stmts[i].ParseStoredID()
        if err != nil {
            scormHTTPError(w, http.StatusBadRequest, "statement id must be a UUID")
            return
        }
        body, err := json.Marshal(stmts[i])
        if err != nil {
            slog.Error("xapi post: marshal", "err", err)
            scormHTTPError(w, http.StatusInternalServerError, "encode failed")
            return
        }
        _, err = tx.Exec(ctx, `
            INSERT INTO xapi_statements
                (tenant_id, user_id, statement_id, verb, object_id, raw)
            VALUES ($1, $2, $3, $4, $5, $6)
            ON CONFLICT (statement_id) DO NOTHING
        `, claims.TenantID, claims.UserID, sid, stmts[i].Verb.ID, stmts[i].Object.ID, body)
        if err != nil {
            slog.Error("xapi post: insert", "err", err)
            scormHTTPError(w, http.StatusInternalServerError, "insert failed")
            return
        }
        ids = append(ids, stmts[i].ID)
    }

    if err := tx.Commit(ctx); err != nil {
        slog.Error("xapi post: commit", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "commit failed")
        return
    }
    scormWriteJSON(w, http.StatusOK, ids)
}

// List returns the caller's own statements, with optional filters:
//
//    ?verb=<iri>&object=<iri>&since=<rfc3339>&until=<rfc3339>&limit=<1..500>
func (h *XAPIHandlers) List(w http.ResponseWriter, r *http.Request) {
    claims, ok := auth.FromContext(r.Context())
    if !ok {
        scormHTTPError(w, http.StatusUnauthorized, "unauthenticated")
        return
    }
    q := r.URL.Query()

    limit := 50
    if v := q.Get("limit"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
            limit = n
        }
    }

    ctx := r.Context()
    tx, err := h.pool.Begin(ctx)
    if err != nil {
        slog.Error("xapi list: begin", "err", err)
        scormHTTPError(w, http.StatusServiceUnavailable, "db unavailable")
        return
    }
    defer tx.Rollback(ctx)
    if err := scormSetTenant(ctx, tx, claims.TenantID, claims.UserID); err != nil {
        slog.Error("xapi list: set tenant", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "tenant setup failed")
        return
    }

    sql := `SELECT raw FROM xapi_statements WHERE user_id = $1`
    args := []any{claims.UserID}
    argN := 1

    addArg := func(v any) string {
        argN++
        args = append(args, v)
        return "$" + strconv.Itoa(argN)
    }
    if v := q.Get("verb"); v != "" {
        sql += " AND verb = " + addArg(v)
    }
    if v := q.Get("object"); v != "" {
        sql += " AND object_id = " + addArg(v)
    }
    if v := q.Get("since"); v != "" {
        if t, err := time.Parse(time.RFC3339, v); err == nil {
            sql += " AND stored_at >= " + addArg(t)
        }
    }
    if v := q.Get("until"); v != "" {
        if t, err := time.Parse(time.RFC3339, v); err == nil {
            sql += " AND stored_at <= " + addArg(t)
        }
    }
    sql += " ORDER BY stored_at DESC LIMIT " + addArg(limit)

    rows, err := tx.Query(ctx, sql, args...)
    if err != nil {
        slog.Error("xapi list: query", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "query failed")
        return
    }
    defer rows.Close()

    out := make([]json.RawMessage, 0)
    for rows.Next() {
        var raw []byte
        if err := rows.Scan(&raw); err != nil {
            slog.Error("xapi list: scan", "err", err)
            scormHTTPError(w, http.StatusInternalServerError, "scan failed")
            return
        }
        out = append(out, json.RawMessage(raw))
    }
    if err := rows.Err(); err != nil {
        slog.Error("xapi list: rows", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "rows failed")
        return
    }
    if err := tx.Commit(ctx); err != nil {
        slog.Error("xapi list: commit", "err", err)
        scormHTTPError(w, http.StatusInternalServerError, "commit failed")
        return
    }
    scormWriteJSON(w, http.StatusOK, map[string]any{
        "statements": out,
        "count":      len(out),
    })
}