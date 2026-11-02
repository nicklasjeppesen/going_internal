package customrouter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/customrouter"
	"github.com/nicklasjeppesen/going_internal/super/request"
)

type queryPayload struct {
	Query string `json:"query" form:"query"`
	Limit int    `json:"limit" form:"limit"`
}

type queryController struct{}

// Search receives the QUERY request body through RequestBodybase, just like a
// POST/PUT/PATCH action would, and echoes the decoded payload back.
func (queryController) Search(body request.RequestBodybase[*queryPayload]) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body.PrintJson()
	}
}

func TestQueryRouteBindsRequestBody(t *testing.T) {
	router := customrouter.NewMyRouter()
	router.Query("/search", queryController{}, "Search")

	mux := http.NewServeMux()
	router.RegisterRoutes(mux)

	cases := []struct {
		name        string
		contentType string
		body        string
	}{
		{"json", "application/json", `{"query":"go router","limit":10}`},
		{"form", "application/x-www-form-urlencoded", "query=go+router&limit=10"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("QUERY", "/search", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			rec := httptest.NewRecorder()

			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("QUERY %s panicked: %v", tc.name, p)
				}
			}()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
			}

			var got queryPayload
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("invalid response %q: %v", rec.Body.String(), err)
			}
			if got.Query != "go router" || got.Limit != 10 {
				t.Fatalf("controller got %+v, want query=%q limit=10", got, "go router")
			}
		})
	}
}
