package sermon

import (
	"bufio"
	"fmt"
	"github.com/rohanthewiz/logger"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	//"github.com/rohanthewiz/church/util/stringops"
)

// Returns JSON as []byte
func CCSWMSermonImport() (byts []byte) {
	byts = []byte(`{"success": false}`)
	const audioPrefix = "http://mediasave.org/ccswm/sermons/"

	csvFile, err := os.Open("jos_content.csv")
	if err != nil {
		logger.LogErr(err, "Error opening jos_content.csv")
		return
	}
	scanner := bufio.NewScanner(csvFile)

	var count int

	for scanner.Scan() { // splits on lines by default
		count++
		if count == 1 {
			continue // skip heading
		}
		if count == 8 {
			break // todo !! - remove limit for dev
		}

		row := strings.Split(scanner.Text(), "|")
		//fmt.Printf("Row[0]: %#v\n", row[0]) // date and title

		if len(row) < 2 {
			logger.Log("Error", "short row -> " + strings.Join(row, "||"))
			continue
		}

		// Parse Title and Date Taught
		re0 := regexp.MustCompile("^[[:space:]]*([[:digit:]].+?([[:digit:]]) )?(.*)")
		arr := re0.FindStringSubmatch(row[0])
		fmt.Printf("%q\n", arr) // debug - Todo ! remove
		if len(arr) < 4 {
			logger.Log("Error", fmt.Sprintf("Parse of date and title yielded less than 4 parts -> %q\n", arr))
			continue // probably some other content
		}
		dateTaught, err := parseDateTaught(arr[1])
		if err != nil {
			logger.Log("Error", "Unable to parse date taught -> "+arr[1])
			continue // no good without a date, probably some other content
		}
		title := strings.TrimSpace(arr[3])
		arrTitle := strings.SplitN(title, "-", 2) // Just split off the scripture ref
		mainScripture := ""
		if len(arrTitle) == 2 {
			mainScripture = strings.TrimSpace(arrTitle[0])
			title = strings.TrimSpace(arrTitle[1])
		}

		// Parse Audio
		re1 := regexp.MustCompile("{s5_mp3}(.*){/s5_mp3}")
		arr1 := re1.FindStringSubmatch(row[1])
		if len(arr) < 2 {
			logger.Log("Error", "Could not parse audio link in: " + row[1])
			continue
		}
		audioLink := arr1[1]
		aarr := strings.Split(audioLink, "/")
		if len(aarr) < 3 {
			logger.Log("Error", "Could not find enough tokens in audio link: " + audioLink)
			continue
		}
		audioLinkNew := audioPrefix + strings.Join(aarr[len(aarr)-2:], "/") // year and filename

		// Teacher
		re2 := regexp.MustCompile("Preached by:[[:space:]]*(.+)?<br")
		arr2 := re2.FindStringSubmatch(row[1])
		if len(arr) < 2 {
			logger.Log("Error", "Could not find preacher in" + row[1])
			continue
		}
		teacher := arr2[1]

		pres := Presenter{}
		pres.Title = title
		pres.Summary = ""
		pres.ScriptureRefs = []string{mainScripture}
		pres.Teacher = teacher
		pres.DateTaught = dateTaught.Format("2006-01-02")
		pres.PlaceTaught = "Burleson"
		pres.AudioLink = audioLinkNew
		pres.Categories = []string{""}
		pres.UpdatedBy = "Importer"
		pres.Published = true
		pres.CreateSlug()
		//
		fmt.Printf("Presenter: %#v\n", pres)
		if _, err = pres.Upsert(); err != nil {
			return []byte(`{"success": false}`)
		}
	}
	if err := scanner.Err(); err != nil {
		logger.LogErr(err, "Error while scanning sermons csv file")
	}

	return []byte(`{"success": true, "count": ` + strconv.Itoa(count-1) + `}`)
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

func parseDateTaught(strDate string) (dateTaught time.Time, err error) {
	strDate = strings.TrimSpace(strDate)
	if strings.Contains(strDate, "/") {
		dateTaught, err = time.Parse("1/2/2006", strDate)
		if err != nil {
			dateTaught, err = time.Parse("1/02/2006", strDate)
			if err != nil {
				dateTaught, err = time.Parse("01/02/2006", strDate)
				if err != nil {
					dateTaught, err = time.Parse("1/2/06", strDate)
					if err != nil {
						dateTaught, err = time.Parse("1/02/06", strDate)
						if err != nil {
							dateTaught, err = time.Parse("01/02/06", strDate)
						}
					}
				}
			}
		}
	} else { // assume dashes
		dateTaught, err = time.Parse("1-2-2006", strDate)
		if err != nil {
			dateTaught, err = time.Parse("1-02-2006", strDate)
			if err != nil {
				dateTaught, err = time.Parse("01-02-2006", strDate)
				if err != nil {
					dateTaught, err = time.Parse("1-2-06", strDate)
					if err != nil {
						dateTaught, err = time.Parse("1-02-06", strDate)
						if err != nil {
							dateTaught, err = time.Parse("01-02-06", strDate)
						}
					}
				}
			}
		}
	}

	return
}
