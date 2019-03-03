package fileops

import "strings"

func FilenameWithoutExt(filename string) (basename string) {
	arr := strings.Split(filename, ".")
	if len(arr) < 2 { return filename }
	return strings.Join(arr[:len(arr) - 1], ".")
}
