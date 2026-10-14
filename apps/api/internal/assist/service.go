package assist

import (
    "context"
    "errors"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
)

// Service retrieves tenant-scoped content for a lesson, hands it to the
// provider, and returns the answer. It never opens its own transaction; the
// caller passes a tenant-scoped pgx.Tx.
type Service struct {
    provider Provider
}

// NewService wraps a provider.
func NewService(p Provider) *Service {
    return &Service{provider: p}
}

// ProviderName is exposed for the status endpoint.
func (s *Service) ProviderName() string { return s.provider.Name() }

// ModelName is exposed for the status endpoint.
func (s *Service) ModelName() string { return s.provider.Model() }

// AskInput is what the handler passes to the service.
type AskInput struct {
    LessonID uuid.UUID
    Question string
}

// AskResult is what the handler returns to the client.
type AskResult struct {
    Answer    string     `json:"answer"`
    Citations []Citation `json:"citations"`
    Provider  string     `json:"provider"`
    Model     string     `json:"model,omitempty"`
}

// Ask retrieves the lesson title (Part D's only retrieval source, matching
// the noop provider), builds a context, and calls the provider.
func (s *Service) Ask(ctx context.Context, tx pgx.Tx, in AskInput) (*AskResult, error) {
    var title string
    err := tx.QueryRow(ctx, `SELECT title FROM lessons WHERE id = $1`, in.LessonID).Scan(&title)
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, fmt.Errorf("assist: lesson not found")
    }
    if err != nil {
        return nil, fmt.Errorf("assist: retrieve lesson: %w", err)
    }

    chunks := []ContextChunk{{
        Source: "lesson:" + in.LessonID.String(),
        Title:  title,
        Text:   title,
    }}

    resp, err := s.provider.Answer(ctx, AnswerRequest{
        Question: in.Question,
        LessonID: in.LessonID,
        Context:  chunks,
    })
    if err != nil {
        return nil, fmt.Errorf("assist: provider: %w", err)
    }

    return &AskResult{
        Answer:    resp.Answer,
        Citations: resp.Citations,
        Provider:  resp.Provider,
        Model:     resp.Model,
    }, nil
}