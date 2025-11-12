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

		// Visualizer area
		b.DivClass("winamp-info-bar", "style", `
			margin-bottom: 10px;
		`).R(
			// Mini visualizer (decorative)
			b.DivClass("winamp-viz", "style", `
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

		// Control buttons and time display
		b.DivClass("winamp-controls", "style", `
			display: flex;
			gap: 6px;
			margin-bottom: 10px;
			justify-content: space-between;
			align-items: center;
		`).R(
			// Button group (left side)
			b.DivClass("winamp-btn-group", "style", `
				display: flex;
				gap: 6px;
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

			// Time display (right side)
			b.DivClass("winamp-time-display", "style", `
				display: flex;
				align-items: center;
				gap: 6px;
				background: #000;
				border: 1px solid #333;
				border-radius: 2px;
				padding: 6px 10px;
				box-shadow: inset 0 2px 4px rgba(0,0,0,0.8);
			`).R(
				// Current time
				b.SpanClass("winamp-current-time", "id", "sermon-time", "style", `
					color: #00ffff;
					font-size: 15px;
					font-weight: bold;
					text-shadow: 0 0 6px #00ffff;
					letter-spacing: 1px;
					font-family: 'Courier New', monospace;
				`).T("00:00"),

				// Separator
				b.SpanClass("time-separator", "style", `
					color: #666;
					font-size: 12px;
					padding: 0 2px;
				`).T("/"),

				// Total time
				b.SpanClass("winamp-total-time", "id", "sermon-total-time", "style", `
					color: #00ff00;
					font-size: 13px;
					font-weight: bold;
					text-shadow: 0 0 6px #00ff00;
					letter-spacing: 1px;
					font-family: 'Courier New', monospace;
				`).T("00:00"),
			),
		),

		// Time display (right side)
		b.DivClass("winamp-time-display", "style", `
				display: flex;
				align-items: center;
				gap: 6px;
				background: #000;
				border: 1px solid #333;
				border-radius: 2px;
				padding: 6px 10px;
				box-shadow: inset 0 2px 4px rgba(0,0,0,0.8);
			`).R(
			// Current time
			b.SpanClass("winamp-current-time", "id", "sermon-time", "style", `
					color: #00ffff;
					font-size: 15px;
					font-weight: bold;
					text-shadow: 0 0 6px #00ffff;
					letter-spacing: 1px;
					font-family: 'Courier New', monospace;
				`).T("00:00"),

			// Separator
			b.SpanClass("time-separator", "style", `
					color: #666;
					font-size: 12px;
					padding: 0 2px;
				`).T("/"),

			// Total time
			b.SpanClass("winamp-total-time", "id", "sermon-total-time", "style", `
					color: #00ff00;
					font-size: 13px;
					font-weight: bold;
					text-shadow: 0 0 6px #00ff00;
					letter-spacing: 1px;
					font-family: 'Courier New', monospace;
				`).T("00:00"),
		),

		// Progress bar with volume control
		b.DivClass("winamp-progress-area", "style", `
			display: flex;
			gap: 8px;
			margin-bottom: 10px;
			align-items: center;
		`).R(
			// Progress bar
			b.DivClass("winamp-progress-container", "style", `
				background: #000;
				border: 1px solid #333;
				border-radius: 2px;
				padding: 4px;
				cursor: pointer;
				box-shadow: inset 0 2px 4px rgba(0,0,0,0.8);
				position: relative;
				flex: 1;
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

			// Volume control container
			b.DivClass("winamp-volume-container", "style", `
				position: relative;
			`).R(
				// Volume button
				b.Button("class", "winamp-volume-btn", "id", "sermon-volume-btn", "title", "Volume", "style", `
					background: linear-gradient(180deg, #4a5158 0%, #2c3137 100%);
					border: 1px solid #000;
					border-radius: 3px;
					color: #fff;
					cursor: pointer;
					padding: 6px 10px;
					font-size: 16px;
					box-shadow: inset 0 1px 0 rgba(255,255,255,0.2), 0 2px 4px rgba(0,0,0,0.3);
					min-width: 40px;
				`, "onclick", "toggleSermonVolumePopup()").R(
					b.SpanClass("volume-icon", "id", "sermon-volume-icon").T("🔊"),
				),

				// Volume popup (vertical slider)
				b.DivClass("winamp-volume-popup", "id", "sermon-volume-popup", "style", `
					display: none;
					position: absolute;
					bottom: 45px;
					right: 0;
					background: linear-gradient(180deg, #2c3137 0%, #1a1d21 100%);
					border: 2px solid #000;
					border-radius: 4px;
					padding: 12px 8px;
					box-shadow: inset 0 1px 0 rgba(255,255,255,0.1), 0 4px 12px rgba(0,0,0,0.7);
					z-index: 1001;
				`).R(
					// Volume percentage display
					b.DivClass("volume-display", "style", `
						text-align: center;
						margin-bottom: 8px;
					`).R(
						b.SpanClass("volume-value", "id", "sermon-volume-display", "style", `
							color: #00ff00;
							font-size: 14px;
							font-weight: bold;
							text-shadow: 0 0 6px #00ff00;
							font-family: 'Courier New', monospace;
						`).T("80"),
						b.SpanClass("volume-percent", "style", `
							color: #00ff00;
							font-size: 11px;
							text-shadow: 0 0 6px #00ff00;
							font-family: 'Courier New', monospace;
						`).T("%"),
					),

					// Vertical slider container
					b.DivClass("vertical-slider-container", "style", `
						height: 120px;
						width: 40px;
						background: #000;
						border: 1px solid #333;
						border-radius: 2px;
						position: relative;
						cursor: pointer;
						box-shadow: inset 0 2px 4px rgba(0,0,0,0.8);
					`, "onclick", "setSermonVolumeFromClick(event)").R(
						// Volume bar (fills from bottom)
						b.DivClass("volume-bar", "id", "sermon-volume-bar", "style", `
							position: absolute;
							bottom: 0;
							left: 0;
							right: 0;
							height: 80%;
							background: linear-gradient(to top, #00ff00, #00cc00);
							border-radius: 1px;
							box-shadow: 0 0 8px rgba(0,255,0,0.6);
							transition: height 0.1s ease;
						`).R(),

						// Slider thumb
						b.DivClass("volume-thumb", "id", "sermon-volume-thumb", "style", `
							position: absolute;
							left: 50%;
							transform: translateX(-50%);
							bottom: 80%;
							width: 36px;
							height: 6px;
							background: linear-gradient(180deg, #fff 0%, #ccc 100%);
							border: 1px solid #000;
							border-radius: 2px;
							box-shadow: 0 2px 4px rgba(0,0,0,0.5);
							cursor: grab;
							transition: bottom 0.1s ease;
						`, "onmousedown", "startSermonVolumeDrag(event)").R(),
					),
				),
			),
		),

		// Download link
		b.DivClass("winamp-download", "style", `
			margin-top: 10px;
			padding-top: 8px;
			border-top: 1px solid #333;
			text-align: center;`).R(
			b.A("href", ser.AudioLink, "download", "download", "title", "Download sermon",
				"style", `
					color: #00ffff;
					text-decoration: none;
					font-size: 12px;
					text-shadow: 0 0 4px #00ffff;`).T("💾 DOWNLOAD"),
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
			const totalTimeDisplay = document.getElementById('sermon-total-time');
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

			// Set total time when metadata is loaded
			audio.addEventListener('loadedmetadata', function() {
				totalTimeDisplay.textContent = formatTime(audio.duration);
			});

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

			// Volume control elements
			const volumePopup = document.getElementById('sermon-volume-popup');
			const volumeBar = document.getElementById('sermon-volume-bar');
			const volumeThumb = document.getElementById('sermon-volume-thumb');
			const volumeIcon = document.getElementById('sermon-volume-icon');
			let isDraggingVolume = false;

			// Update volume display and icon
			function updateVolumeUI(value) {
				volumeDisplay.textContent = value;
				volumeBar.style.height = value + '%';
				volumeThumb.style.bottom = value + '%';

				// Update icon based on volume level
				if (value == 0) {
					volumeIcon.textContent = '🔇';
				} else if (value < 50) {
					volumeIcon.textContent = '🔉';
				} else {
					volumeIcon.textContent = '🔊';
				}
			}

			// Set volume from value (0-100)
			window.setSermonVolume = function(value) {
				value = Math.max(0, Math.min(100, value));
				audio.volume = value / 100;
				updateVolumeUI(value);
			};

			// Toggle volume popup
			window.toggleSermonVolumePopup = function() {
				if (volumePopup.style.display === 'none' || volumePopup.style.display === '') {
					volumePopup.style.display = 'block';
				} else {
					volumePopup.style.display = 'none';
				}
			};

			// Close volume popup when clicking outside
			document.addEventListener('click', function(event) {
				const volumeContainer = document.querySelector('.winamp-volume-container');
				if (volumeContainer && !volumeContainer.contains(event.target)) {
					volumePopup.style.display = 'none';
				}
			});

			// Set volume from click on slider
			window.setSermonVolumeFromClick = function(event) {
				const container = event.currentTarget;
				const rect = container.getBoundingClientRect();
				const y = rect.bottom - event.clientY;
				const percentage = Math.round((y / rect.height) * 100);
				setSermonVolume(percentage);
			};

			// Start dragging volume thumb
			window.startSermonVolumeDrag = function(event) {
				event.preventDefault();
				event.stopPropagation();
				isDraggingVolume = true;
				document.body.style.cursor = 'grabbing';
				volumeThumb.style.cursor = 'grabbing';
			};

			// Handle volume drag
			document.addEventListener('mousemove', function(event) {
				if (!isDraggingVolume) return;

				const container = document.querySelector('.vertical-slider-container');
				const rect = container.getBoundingClientRect();
				const y = rect.bottom - event.clientY;
				const percentage = Math.round((y / rect.height) * 100);
				setSermonVolume(percentage);
			});

			// Stop dragging
			document.addEventListener('mouseup', function() {
				if (isDraggingVolume) {
					isDraggingVolume = false;
					document.body.style.cursor = '';
					volumeThumb.style.cursor = 'grab';
				}
			});

			// Initialize volume UI
			updateVolumeUI(80);

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
