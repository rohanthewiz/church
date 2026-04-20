package sermon

import (
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
)

// APISermonsRWeb is the rweb-flavored mirror of APISermons. Kept in lockstep
// with the echo handler — any change to the response shape must be made in
// both.
func APISermonsRWeb(ctx rweb.Context) error {
	limit := ctx.Request().QueryParam("limit")
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

	return ctx.WriteJSON(&sermons)
}
