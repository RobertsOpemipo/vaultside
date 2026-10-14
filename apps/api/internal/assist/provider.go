// Package assist implements the AI guide: a provider interface, a noop
// default, and a service that retrieves tenant-scoped content, builds a
// prompt, and returns an answer with citations.
package assist

import (
    "context"
    "fmt"

    "github.com/google/uuid"
)

// Provider is the contract a real model must satisfy. The runtime picks one
// by name from config (ASSIST_PROVIDER). Only "noop" ships today.
type Provider interface {
    Name() string
    Model() string
    Answer(ctx context.Context, req AnswerRequest) (AnswerResponse, error)
}

// AnswerRequest is what the service hands the provider.
type AnswerRequest struct {
    Question string
    LessonID uuid.UUID
    Context  []ContextChunk
}

// ContextChunk is a retrieved piece of tenant content the provider may cite.
type ContextChunk struct {
    Source string `json:"source"`           // e.g. "lesson:uuid", "transcript:uuid"
    Title  string `json:"title"`
    Text   string `json:"text"`
    Offset *int   `json:"offset,omitempty"` // seconds into a video, if applicable
}

// AnswerResponse is what the provider returns to the service.
type AnswerResponse struct {
    Answer    string     `json:"answer"`
    Citations []Citation `json:"citations"`
    Provider  string     `json:"provider"`
    Model     string     `json:"model,omitempty"`
}

// Citation ties part of an answer back to a source the learner can open.
type Citation struct {
    Source string `json:"source"`
    Title  string `json:"title"`
    Quote  string `json:"quote,omitempty"`
    Offset *int   `json:"offset,omitempty"`
}

// New picks a provider by name. Unknown names fall back to noop.
func New(name string) (Provider, error) {
    switch name {
    case "", "noop":
        return &Noop{}, nil
    default:
        return nil, fmt.Errorf("assist: unknown provider %q", name)
    }
}

// Noop is the default. It returns a well-formed stub answer that cites the
// first retrieved chunk, so the endpoint, the citation plumbing and the audit
// log all work end to end without an external API key.
type Noop struct{}

func (n *Noop) Name() string  { return "noop" }
func (n *Noop) Model() string { return "noop" }

func (n *Noop) Answer(_ context.Context, req AnswerRequest) (AnswerResponse, error) {
    cites := make([]Citation, 0, 1)
    if len(req.Context) > 0 {
        c := req.Context[0]
        cites = append(cites, Citation{
            Source: c.Source,
            Title:  c.Title,
            Quote:  c.Text,
            Offset: c.Offset,
        })
    }
    return AnswerResponse{
        Answer: fmt.Sprintf(
            "[noop assist] The AI provider is not configured. "+
                "Your question was: %q. This is a stub answer, returned with "+
                "the citation a real provider would use.",
            req.Question,
        ),
        Citations: cites,
        Provider:  "noop",
        Model:     "noop",
    }, nil
}