package httpx

import "net/http"

type statusWriter struct {
    http.ResponseWriter
    status int
    wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
    if w.wrote {
        return
    }
    w.status = code
    w.wrote = true
    w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
    if !w.wrote {
        w.status = http.StatusOK
        w.wrote = true
    }
    return w.ResponseWriter.Write(b)
}