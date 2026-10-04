package request

import (
	"context"
	"io"
	"net/http"
)

// limitedBody caps how much of a request body can be read. The cap is applied
// on the first Read, so a route can still raise it (SetBodyLimit) before the
// body is read — e.g. for file uploads.
type limitedBody struct {
	src    io.ReadCloser
	w      http.ResponseWriter
	limit  int64
	reader io.ReadCloser
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.reader == nil {
		b.reader = http.MaxBytesReader(b.w, b.src, b.limit)
	}
	return b.reader.Read(p)
}

func (b *limitedBody) Close() error { return b.src.Close() }

type bodyLimitKey struct{}

// LimitBody caps the request body at limit bytes; reading more fails with
// "http: request body too large". Returns the request to pass on.
func LimitBody(w http.ResponseWriter, r *http.Request, limit int64) *http.Request {
	if r.Body == nil || r.Body == http.NoBody {
		return r
	}
	body := &limitedBody{src: r.Body, w: w, limit: limit}
	r.Body = body
	return r.WithContext(context.WithValue(r.Context(), bodyLimitKey{}, body))
}

// SetBodyLimit changes the cap set by LimitBody for this request. It has no
// effect (and returns false) once reading of the body has started.
func SetBodyLimit(r *http.Request, limit int64) bool {
	body, ok := r.Context().Value(bodyLimitKey{}).(*limitedBody)
	if !ok || body.reader != nil {
		return false
	}
	body.limit = limit
	return true
}
