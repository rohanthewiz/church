package stringops

import (
	"fmt"
	"crypto/sha1"
	"time"
	"github.com/rohanthewiz/church/util/stringops/slugify"
	"strings"
	"github.com/pierrec/xxHash/xxHash64"
)

func StringSliceContains(slice []string, given string) (contained bool) {
	for _, item := range slice {
		if item == given {
			return true
		}
	}
	return
}

func StringSplitAndTrim(instr, separator string) (out []string) {
	arr := strings.Split(instr, separator)
	for _, item := range arr {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return
}

func Sha1(data []byte) string {
	return fmt.Sprintf("%x", sha1.Sum(data))
}

func Sha1WithUnixTime(data []byte) string {
	return fmt.Sprintf("%d.%x", time.Now().UnixNano(), sha1.Sum(data))
}

func Sha1WithUnixTimeShort(data []byte) string {
	str := fmt.Sprintf("%d.%s", time.Now().UnixNano(), string(data))
	return fmt.Sprintf("%x", sha1.Sum([]byte(str)))[4:16]
}

func SlugWithRandomString(title string) string {
	slug := slugify.Marshal(strings.ToLower(title))
	timestr := time.Now().Format("2006-0102")
	return slug + "-" + timestr + "-" + Sha1WithUnixTimeShort([]byte(slug))
}

func Slugify(instr string) string {
	return slugify.Marshal(strings.ToLower(instr))
}

// Return the numeric portion of a string otherwise the whole string
// This is useful for parsing a string with nonnumeric chars eg. " 108px"
func NumericString(astr string) string {
	startPos := -1
	endPos := -1
	for i, c := range astr {
		d := int(c)
		if d < 48 || d > 57 {
			if startPos != -1 {
				endPos = i
				break
			}
			continue
		}
		if startPos == -1 { startPos = i }
		//fmt.Printf("pos: %d, rune: %c, int: %d\n", i, c, c)
	}
	//fmt.Println("startPos:", startPos, "- endPos:", endPos)
	if startPos == -1 || endPos == -1 { return astr }
	return astr[startPos:endPos]
}

func XXHash(str string) string {
	const random_int = 492137458173718
	return fmt.Sprintf("%x",xxHash64.Checksum([]byte(str), random_int))
}