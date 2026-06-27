package sermoncleanup

// Package sermoncleanup provides the admin "Sermon Cleanup" UI module: it lists
// locally-cached sermon files that already have a verified copy on IDrive e2 and
// lets an admin batch-delete the local copies to reclaim disk.
//
// It lives in its own package (rather than resource/sermon) because it depends on
// core/idrive for the scan/delete service, and core/idrive already imports
// resource/sermon — putting this module in resource/sermon would create a cycle.

import (
	"fmt"
	"strconv"
	"time"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/core/idrive"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const ModuleTypeSermonCleanup = "sermon_cleanup"

// CleanupActionPath is the admin route the cleanup form posts to. Kept here so the
// route registration and the form action stay in sync.
const CleanupActionPath = "/admin/sermons/cleanup"

type ModuleSermonCleanup struct {
	module.Presenter
	csrf string
}

// NewModuleSermonCleanup builds the module and mints a CSRF token for its form.
func NewModuleSermonCleanup(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleSermonCleanup)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token.")
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

func (m *ModuleSermonCleanup) Render(params map[string]map[string]string, loggedIn bool) string {
	// Scan the local sermons dir and check each file against IDrive e2. This does
	// network I/O (bounded concurrency inside the service) but only on this
	// admin-only page, so the cost is acceptable.
	eligible, err := idrive.ScanEligibleForDeletion()
	if err != nil {
		logger.LogErr(err, "sermon-cleanup: failed to scan eligible sermons")
		b := element.NewBuilder()
		b.DivClass("ch-module-wrapper ch-" + ModuleTypeSermonCleanup).R(
			b.DivClass("ch-module-heading").T(m.Opts.Title),
			b.PClass("error").T("Could not scan local sermons: "+err.Error()),
		)
		return b.String()
	}

	// Group eligible files by year, preserving the service's sort order
	// (year newest-first, then file name).
	var years []string
	groups := map[string][]idrive.LocalSermonInfo{}
	var totalBytes int64
	for _, s := range eligible {
		if _, ok := groups[s.Year]; !ok {
			years = append(years, s.Year)
		}
		groups[s.Year] = append(groups[s.Year], s)
		totalBytes += s.CloudSize
	}

	b := element.NewBuilder()

	b.DivClass("ch-module-wrapper ch-"+ModuleTypeSermonCleanup).R(
		b.DivClass("ch-module-heading").T(m.Opts.Title),

		b.Style().T(cleanupCSS),

		b.Wrap(func() {
			if len(eligible) == 0 {
				b.PClass("sc-empty").T("No local sermons are currently eligible for deletion. " +
					"A sermon becomes eligible once a non-zero copy with the same name exists on IDrive e2 under its year.")
				return
			}

			b.Form("method", "post", "action", CleanupActionPath, "onsubmit", "return scPrepare();").R(
				b.Input("type", "hidden", "name", "csrf", "value", m.csrf),
				// JS fills this with the newline-joined selected keys at submit time,
				// because rweb's FormValue only exposes a single value per field.
				b.Input("type", "hidden", "name", "selected_specs", "id", "selected_specs"),

				// Toolbar: summary + global select-all + submit.
				b.DivClass("sc-toolbar").R(
					b.LabelClass("sc-selectall").R(
						b.Input("type", "checkbox", "id", "sc-master", "onclick", "scToggleAll(this);"),
						b.T(" Select all"),
					),
					b.SpanClass("sc-summary").R(
						b.T(strconv.Itoa(len(eligible))+" eligible across "+strconv.Itoa(len(years))+" year(s) · "),
						b.T(formatBytes(totalBytes)+" reclaimable · "),
						b.T("Selected: "),
						b.Span("id", "sc-count").T("0"),
					),
					b.Button("type", "submit", "id", "sc-submit", "class", "sc-delete-btn", "disabled", "disabled").
						T("Delete selected local copies"),
				),

				// One section + table per year.
				b.Wrap(func() {
					element.ForEach(years, func(year string) {
						rows := groups[year]
						b.DivClass("sc-year-group").R(
							b.H3Class("sc-year-heading").R(
								b.LabelClass("sc-year-select").R(
									b.Input("type", "checkbox", "class", "sc-year-master",
										"data-year", year, "onclick", "scToggleYear(this);"),
									b.T(" "+year),
								),
								b.SpanClass("sc-year-count").T(" ("+strconv.Itoa(len(rows))+")"),
							),
							b.TableClass("sc-table").R(
								b.THead().R(
									b.Tr().R(
										b.Th().T(""),
										b.Th().T("File"),
										b.Th().T("IDrive e2 path"),
										b.Th().T("Size"),
										b.Th().T("Last accessed"),
									),
								),
								b.TBody().R(
									element.ForEach(rows, func(s idrive.LocalSermonInfo) {
										b.Tr().R(
											b.TdClass("sc-cb-cell").R(
												b.Input("type", "checkbox", "class", "sc-cb",
													"data-year", s.Year, "value", s.RelFileSpec,
													"onclick", "scUpdateCount();"),
											),
											b.TdClass("sc-file").T(s.FileName),
											b.TdClass("sc-path").T(s.CloudPath),
											b.TdClass("sc-size").T(formatBytes(s.CloudSize)),
											b.TdClass("sc-accessed").T(formatLastAccessed(s.LastAccessed)),
										)
									}),
								),
							),
						)
					})
				}),
			)
			b.Script().T(cleanupJS)
		}),
	)

	return b.String()
}

// formatBytes renders a byte count in human-friendly units.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// formatLastAccessed shows the absolute time plus a coarse relative age, or a dash
// when the file was never tracked (e.g. uploaded but never served).
func formatLastAccessed(t *time.Time) string {
	if t == nil {
		return "—"
	}
	age := time.Since(*t)
	return t.Local().Format("2006-01-02 15:04") + " (" + humanizeAge(age) + ")"
}

func humanizeAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h ago"
	default:
		return strconv.Itoa(int(d.Hours()/24)) + "d ago"
	}
}
