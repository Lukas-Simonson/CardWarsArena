package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFeedbackLinksAreOffUnlessAsked(t *testing.T) {
	for _, on := range []bool{false, true} {
		s := &Server{cfg: Config{Feedback: on}}
		rec := httptest.NewRecorder()
		s.handleFeedback(rec, httptest.NewRequest(http.MethodGet, "/api/feedback", nil))
		var got struct {
			Enabled bool   `json:"enabled"`
			Links   []Link `json:"links"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Enabled != on || (len(got.Links) > 0) != on {
			t.Errorf("feedback=%v answered %+v", on, got)
		}
		for _, l := range got.Links {
			if !strings.HasPrefix(l.URL, "https://") || l.Label == "" {
				t.Errorf("bad link %+v", l)
			}
		}
	}
}
