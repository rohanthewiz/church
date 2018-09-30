package sermon

import (
	"github.com/rohanthewiz/logger"
	"strings"
	"path"
	"strconv"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"fmt"
	"database/sql"
	"github.com/rohanthewiz/church/util/stringops"
)

const sqlGetSermons = `select name, summary, array_to_string(scripture_refs, ','), "text", teacher, date_taught, place_taught,
    audio_link, array_to_string(categories, ',')
from sermons`

type importReceptor struct {
	Name, Summary, ScriptureRefs, Body, Teacher, DateTaught, PlaceTaught, AudioLink, Categories string
}

func (i *importReceptor) Scan(rs *sql.Rows) error {
	return rs.Scan(&i.Name, &i.Summary, &i.ScriptureRefs, &i.Body, &i.Teacher, &i.DateTaught, &i.PlaceTaught,
			&i.AudioLink, &i.Categories)
}

func Import() (byts []byte) {
	fail := []byte(`{"success": false}`)
	fmt.Printf("%#v", config.Options.PG2)
	err := db.InitDB2(db.DBOpts{
		DBType: db.DBTypes.Postgres,
		Host: config.Options.PG2.Host,
		Port: config.Options.PG2.Port,
		User: config.Options.PG2.User,
		Word: config.Options.PG2.Word,
		Database: config.Options.PG2.Database,
	})
	if err != nil {
		logger.LogErr(err, "Could not setup database")
		return fail
	}
	defer db.CloseDB2()

	var count int

	dbH, err := db.Db2()
	if err != nil {
		logger.LogErr(err, "Could not obtain a handle on the second database")
		return fail
	}

	rs, err := dbH.Query(sqlGetSermons)
	if err != nil {
		logger.LogErr(err, "Error executing query for sermons for import")
		return fail
	}
	defer rs.Close()

	//var rows []importReceptor

	for rs.Next() {
		ir := importReceptor{}
		err = ir.Scan(rs)
		if err != nil {
			logger.LogErr(err, "Error scanning rs on sermons import")
			break
		}
		if len(ir.Summary) > 300 {
			logger.LogAsync("Info", "Truncating long summary", "sermon title", ir.Name)
			ir.Summary = ""
		}
		if len(ir.Body) > 300 {
			logger.LogAsync("Info", "Truncating long Body", "sermon title", ir.Name)
			ir.Body = ""
		}
		//rows = append(rows, ir)
		count++

		pres := Presenter{}
		pres.Title = ir.Name
		pres.Summary = ir.Summary
		pres.ScriptureRefs = stringops.StringSplitAndTrim(ir.ScriptureRefs, ",")
		if len(pres.ScriptureRefs) < 1 { pres.ScriptureRefs = []string{""} }
		pres.Teacher =  ir.Teacher
		pres.DateTaught = strings.SplitN(ir.DateTaught, "T", 2)[0]
		pres.PlaceTaught = ir.PlaceTaught
		pres.AudioLink =  transformAudioLink(ir.AudioLink)
		pres.Categories = stringops.StringSplitAndTrim(ir.Categories, ",")
		if len(pres.Categories) < 1 { pres.Categories = []string{""} }
		pres.UpdatedBy = "Importer"
		pres.Published = true
		pres.CreateSlug()
		if _, err = pres.Upsert(); err != nil {
			return []byte(`{"success": false}`)
		}

	}

	//fmt.Printf("Rows (%d): %v\n", len(rows), rows)
	if err = rs.Err(); err != nil {
		logger.LogErr(err, "Some rs error occurred")
		return fail
	}

	return []byte(`{"success": true, "count": ` + strconv.Itoa(count) + `}`)
}

func transformAudioLink(iLink string) (oLink string) {
	arr := strings.Split(iLink, "/")
	if ln := len(arr); ln > 2 {
		oLink = "http://mediasave.org/" + path.Join("cema", strings.Join(arr[2:], "/"))
	}
	return
}
