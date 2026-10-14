// Package xapi implements the minimal subset of the Experience API (xAPI)
// statement model Vaultside needs: parse, validate, normalize, store.
package xapi

import (
    "encoding/json"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
)

// Actor is the xAPI actor (Agent or Group).
type Actor struct {
    ObjectType string `json:"objectType,omitempty"`
    Name       string `json:"name,omitempty"`
    Mbox       string `json:"mbox,omitempty"`
    Account    *struct {
        HomePage string `json:"homePage"`
        Name     string `json:"name"`
    } `json:"account,omitempty"`
}

// Verb is the xAPI verb.
type Verb struct {
    ID      string            `json:"id"`
    Display map[string]string `json:"display,omitempty"`
}

// Object is the xAPI object (Activity, Agent, StatementRef).
type Object struct {
    ObjectType string         `json:"objectType,omitempty"`
    ID         string         `json:"id"`
    Definition map[string]any `json:"definition,omitempty"`
}

// Statement is what we accept and store. Unknown fields are preserved in the
// raw payload the handler writes to xapi_statements.raw.
type Statement struct {
    ID        string          `json:"id,omitempty"`
    Actor     Actor           `json:"actor"`
    Verb      Verb            `json:"verb"`
    Object    Object          `json:"object"`
    Result    json.RawMessage `json:"result,omitempty"`
    Context   json.RawMessage `json:"context,omitempty"`
    Timestamp string          `json:"timestamp,omitempty"`
    Stored    string          `json:"stored,omitempty"`
    Authority json.RawMessage `json:"authority,omitempty"`
    Version   string          `json:"version,omitempty"`
}

var (
    ErrInvalidStatement = errors.New("xapi: invalid statement")
    ErrMissingVerb      = fmt.Errorf("%w: verb.id required", ErrInvalidStatement)
    ErrMissingObject    = fmt.Errorf("%w: object.id required", ErrInvalidStatement)
)

// Validate enforces the minimum the spec requires and we enforce server-side.
func (s *Statement) Validate() error {
    if s.Verb.ID == "" {
        return ErrMissingVerb
    }
    if s.Object.ID == "" {
        return ErrMissingObject
    }
    return nil
}

// Normalize fills in server-controlled fields (id, timestamps).
func (s *Statement) Normalize(now time.Time) {
    if s.ID == "" {
        s.ID = uuid.NewString()
    }
    if s.Timestamp == "" {
        s.Timestamp = now.UTC().Format(time.RFC3339Nano)
    }
    if s.Stored == "" {
        s.Stored = s.Timestamp
    }
}

// ParseStoredID returns the statement id as a UUID.
func (s *Statement) ParseStoredID() (uuid.UUID, error) {
    return uuid.Parse(s.ID)
}