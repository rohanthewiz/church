package sermon

import (
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/church/util/timeutil"
	"github.com/rohanthewiz/rweb"
	"github.com/rohanthewiz/serr"
	"github.com/vattle/sqlboiler/queries/qm"
)

func APISermonsRWeb(ctx rweb.Context) error {
	// TODO - Query params
	limit := ctx.Request().QueryParam("limit")
	//endDate = ctx.Request().QueryParam("end")

	lmt, err := strconv.Atoi(limit)
	if err != nil {
		lmt = 50
		//return serr.Wrap(err)
	}

	var sermons []SermonsResp

	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	condition := "1 = 1"
	sms, err := models.Sermons(dbH, qm.Where(condition), qm.OrderBy("date_taught DESC"), qm.Limit(lmt)).All()
	if err != nil {
		return serr.Wrap(err, "Error obtaining sermons")
	}

	for _, ser := range sms {
		s := SermonsResp{
			Title:         ser.Title,
			DateTaught:    ser.DateTaught.Format(timeutil.ISO8601DateTime),
			ScriptureRefs: strings.Join(ser.ScriptureRefs, ","),
			AudioLink:     ser.AudioLink.String,
		}
		sermons = append(sermons, s)
	}

	return ctx.WriteJSON(&sermons)
}