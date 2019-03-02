//+build ignore

package main

import (
	"bufio"
	"fmt"
	"github.com/rohanthewiz/church/util/fileops"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var initialTmplBytes = []byte(initialTemplate)
const initialTemplate = `
// Code generated by go generate; DO NOT EDIT.
package packed
`
const goExt = ".go"
const packSrc = "pack/src"
const packedDest = "pack/packed"
const packerStart = "PACKER START"
const packerEnd = "PACKER END"

// Synopsis
// For each file of allowed extension in pack/src
// Read file with bufio.Scanner
// Look for a line starting with // PACKER START
// Get the var name from the above line
// Read all other lines till // PACKER END
// Append to the initial template:
//	var <NAME> = `<Other lines read>` (escape backticks in Other lines)
// create the file with the name of the source but with the ext '.go'
func main() {
	fmt.Println("Packing assets...")
	_ = packFiles()
}

func packFiles() (err error) {
	filesInfo, err := ioutil.ReadDir(packSrc)
	if err != nil {
		logger.LogErr(err, "Error reading directory " + packSrc)
		return err
	}
	for _, fileInfo := range filesInfo {
		if !fileInfo.IsDir() { // Todo ! - and file is of allowed extension
			err = packFile(fileInfo.Name())
			if err != nil {
				logger.LogErr(err)
				// continue
			}
		}
	}
	return
}

func packFile(filename string) (err error) {
	fmt.Println("Packing", filename, "...")

	content, err := extractContentFromFile(filename)
	if err != nil { return serr.Wrap(err, "Unable to extract content from file") }

	basename := fileops.FilenameWithoutExt(filename)

	file, err := os.Create(filepath.Join(packedDest, basename + goExt))
	if err != nil { return serr.Wrap(err) }
	defer file.Close()

	file.Write(initialTmplBytes)
	file.Write([]byte(content))

	return
}

func extractContentFromFile(filename string) (content string, err error) {
	file, err := os.Open(filepath.Join(packSrc, filename))
	if err != nil { return content, serr.Wrap(err) }

	scnr := bufio.NewScanner(file)
	varname := ""
	contentLines := []string{}

	for scnr.Scan() { // default split on line break
		line := scnr.Text()
		if varname == "" {
			if i := strings.Index(line, packerStart); i != -1 {
				varname = strings.TrimSpace(line[i+len(packerStart)+1:])
				continue
			}
		} else if strings.Contains(line, packerEnd) {
			break
		} else {
			contentLines = append(contentLines, line)
		}
	}

	if err := scnr.Err(); err != nil {
		return content, serr.Wrap(err, "Error while scanning", "filename", filename)
	}
	if varname == "" {
		return content, serr.Wrap(err, "Could not obtain a valid variable name",
			"filename", filename)
	}
	content = fmt.Sprintf("\nvar %s = `%s`\n", varname, strings.Join(contentLines, "\n"))
	return
}
