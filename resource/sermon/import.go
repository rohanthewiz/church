package sermon

//import (
//	"os"
//	"github.com/rohanthewiz/logger"
//	"fmt"
//	"strings"
//	"path"
//	"strconv"
//	"bufio"
//	"github.com/rohanthewiz/church/util/stringops"
//)
//
// Old import - we now can import directly from database
//func Import() (byts []byte) {
//	csvFile, err := os.Open("sermons.csv")
//	if err != nil {
//		println("Error on import"); return []byte(`{"success": false}`)
//	}
//	scanner := bufio.NewScanner(csvFile)
//
//	var count int
//
//	for scanner.Scan() { // splits on lines by default
//		line := scanner.Text()
//		row := strings.Split(line, "|")
//		fmt.Printf("Row: %#v\n", row)
//		if len(row) < 9 { println("short row (", strings.Join(row, "||"), ")"); continue }
//
//		// Prep and fixup
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
//		count++
//		if count == 1 { continue }  // skip heading
//
//		if count == 30 { break }  // todo - limit for dev
//
//		fmt.Printf("Presenter: %#v\n", pres)
//		//if _, err = pres.Upsert(); err != nil {
//		//	return []byte(`{"success": false}`)
//		//}
//	}
//	if err := scanner.Err(); err != nil {
//		logger.LogErr(err, "Error while scanning sermons csv file")
//	}
//
//
//	return []byte(`{"success": true, "count": ` + strconv.Itoa(count - 1) + `}`)
//}
//
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