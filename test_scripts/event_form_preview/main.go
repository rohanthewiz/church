// Renders the revamped admin event form module to a standalone HTML file so
// the UI can be eyeballed in a browser without a running server/DB.
// Run from the church root (needs cfg/random_seeds.txt for the auth init):
//
//	go run ./test_scripts/event_form_preview
//
// Summernote/jQuery come from CDNs here purely for the preview; the real
// admin layout serves its own copies.
package main

import (
	"fmt"
	"os"

	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/event"
)

func main() {
	pres := module.Presenter{}
	pres.Name.Singular = "event"
	pres.Name.Plural = "events"

	mod, err := event.NewModuleEventForm(pres)
	if err != nil {
		fmt.Println("Error creating module:", err)
		os.Exit(1)
	}

	formHTML := mod.Render(nil, true)

	page := `<!DOCTYPE html>
<html><head><title>Event Form Preview</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/summernote/0.8.18/summernote.min.css">
<script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/2.1.4/jquery.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/summernote/0.8.18/summernote.min.js"></script>
<style>body { font-family: -apple-system, "Segoe UI", Roboto, sans-serif; background: #eef1ec; margin: 0; padding: 1rem; }</style>
</head><body>` + formHTML + `</body></html>`

	outPath := "test_scripts/event_form_preview/preview.html"
	if err := os.WriteFile(outPath, []byte(page), 0644); err != nil {
		fmt.Println("Error writing preview:", err)
		os.Exit(1)
	}
	fmt.Println("Wrote", outPath)
}
