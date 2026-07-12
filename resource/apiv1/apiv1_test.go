package apiv1

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rohanthewiz/rweb"
)

// ParseLimitOffset guards the API from bulk-sync abuse; the cap and the
// garbage-tolerant defaults are contract, not convenience.
func TestParseLimitOffset(t *testing.T) {
	s := rweb.NewServer(rweb.ServerOptions{})
	s.Get("/t", func(ctx rweb.Context) error {
		limit, offset := ParseLimitOffset(ctx, 20, 100)
		return ctx.WriteJSON(map[string]int{"limit": limit, "offset": offset})
	})

	cases := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{"defaults", "", 20, 0},
		{"explicit", "?limit=5&offset=3", 5, 3},
		{"capped", "?limit=999", 100, 0},
		{"negatives ignored", "?limit=-2&offset=-9", 20, 0},
		{"garbage ignored", "?limit=abc&offset=xyz", 20, 0},
		{"zero limit ignored", "?limit=0", 20, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := s.Request("GET", "/t"+tc.query, nil, nil)
			var got struct{ Limit, Offset int }
			if err := json.Unmarshal(resp.Body(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Limit != tc.wantLimit || got.Offset != tc.wantOffset {
				t.Errorf("got limit=%d offset=%d, want %d/%d",
					got.Limit, got.Offset, tc.wantLimit, tc.wantOffset)
			}
		})
	}
}

func TestErrorShape(t *testing.T) {
	s := rweb.NewServer(rweb.ServerOptions{})
	s.Get("/t", func(ctx rweb.Context) error {
		return Error(ctx, 404, "Thing not found")
	})

	resp := s.Request("GET", "/t", nil, nil)
	if resp.Status() != 404 {
		t.Errorf("status = %d, want 404", resp.Status())
	}
	if got := string(resp.Body()); got != `{"error":"Thing not found"}` {
		t.Errorf("body = %s", got)
	}
}

// ServerError must (a) answer with the uniform JSON shape — never rweb's HTML
// error page, which the mobile client can't parse — and (b) keep internal
// error details out of the body; they belong in server logs only.
func TestServerErrorIsJSONAndHidesInternals(t *testing.T) {
	s := rweb.NewServer(rweb.ServerOptions{})
	s.Get("/t", func(ctx rweb.Context) error {
		return ServerError(ctx, errors.New("pq: connection refused"), "Could not load things")
	})

	resp := s.Request("GET", "/t", nil, nil)
	if resp.Status() != 500 {
		t.Errorf("status = %d, want 500", resp.Status())
	}
	body := string(resp.Body())
	if body != `{"error":"Could not load things"}` {
		t.Errorf("body = %s", body)
	}
	if strings.Contains(body, "connection refused") {
		t.Errorf("internal error text leaked to client: %s", body)
	}
}
