package sermon

import (
	"errors"
	"fmt"
	strconv "strconv"
	"strings"

	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/element"
	. "github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const ModuleTypeSingleSermon = "sermon_single"

type ModuleSingleSermon struct {
	module.Presenter
}

func NewModuleSingleSermon(pres module.Presenter) (module.Module, error) {
	mod := new(ModuleSingleSermon)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	return module.Module(mod), nil
}

func (m ModuleSingleSermon) getData() (pres Presenter, err error) {
	// If the module instance has an item slug defined, it takes highest precedence
	if m.Opts.ItemSlug != "" {
		pres, err = PresenterFromSlug(m.Opts.ItemSlug)
	} else {
		if len(m.Opts.ItemIds) < 1 {
			return pres, serr.Wrap(errors.New("No item ids found"),
				"module_options", fmt.Sprintf("%#v", m.Opts))
		}
		pres, err = presenterFromId(m.Opts.ItemIds[0])
	}
	return
}

func (m *ModuleSingleSermon) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetId(opts)
	}
	ser, err := m.getData()
	if err != nil {
		LogErr(err, "Error in module render")
		return ""
	}

	b := element.NewBuilder()

	b.H3Class("sermon-title").T(ser.Title)
	b.SpanClass("sermon-sub-title").R(
		b.T(ser.Teacher + " - " + ser.DateTaught),
	)

	// WinAmp-style audio player
	b.DivClass("winamp-player", "style", `
		margin: 20px 0;
		width: 100%;
		max-width: 440px;
		background: linear-gradient(180deg, #2c3137 0%, #1a1d21 100%);
		border: 2px solid #000;
		border-radius: 4px;
		padding: 8px;
		box-shadow: inset 0 1px 0 rgba(255,255,255,0.1), 0 4px 8px rgba(0,0,0,0.5);
		font-family: 'Courier New', monospace;
	`).R(
		// Display area (title marquee)
		b.DivClass("winamp-display", "style", `
			background: #000;
			color: #00ff00;
			padding: 8px 12px;
			margin-bottom: 10px;
			border: 1px solid #333;
			border-radius: 2px;
			font-size: 14px;
			font-weight: bold;
			text-shadow: 0 0 8px #00ff00;
			letter-spacing: 1px;
			overflow: hidden;
			white-space: nowrap;
			box-shadow: inset 0 2px 4px rgba(0,0,0,0.8);
		`).T(ser.Title),

		// Time display and visualizer area
		b.DivClass("winamp-info-bar", "style", `
			display: flex;
			justify-content: space-between;
			margin-bottom: 10px;
			gap: 8px;
		`).R(
			// Time display
			b.DivClass("winamp-time", "id", "sermon-time", "style", `
				background: #000;
				color: #00ffff;
				padding: 6px 10px;
				border: 1px solid #333;
				border-radius: 2px;
				font-size: 16px;
				font-weight: bold;
				text-shadow: 0 0 6px #00ffff;
				letter-spacing: 2px;
				min-width: 70px;
				text-align: center;
				box-shadow: inset 0 2px 4px rgba(0,0,0,0.8);
			`).T("00:00"),

			// Mini visualizer (decorative)
			b.DivClass("winamp-viz", "style", `
				flex: 1;
				background: #000;
				border: 1px solid #333;
				border-radius: 2px;
				height: 32px;
				position: relative;
				overflow: hidden;
				box-shadow: inset 0 2px 4px rgba(0,0,0,0.8);
			`).R(
				b.DivClass("viz-bars", "style", `
					display: flex;
					align-items: flex-end;
					height: 100%;
					gap: 2px;
					padding: 4px;
				`).R(
					func() any {
						// Create 12 visualizer bars
						for i := 0; i < 12; i++ {
							height := 20 + (i%3)*10
							b.DivClass("viz-bar", "style", fmt.Sprintf(`
								flex: 1;
								background: linear-gradient(to top, #00ff00, #00cc00);
								height: %d%%;
								border-radius: 1px;
								opacity: 0.7;
							`, height)).R()
						}
						return nil
					}(),
				),
			),
		),

		// HTML5 audio element (hidden)
		b.Audio("id", "sermon-audio", "style", "display: none;").R(
			b.Source("src", ser.AudioLink, "type", "audio/mpeg").R(),
		),

		// Control buttons
		b.DivClass("winamp-controls", "style", `
			display: flex;
			gap: 6px;
			margin-bottom: 10px;
			justify-content: center;
		`).R(
			// Previous button
			b.Button("class", "winamp-btn", "title", "Previous", "style", `
				background: linear-gradient(180deg, #4a5158 0%, #2c3137 100%);
				border: 1px solid #000;
				border-radius: 3px;
				color: #fff;
				cursor: pointer;
				padding: 8px 12px;
				font-size: 14px;
				box-shadow: inset 0 1px 0 rgba(255,255,255,0.2), 0 2px 4px rgba(0,0,0,0.3);
			`, "onclick", "return false;").T("⏮"),

			// Play/Pause button
			b.Button("class", "winamp-btn", "id", "sermon-play-btn", "title", "Play/Pause", "style", `
				background: linear-gradient(180deg, #00cc00 0%, #008800 100%);
				border: 1px solid #000;
				border-radius: 3px;
				color: #fff;
				cursor: pointer;
				padding: 8px 16px;
				font-size: 16px;
				font-weight: bold;
				box-shadow: inset 0 1px 0 rgba(255,255,255,0.3), 0 2px 4px rgba(0,0,0,0.3);
			`, "onclick", "toggleSermonPlayback()").T("▶"),

			// Stop button
			b.Button("class", "winamp-btn", "id", "sermon-stop-btn", "title", "Stop", "style", `
				background: linear-gradient(180deg, #cc0000 0%, #880000 100%);
				border: 1px solid #000;
				border-radius: 3px;
				color: #fff;
				cursor: pointer;
				padding: 8px 12px;
				font-size: 14px;
				box-shadow: inset 0 1px 0 rgba(255,255,255,0.2), 0 2px 4px rgba(0,0,0,0.3);
			`, "onclick", "stopSermonPlayback()").T("■"),

			// Next button
			b.Button("class", "winamp-btn", "title", "Next", "style", `
				background: linear-gradient(180deg, #4a5158 0%, #2c3137 100%);
				border: 1px solid #000;
				border-radius: 3px;
				color: #fff;
				cursor: pointer;
				padding: 8px 12px;
				font-size: 14px;
				box-shadow: inset 0 1px 0 rgba(255,255,255,0.2), 0 2px 4px rgba(0,0,0,0.3);
			`, "onclick", "return false;").T("⏭"),
		),

		// Progress bar
		b.DivClass("winamp-progress-container", "style", `
			background: #000;
			border: 1px solid #333;
			border-radius: 2px;
			padding: 4px;
			margin-bottom: 10px;
			cursor: pointer;
			box-shadow: inset 0 2px 4px rgba(0,0,0,0.8);
			position: relative;
		`, "onclick", "seekSermonAudio(event)", "onmousemove", "showSermonTimeTooltip(event)", "onmouseleave", "hideSermonTimeTooltip()").R(
			b.DivClass("winamp-progress-bar", "id", "sermon-progress", "style", `
				background: linear-gradient(90deg, #00ff00 0%, #00cc00 100%);
				height: 8px;
				width: 0%;
				border-radius: 1px;
				box-shadow: 0 0 8px rgba(0,255,0,0.6);
				transition: width 0.1s linear;
			`).R(),
			// Time tooltip
			b.DivClass("winamp-time-tooltip", "id", "sermon-time-tooltip", "style", `
				display: none;
				position: absolute;
				bottom: 24px;
				background: #000;
				color: #00ffff;
				padding: 4px 8px;
				border: 1px solid #00ffff;
				border-radius: 2px;
				font-size: 11px;
				font-weight: bold;
				text-shadow: 0 0 6px #00ffff;
				pointer-events: none;
				white-space: nowrap;
				z-index: 1000;
				box-shadow: 0 2px 8px rgba(0,255,255,0.5);
			`).T("00:00"),
		),

		// Volume control
		b.DivClass("winamp-volume", "style", `
			display: flex;
			align-items: center;
			gap: 8px;
			padding: 0 8px;
		`).R(
			b.SpanClass("volume-label", "style", `
				color: #aaa;
				font-size: 12px;
			`).T("VOL"),
			b.Input("type", "range", "id", "sermon-volume", "min", "0", "max", "100", "value", "80",
				"style", `
					flex: 1;
					height: 6px;
					-webkit-appearance: none;
					appearance: none;
					background: #333;
					outline: none;
					border-radius: 3px;
				`,
				"oninput", "setSermonVolume(this.value)").R(),
			b.SpanClass("volume-value", "id", "sermon-volume-display", "style", `
				color: #00ff00;
				font-size: 12px;
				min-width: 30px;
				text-align: right;
				text-shadow: 0 0 4px #00ff00;
			`).T("80"),
		),

		// Download link
		b.DivClass("winamp-download", "style", `
			margin-top: 10px;
			padding-top: 8px;
			border-top: 1px solid #333;
			text-align: center;
		`).R(
			b.A("href", ser.AudioLink, "download", "download", "title", "Download sermon",
				"style", `
					color: #00ffff;
					text-decoration: none;
					font-size: 12px;
					text-shadow: 0 0 4px #00ffff;
				`).T("💾 DOWNLOAD"),
		),
	)

	// JavaScript for player controls
	b.Script().T(`
		(function() {
			const audio = document.getElementById('sermon-audio');
			const playBtn = document.getElementById('sermon-play-btn');
			const stopBtn = document.getElementById('sermon-stop-btn');
			const progressBar = document.getElementById('sermon-progress');
			const timeDisplay = document.getElementById('sermon-time');
			const volumeSlider = document.getElementById('sermon-volume');
			const volumeDisplay = document.getElementById('sermon-volume-display');
			const timeTooltip = document.getElementById('sermon-time-tooltip');

			// Set initial volume
			audio.volume = 0.8;

			// Format time helper function
			function formatTime(seconds) {
				const mins = Math.floor(seconds / 60);
				const secs = Math.floor(seconds % 60);
				return mins.toString().padStart(2, '0') + ':' + secs.toString().padStart(2, '0');
			}

			// Update progress and time
			audio.addEventListener('timeupdate', function() {
				const progress = (audio.currentTime / audio.duration) * 100;
				progressBar.style.width = progress + '%';
				timeDisplay.textContent = formatTime(audio.currentTime);
			});

			// Update button when playback ends
			audio.addEventListener('ended', function() {
				playBtn.textContent = '▶';
			});

			// Global functions for onclick handlers
			window.toggleSermonPlayback = function() {
				if (audio.paused) {
					audio.play();
					playBtn.textContent = '⏸';
				} else {
					audio.pause();
					playBtn.textContent = '▶';
				}
			};

			window.stopSermonPlayback = function() {
				audio.pause();
				audio.currentTime = 0;
				playBtn.textContent = '▶';
				progressBar.style.width = '0%';
				timeDisplay.textContent = '00:00';
			};

			window.seekSermonAudio = function(event) {
				const container = event.currentTarget;
				const rect = container.getBoundingClientRect();
				const x = event.clientX - rect.left;
				const percentage = x / rect.width;
				audio.currentTime = percentage * audio.duration;
			};

			window.setSermonVolume = function(value) {
				audio.volume = value / 100;
				volumeDisplay.textContent = value;
			};

			// Show time tooltip on hover
			window.showSermonTimeTooltip = function(event) {
				if (!audio.duration) return; // Don't show if audio not loaded

				const container = event.currentTarget;
				const rect = container.getBoundingClientRect();
				const x = event.clientX - rect.left;
				const percentage = x / rect.width;
				const time = percentage * audio.duration;

				// Update tooltip content and position
				timeTooltip.textContent = formatTime(time);
				timeTooltip.style.display = 'block';

				// Center tooltip on cursor, but keep within bounds
				const tooltipWidth = timeTooltip.offsetWidth;
				let leftPosition = x - (tooltipWidth / 2);

				// Constrain to container bounds
				if (leftPosition < 0) leftPosition = 0;
				if (leftPosition + tooltipWidth > rect.width) {
					leftPosition = rect.width - tooltipWidth;
				}

				timeTooltip.style.left = leftPosition + 'px';
			};

			// Hide time tooltip
			window.hideSermonTimeTooltip = function() {
				timeTooltip.style.display = 'none';
			};
		})();
	`)
	b.Div().T(ser.Summary)
	b.Div().T(ser.Body)
	b.Wrap(func() {
		if loggedIn && len(m.Opts.ItemIds) > 0 {
			b.AClass("edit-link", "href", m.GetEditURL()+
				strconv.FormatInt(m.Opts.ItemIds[0], 10)).R(
				b.ImgClass("edit-icon", "title", "Edit Sermon", "src", "/assets/images/edit_article.svg").R(),
			)
		}
	})
	b.DivClass("sermon-footer").R(
		b.SpanClass("scripture").T(strings.Join(ser.ScriptureRefs, ", ")),
		b.SpanClass("categories").T(strings.Join(ser.Categories, ", ")),
	)

	return b.String()
}
