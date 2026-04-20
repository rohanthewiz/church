package sermon

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo"
	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/serr"
)

type SermonsResp struct {
	Title         string `json:"title"`
	DateTaught    string `json:"date_taught"`
	ScriptureRefs string `json:"scripture_refs"`
	AudioLink     string `json:"audio_link"`
}

// APISermons returns up to `limit` (default 50) most-recent sermons as a flat
// JSON array. The `1 = 1` WHERE is a deliberate no-op so the DAO's condition
// fragment path stays consistent with the other endpoints.
func APISermons(c echo.Context) error {
	limit := c.QueryParam("limit")
	lmt, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		lmt = 50
	}

	sms, err := model.QuerySermons("1 = 1", "date_taught DESC", lmt, 0)
	if err != nil {
		return serr.Wrap(err, "Error obtaining sermons")
	}

	sermons := make([]SermonsResp, 0, len(sms))
	for _, ser := range sms {
		sermons = append(sermons, SermonsResp{
			Title:         ser.Title,
			DateTaught:    ser.DateTaught.Format(timeutil.ISO8601DateTime),
			ScriptureRefs: strings.Join([]string(ser.ScriptureRefs), ","),
			AudioLink:     ser.AudioLink.String,
		})
	}

	return c.JSON(http.StatusOK, &sermons)
}
