package chimage

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/rohanthewiz/logger"
	"github.com/vincent-petithory/dataurl"
	"bytes"
	"github.com/h2non/bimg"
	"github.com/rohanthewiz/serr"
	"github.com/rohanthewiz/church/util/stringops"
	"strings"
	"strconv"
	"path/filepath"
	"io/ioutil"
)

const localImagesFolder = "dist/img/"  // separate from our core images like our banner images etc.
const webImagesFolder = "/assets/img/"
const resizeThreshold = 3000

// Mainly Resize
func ProcessInlineImages(field string) (out string, err error){
	//fmt.Println("|* ", field)
	buf := bytes.NewBuffer([]byte(field))
	doc, err := goquery.NewDocumentFromReader(buf)
	if err != nil {
		return "", serr.Wrap(err, "Error creating goquery document from given field")
	}
	doc.Find("img").Each(func(i int, sel *goquery.Selection) {
		strDataUrl, exists := sel.Attr("src")
		if !exists {
			logger.LogAsync("error", "could not obtain 'src' attribute of image")
			return
		}
		dUrl, err := dataurl.DecodeString(strDataUrl)
		if err != nil {
			logger.LogAsync("warn", "could not find a dataurl in img src", "error", err.Error())
			return
		}
		lenOrig := len(dUrl.Data)
		fmt.Println("|* image size:", lenOrig)
		if lenOrig > resizeThreshold {
			height, ok := getHeightFromStyle(sel)
			if !ok { height = 400 }
			logger.LogAsync("Info", "Resizing", "height", strconv.Itoa(height))
			resized, err := bimg.Resize(dUrl.Data, bimg.Options{Height: height})
			if err != nil {
				logger.LogErrAsync(err, "Could not resize image")
				return
			}
			lnResized := len(resized)
			fmt.Println("|* Resized image size:", lnResized)
			if lnResized < lenOrig { // seems like pngs are not recompressed after sizing
				dUrl.Data = resized
			}
		}
		if lenOrig < 5000 { // inline smaller images
			dUrlOut, err := dUrl.MarshalText()
			if err != nil {
				logger.LogErrAsync(err, "Error marshalling data url")
				return
			}
			sel.SetAttr("src", string(dUrlOut))
		} else {
			filename, ext, ok := getFilename(sel)
			if !ok { return }
			uniqueFilename := filename + "." + stringops.XXHash(string(dUrl.Data)) + ext
			//uniqueFilename := stringops.SlugWithRandomString(filename) + ext
			ioutil.WriteFile(localImagesFolder + uniqueFilename, dUrl.Data, 0644)
			sel.SetAttr("src", webImagesFolder + uniqueFilename)
		}
	})
	out, err = doc.Html()
	if err != nil {
		logger.LogErrAsync(err)
	}
	return
}

func getHeightFromStyle(sel *goquery.Selection) (height int, ok bool) {
	style, exists := sel.Attr("style")
	if !exists {
		logger.LogAsync("warn", "could not obtain 'style' attribute of image")
		return
	}
	arr := strings.Split(style, ";")
	for _, property := range arr {
		if strings.Contains(property, "height") {
			attVal := strings.Split(property, ":")
			if len(attVal) == 2 {
				hght, err := strconv.Atoi(stringops.NumericString(attVal[1]))
				if err == nil {
					return int(hght + hght * 2 / 10), true
				}
				logger.LogErrAsync(err, "Error parsing height", "string_value", attVal[1])
			}
		}
	}
	return
}

func getFilename(sel *goquery.Selection) (filename, ext string, ok bool) {
	exists := false
	filename, exists = sel.Attr("data-filename")
	if !exists {
		logger.LogAsync("warn", "could not obtain 'data-filename' attribute of image")
		return
	}
	ext = filepath.Ext(filename)
	return filename, ext, true
}

// Interesting img attributes:
// 1. data-filename="rohan.png"
// 2. style="width: 103.825px; height: 103.825px; float: left;"