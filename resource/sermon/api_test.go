package sermon

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rohanthewiz/church/models"
	"gopkg.in/nullbio/null.v6"
)

// The mobile app iterates scripture_refs/categories without null checks, so
// the DTO must serialize empty arrays as [], never null.
func TestSermonToAPIEmptyArraysSerializeAsArrays(t *testing.T) {
	ser := &models.Sermon{
		ID:         42,
		Title:      "On Grace",
		Teacher:    "Pastor A",
		DateTaught: time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC),
	}

	byts, err := json.Marshal(sermonToAPI(ser, false))
	if err != nil {
		t.Fatal(err)
	}
	s := string(byts)

	for _, want := range []string{`"scripture_refs":[]`, `"categories":[]`, `"id":42`} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in %s", want, s)
		}
	}
	// body is list-omitted (omitempty) to keep list payloads lean
	if strings.Contains(s, `"body"`) {
		t.Errorf("body should be omitted from list DTOs, got %s", s)
	}
}

func TestSermonToAPIDetailIncludesBody(t *testing.T) {
	ser := &models.Sermon{
		ID:            7,
		Title:         "With Body",
		DateTaught:    time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
		Body:          null.StringFrom("<p>notes</p>"),
		AudioLink:     null.StringFrom("/sermon-audio/2026/msg.mp3"),
		ScriptureRefs: []string{"John 3:16", "Rom 8:1"},
	}

	dto := sermonToAPI(ser, true)
	if dto.Body != "<p>notes</p>" {
		t.Errorf("detail DTO should carry body, got %q", dto.Body)
	}
	if dto.AudioURL != "/sermon-audio/2026/msg.mp3" {
		t.Errorf("audio_url should pass through as stored, got %q", dto.AudioURL)
	}
	if len(dto.ScriptureRefs) != 2 || dto.ScriptureRefs[0] != "John 3:16" {
		t.Errorf("scripture_refs should remain an array, got %v", dto.ScriptureRefs)
	}
	if dto.DateTaught != "2026-01-04T00:00:00" {
		t.Errorf("date_taught should be ISO8601, got %q", dto.DateTaught)
	}
}
