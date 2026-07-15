package sermon

import (
	"net/http"
	"strconv"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/resource/apiv1"
	"github.com/rohanthewiz/church/resource/bibleref"
	"github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
	"github.com/vattle/sqlboiler/queries/qm"
)

// SermonAPI is the public JSON DTO for a sermon. It is a deliberate subset of
// the model — presenters/models must never be serialized directly.
//
// Contract notes for the mobile app (church_mobile):
//   - scripture_refs is a real array (the app parses each ref for
//     BlueLetterBible deep links), unlike the old comma-joined string.
//   - audio_url is returned exactly as stored: usually site-relative
//     ("/sermon-audio/<year>/<file>"), occasionally absolute for legacy
//     mediasave.org imports. The client resolves relative URLs against its
//     API base — the server can't reliably know its public scheme/host
//     behind proxies, so it doesn't try.
//   - body is only populated on the detail endpoint; list payloads stay lean.
//   - scripture_ref_urls is index-aligned with scripture_refs: entry i is the
//     BlueLetterBible deep link for ref i, "" when the ref doesn't parse
//     (topical notes). Precomputed server-side so link/translation rules live
//     in exactly one place (resource/bibleref).
//   - summary_refs are references found inside summary, with byte offsets
//     (start/end) so the app can splice tappable spans without re-parsing.
//     The offsets index the UTF-8 bytes of summary — a Dart client must
//     slice the utf8-encoded bytes, not the UTF-16 string.
type SermonAPI struct {
	ID               int64             `json:"id"`
	Title            string            `json:"title"`
	Summary          string            `json:"summary"`
	Teacher          string            `json:"teacher"`
	PlaceTaught      string            `json:"place_taught"`
	DateTaught       string            `json:"date_taught"`
	ScriptureRefs    []string          `json:"scripture_refs"`
	ScriptureRefURLs []string          `json:"scripture_ref_urls"`
	SummaryRefs      []bibleref.APIRef `json:"summary_refs"`
	Categories       []string          `json:"categories"`
	AudioURL         string            `json:"audio_url"`
	Body             string            `json:"body,omitempty"`
}

// SermonsResp was the original thin list DTO (title/date/refs-as-string/audio_link).
// Superseded by SermonAPI before any known consumer shipped; kept for reference.
// type SermonsResp struct {
// 	Title         string `json:"title"`
// 	DateTaught    string `json:"date_taught"`
// 	ScriptureRefs string `json:"scripture_refs"`
// 	AudioLink     string `json:"audio_link"`
// }

func sermonToAPI(ser *models.Sermon, includeBody bool) SermonAPI {
	s := SermonAPI{
		ID:            ser.ID,
		Title:         ser.Title,
		Summary:       ser.Summary.String,
		Teacher:       ser.Teacher,
		PlaceTaught:   ser.PlaceTaught.String,
		DateTaught:    ser.DateTaught.Format(timeutil.ISO8601DateTime),
		ScriptureRefs: ser.ScriptureRefs,
		Categories:    ser.Categories,
		AudioURL:      ser.AudioLink.String,
	}
	// Empty arrays serialize as [] rather than null so clients can iterate blindly
	if s.ScriptureRefs == nil {
		s.ScriptureRefs = []string{}
	}
	if s.Categories == nil {
		s.Categories = []string{}
	}
	// Parsed BLB links: one per scripture_refs entry (aligned by index), plus
	// offset-carrying refs found in the summary. Empty translation = NKJV,
	// matching the website's ScriptTagger. make(len) not nil-check: a zero-len
	// make still marshals as [].
	s.ScriptureRefURLs = make([]string, len(s.ScriptureRefs))
	for i, raw := range s.ScriptureRefs {
		s.ScriptureRefURLs[i] = bibleref.FirstURL(raw, "")
	}
	s.SummaryRefs = bibleref.FindAllAPI(s.Summary, "")
	if includeBody {
		s.Body = ser.Body.String
	}
	return s
}

// GET /api/v1/sermons?limit&offset&year&teacher&ref
// Published sermons, newest first.
func APISermonsRWeb(ctx rweb.Context) error {
	limit, offset := apiv1.ParseLimitOffset(ctx, 50, 200)

	// All filters are bound parameters — never concatenate user input into SQL
	// (the calendar endpoint was bitten by exactly that; see commit cb80039).
	mods := []qm.QueryMod{
		qm.Where("published = true"),
		qm.OrderBy("date_taught DESC"),
		qm.Limit(limit),
		qm.Offset(offset),
	}

	if yr := ctx.Request().QueryParam("year"); yr != "" {
		y, err := strconv.Atoi(yr)
		if err != nil {
			return apiv1.Error(ctx, http.StatusBadRequest, "year must be a four digit number")
		}
		mods = append(mods, qm.Where("EXTRACT(YEAR FROM date_taught) = ?", y))
	}
	if teacher := ctx.Request().QueryParam("teacher"); teacher != "" {
		mods = append(mods, qm.Where("teacher ILIKE ?", "%"+teacher+"%"))
	}
	// Substring match over the refs array, e.g. ref=John matches "John 3:16".
	// array_to_string flattens so ILIKE can search; '|' separator avoids false
	// hits across adjacent refs that a comma-space join could produce.
	if ref := ctx.Request().QueryParam("ref"); ref != "" {
		mods = append(mods, qm.Where("array_to_string(scripture_refs, '|') ILIKE ?", "%"+ref+"%"))
	}

	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not load sermons")
	}
	sms, err := models.Sermons(dbH, mods...).All()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not load sermons")
	}

	sermons := make([]SermonAPI, 0, len(sms))
	for _, ser := range sms {
		sermons = append(sermons, sermonToAPI(ser, false))
	}

	// Enveloped so count/paging metadata can be added without breaking clients
	return ctx.WriteJSON(map[string]any{
		"sermons": sermons,
		"limit":   limit,
		"offset":  offset,
	})
}

// GET /api/v1/sermons/:id — single sermon including body.
func APISermonRWeb(ctx rweb.Context) error {
	id, err := strconv.ParseInt(ctx.Request().Param("id"), 10, 64)
	if err != nil {
		return apiv1.Error(ctx, http.StatusBadRequest, "sermon id must be an integer")
	}

	dbH, err := db.Db()
	if err != nil {
		return apiv1.ServerError(ctx, err, "Could not load sermon")
	}
	// Published check in the query itself, so drafts 404 identically to
	// nonexistent ids — no oracle for unpublished content.
	ser, err := models.Sermons(dbH, qm.Where("id = ? AND published = true", id)).One()
	if err != nil {
		logger.LogErr(err, "sermon not found for API", "id", ctx.Request().Param("id"))
		return apiv1.Error(ctx, http.StatusNotFound, "Sermon not found")
	}

	return ctx.WriteJSON(sermonToAPI(ser, true))
}

// RecentSermonsAPI returns the newest published sermons as API DTOs.
// Exported for the /api/v1/feed aggregator.
func RecentSermonsAPI(limit int) ([]SermonAPI, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	sms, err := models.Sermons(dbH, qm.Where("published = true"),
		qm.OrderBy("date_taught DESC"), qm.Limit(limit)).All()
	if err != nil {
		return nil, serr.Wrap(err, "Error obtaining recent sermons")
	}
	out := make([]SermonAPI, 0, len(sms))
	for _, ser := range sms {
		out = append(out, sermonToAPI(ser, false))
	}
	return out, nil
}
