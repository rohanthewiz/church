package sermon

import (
	"os"
	"github.com/rohanthewiz/logger"
	"fmt"
	"regexp"
	"strings"
	"strconv"
	"bufio"
	//"github.com/rohanthewiz/church/util/stringops"
)

func CCSWMSermonImport() (byts []byte) {
	csvFile, err := os.Open("jos_content.csv")
	if err != nil {
		println("Error on import"); return []byte(`{"success": false}`)
	}
	scanner := bufio.NewScanner(csvFile)

	var count int

	for scanner.Scan() { // splits on lines by default
		row := strings.Split(scanner.Text(), "|")
		fmt.Printf("Row[0]: %#v\n", row[0])
		if len(row) < 3 { println("short row (", strings.Join(row, "||"), ")"); continue }

		// Prep and fixup
		re := regexp.MustCompile("^[[:space:]]*([[:digit:]].+?([[:digit:]]) )?(.*)")
		fmt.Printf("%s\n", re.FindStringSubmatch(row[0]))

//		pres := Presenter{}
//		pres.Title = row[0]
//		pres.Summary = row[1]
//		pres.ScriptureRefs = stringops.StringSplitAndTrim(row[2], ",")
//		pres.Teacher =  row[4]
//		pres.DateTaught = strings.SplitN(row[5], " ", 2)[0]
//		pres.PlaceTaught = row[6]
//		pres.AudioLink =  transformAudioLink(row[7])
//		pres.Categories = stringops.StringSplitAndTrim(row[8], ",")
//		pres.UpdatedBy = "Importer"
//		pres.CreateSlug()
//
		count++
		if count == 1 { continue }  // skip heading
//
		if count == 8 { break }  // todo - limit for dev
//
//		fmt.Printf("Presenter: %#v\n", pres)
//		//if _, err = pres.Upsert(); err != nil {
//		//	return []byte(`{"success": false}`)
//		//}
	}
	if err := scanner.Err(); err != nil {
		logger.LogErr(err, "Error while scanning sermons csv file")
	}

	return []byte(`{"success": true, "count": ` + strconv.Itoa(count - 1) + `}`)
}

//func transformAudioLink(iLink string) (oLink string) {
//	arr := strings.Split(iLink, "/")
//	if ln := len(arr); ln > 2 {
//		oLink = "http://mediasave.org/" + path.Join("cema", strings.Join(arr[2:], "/"))
//	}
//	return
//}

//func cleanList(field string) (cleanedItems []string) {
//	arr := stringops.StringSplitAndTrim(field[1:len(field)-1], ",")  // drop `[` and `]'
//	for _, item := range arr {
//		if ln := len(item); ln > 5 {
//			cleanedItems = append(cleanedItems, item[2:ln-1])  //remove `u'` and `'`
//		}
//	}
//	if len(cleanedItems) == 0 {
//		cleanedItems = []string{" "} // todo not liking - categories need to be nullable
//	}
//	return
//}