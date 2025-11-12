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

	// Audio player styled to match page
	b.DivClass("sermon-audio-player", "style", `
		margin: 20px 0;
		width: 100%;
		max-width: 500px;
		background: #ffffff;
		border: 1px solid #d0d0d0;
		border-radius: 6px;
		padding: 15px;
		box-shadow: 0 2px 8px rgba(0,0,0,0.1);
		font-family: Arial, sans-serif;
	`).R(
		// Title display
		b.DivClass("audio-title", "style", `
			color: #333;
			padding: 10px 12px;
			margin-bottom: 15px;
			font-size: 15px;
			font-weight: 600;
			overflow: hidden;
			white-space: nowrap;
			text-overflow: ellipsis;
			background: #f8f9fa;
			border-left: 4px solid #0066a1;
			border-radius: 3px;
		`).T(ser.Title),

		// HTML5 audio element (hidden)
		b.Audio("id", "sermon-audio", "style", "display: none;").R(
			b.Source("src", ser.AudioLink, "type", "audio/mpeg").R(),
		),

		// Control buttons and time display
		b.DivClass("audio-controls", "style", `
			display: flex;
			gap: 10px;
			margin-bottom: 12px;
			justify-content: space-between;
			align-items: center;
		`).R(
			// Left side: Button group + Download
			b.DivClass("audio-controls-left", "style", `
				display: flex;
				gap: 10px;
				align-items: center;
			`).R(
				// Button group
				b.DivClass("audio-btn-group", "style", `
					display: flex;
					gap: 6px;
				`).R(
					// Previous button (go to start) with tooltip
					b.DivClass("btn-tooltip-container", "style", `position: relative;`).R(
						b.Button("class", "audio-btn", "style", `
							background: #e9ecef;
							border: 1px solid #ced4da;
							border-radius: 4px;
							color: #495057;
							cursor: pointer;
							padding: 10px 14px;
							font-size: 15px;
							transition: all 0.2s;
							min-width: 44px;
							display: flex;
							align-items: center;
							justify-content: center;
						`, "onclick", "goToSermonStart()",
							"onmouseover", "this.style.background='#dee2e6'; this.nextElementSibling.style.display='block'",
							"onmouseout", "this.style.background='#e9ecef'; this.nextElementSibling.style.display='none'").T("⏮"),
						b.DivClass("btn-tooltip", "style", `
							display: none;
							position: absolute;
							bottom: 45px;
							left: 50%;
							transform: translateX(-50%);
							background: #333;
							color: #fff;
							padding: 6px 10px;
							border-radius: 4px;
							font-size: 12px;
							white-space: nowrap;
							z-index: 1000;
							pointer-events: none;
						`).T("Go to start"),
					),

					// Play/Pause button with tooltip
					b.DivClass("btn-tooltip-container", "style", `position: relative;`).R(
						b.Button("class", "audio-btn", "id", "sermon-play-btn", "style", `
							background: #0066a1;
							border: 1px solid #005a8d;
							border-radius: 4px;
							color: #fff;
							cursor: pointer;
							padding: 10px 14px;
							font-size: 15px;
							font-weight: bold;
							transition: all 0.2s;
							min-width: 44px;
							display: flex;
							align-items: center;
							justify-content: center;
						`, "onclick", "toggleSermonPlayback()",
							"onmouseover", "this.style.background='#005a8d'; this.nextElementSibling.style.display='block'",
							"onmouseout", "this.style.background='#0066a1'; this.nextElementSibling.style.display='none'").T("▶"),
						b.DivClass("btn-tooltip", "style", `
							display: none;
							position: absolute;
							bottom: 45px;
							left: 50%;
							transform: translateX(-50%);
							background: #333;
							color: #fff;
							padding: 6px 10px;
							border-radius: 4px;
							font-size: 12px;
							white-space: nowrap;
							z-index: 1000;
							pointer-events: none;
						`).T("Play/Pause"),
					),

					// Stop button with tooltip
					b.DivClass("btn-tooltip-container", "style", `position: relative;`).R(
						b.Button("class", "audio-btn", "id", "sermon-stop-btn", "style", `
							background: #dc3545;
							border: 1px solid #c82333;
							border-radius: 4px;
							color: #fff;
							cursor: pointer;
							padding: 10px 14px;
							font-size: 15px;
							transition: all 0.2s;
							min-width: 44px;
							display: flex;
							align-items: center;
							justify-content: center;
						`, "onclick", "stopSermonPlayback()",
							"onmouseover", "this.style.background='#c82333'; this.nextElementSibling.style.display='block'",
							"onmouseout", "this.style.background='#dc3545'; this.nextElementSibling.style.display='none'").T("■"),
						b.DivClass("btn-tooltip", "style", `
							display: none;
							position: absolute;
							bottom: 45px;
							left: 50%;
							transform: translateX(-50%);
							background: #333;
							color: #fff;
							padding: 6px 10px;
							border-radius: 4px;
							font-size: 12px;
							white-space: nowrap;
							z-index: 1000;
							pointer-events: none;
						`).T("Stop"),
					),

					// Next button (go to near end) with tooltip
					b.DivClass("btn-tooltip-container", "style", `position: relative;`).R(
						b.Button("class", "audio-btn", "style", `
							background: #e9ecef;
							border: 1px solid #ced4da;
							border-radius: 4px;
							color: #495057;
							cursor: pointer;
							padding: 10px 14px;
							font-size: 15px;
							transition: all 0.2s;
							min-width: 44px;
							display: flex;
							align-items: center;
							justify-content: center;
						`, "onclick", "goToSermonEnd()",
							"onmouseover", "this.style.background='#dee2e6'; this.nextElementSibling.style.display='block'",
							"onmouseout", "this.style.background='#e9ecef'; this.nextElementSibling.style.display='none'").T("⏭"),
						b.DivClass("btn-tooltip", "style", `
							display: none;
							position: absolute;
							bottom: 45px;
							left: 50%;
							transform: translateX(-50%);
							background: #333;
							color: #fff;
							padding: 6px 10px;
							border-radius: 4px;
							font-size: 12px;
							white-space: nowrap;
							z-index: 1000;
							pointer-events: none;
						`).T("Go to end"),
					),
				),

				// Download button with tooltip
				b.DivClass("audio-download-container", "style", `
					position: relative;
				`).R(
					b.A("href", ser.AudioLink, "download", "download", "class", "audio-download-btn",
						"style", `
							display: flex;
							align-items: center;
							justify-content: center;
							background: #e9ecef;
							border: 1px solid #ced4da;
							border-radius: 4px;
							color: #495057;
							cursor: pointer;
							padding: 10px 14px;
							font-size: 15px;
							transition: all 0.2s;
							text-decoration: none;
							min-width: 44px;
						`,
						"onmouseover", "this.style.background='#dee2e6'; document.getElementById('download-tooltip').style.display='block'",
						"onmouseout", "this.style.background='#e9ecef'; document.getElementById('download-tooltip').style.display='none'").T("💾"),
					// Tooltip
					b.DivClass("download-tooltip", "id", "download-tooltip", "style", `
						display: none;
						position: absolute;
						bottom: 45px;
						left: 50%;
						transform: translateX(-50%);
						background: #333;
						color: #fff;
						padding: 6px 10px;
						border-radius: 4px;
						font-size: 12px;
						white-space: nowrap;
						z-index: 1000;
						pointer-events: none;
					`).T("Download"),
				),
			),

			// Time display (right side) - LED style
			b.DivClass("audio-time-display", "style", `
				display: flex;
				align-items: center;
				gap: 8px;
				background: #1a1a1a;
				border: 1px solid #333;
				border-radius: 4px;
				padding: 10px 16px;
				box-shadow: inset 0 2px 4px rgba(0,0,0,0.5);
				height: 44px;
				box-sizing: border-box;
			`).R(
				// Current time - LED style
				b.SpanClass("audio-current-time", "id", "sermon-time", "style", `
					color: #00e5ff;
					font-size: 18px;
					font-weight: bold;
					text-shadow: 0 0 8px rgba(0,229,255,0.8);
					letter-spacing: 1.5px;
					font-family: 'Courier New', Monaco, monospace;
					line-height: 1;
				`).T("00:00"),

				// Separator
				b.SpanClass("time-separator", "style", `
					color: #555;
					font-size: 16px;
					padding: 0 2px;
					font-family: 'Courier New', Monaco, monospace;
					line-height: 1;
				`).T("/"),

				// Total time - LED style
				b.SpanClass("audio-total-time", "id", "sermon-total-time", "style", `
					color: #ffffff;
					font-size: 16px;
					font-weight: bold;
					text-shadow: 0 0 8px rgba(255,255,255,0.6);
					letter-spacing: 1.5px;
					font-family: 'Courier New', Monaco, monospace;
					line-height: 1;
				`).T("00:00"),
			),
		),

		// Progress bar with volume control
		b.DivClass("audio-progress-area", "style", `
			display: flex;
			gap: 8px;
			margin-bottom: 12px;
			align-items: center;
		`).R(
			// Progress bar
			b.DivClass("audio-progress-container", "style", `
				background: #e9ecef;
				border: 1px solid #ced4da;
				border-radius: 4px;
				padding: 3px;
				cursor: pointer;
				position: relative;
				flex: 1;
			`, "onclick", "seekSermonAudio(event)", "onmousemove", "showSermonTimeTooltip(event)", "onmouseleave", "hideSermonTimeTooltip()").R(
				b.DivClass("audio-progress-bar", "id", "sermon-progress", "style", `
					background: linear-gradient(90deg, #0066a1 0%, #004d7a 100%);
					height: 8px;
					width: 0%;
					border-radius: 3px;
					transition: width 0.1s linear;
				`).R(),
				// Time tooltip - LED style
				b.DivClass("audio-time-tooltip", "id", "sermon-time-tooltip", "style", `
					display: none;
					position: absolute;
					bottom: 24px;
					background: #1a1a1a;
					color: #00e5ff;
					padding: 4px 8px;
					border: 1px solid #333;
					border-radius: 3px;
					font-size: 11px;
					font-weight: bold;
					text-shadow: 0 0 8px rgba(0,229,255,0.8);
					pointer-events: none;
					white-space: nowrap;
					z-index: 1000;
					box-shadow: 0 2px 8px rgba(0,0,0,0.3);
					font-family: 'Courier New', Monaco, monospace;
				`).T("00:00"),
			),

			// Volume control container
			b.DivClass("audio-volume-container", "style", `
				position: relative;
			`).R(
				// Volume button
				b.Button("class", "audio-volume-btn", "id", "sermon-volume-btn", "title", "Volume", "style", `
					background: #e9ecef;
					border: 1px solid #ced4da;
					border-radius: 4px;
					color: #495057;
					cursor: pointer;
					padding: 8px 12px;
					font-size: 16px;
					min-width: 44px;
					transition: all 0.2s;
				`, "onclick", "toggleSermonVolumePopup()", "onmouseover", "this.style.background='#dee2e6'", "onmouseout", "this.style.background='#e9ecef'").R(
					b.SpanClass("volume-icon", "id", "sermon-volume-icon").T("🔊"),
				),

				// Volume popup (vertical slider)
				b.DivClass("audio-volume-popup", "id", "sermon-volume-popup", "style", `
					display: none;
					position: absolute;
					bottom: 50px;
					right: 0;
					background: #ffffff;
					border: 1px solid #ced4da;
					border-radius: 6px;
					padding: 12px 10px;
					box-shadow: 0 4px 12px rgba(0,0,0,0.15);
					z-index: 1001;
				`).R(
					// Volume percentage display
					b.DivClass("volume-display", "style", `
						text-align: center;
						margin-bottom: 10px;
					`).R(
						b.SpanClass("volume-value", "id", "sermon-volume-display", "style", `
							color: #0066a1;
							font-size: 16px;
							font-weight: bold;
							font-family: Arial, sans-serif;
						`).T("80"),
						b.SpanClass("volume-percent", "style", `
							color: #0066a1;
							font-size: 12px;
							font-family: Arial, sans-serif;
						`).T("%"),
					),

					// Vertical slider container
					b.DivClass("vertical-slider-container", "style", `
						height: 120px;
						width: 44px;
						background: #e9ecef;
						border: 1px solid #ced4da;
						border-radius: 4px;
						position: relative;
						cursor: pointer;
					`, "onclick", "setSermonVolumeFromClick(event)").R(
						// Volume bar (fills from bottom)
						b.DivClass("volume-bar", "id", "sermon-volume-bar", "style", `
							position: absolute;
							bottom: 0;
							left: 0;
							right: 0;
							height: 80%;
							background: linear-gradient(to top, #0066a1 0%, #005a8d 100%);
							border-radius: 3px;
							transition: height 0.1s ease;
						`).R(),

						// Slider thumb
						b.DivClass("volume-thumb", "id", "sermon-volume-thumb", "style", `
							position: absolute;
							left: 50%;
							transform: translateX(-50%);
							bottom: 80%;
							width: 40px;
							height: 8px;
							background: #ffffff;
							border: 2px solid #0066a1;
							border-radius: 4px;
							box-shadow: 0 2px 4px rgba(0,0,0,0.2);
							cursor: grab;
							transition: bottom 0.1s ease;
						`, "onmousedown", "startSermonVolumeDrag(event)").R(),
					),
				),
			),
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

			// Go to start of audio
			window.goToSermonStart = function() {
				audio.currentTime = 0;
			};

			// Go to near end of audio (5 seconds before end to avoid auto-ending)
			window.goToSermonEnd = function() {
				if (audio.duration) {
					audio.currentTime = Math.max(0, audio.duration - 5);
				}
			};

			window.seekSermonAudio = function(event) {
				const container = event.currentTarget;
				const rect = container.getBoundingClientRect();
				const x = event.clientX - rect.left;
				const percentage = x / rect.width;
				audio.currentTime = percentage * audio.duration;

				// Start playing immediately after seeking
				if (audio.paused) {
					audio.play();
					playBtn.textContent = '⏸';
				}
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
				const volumeContainer = document.querySelector('.audio-volume-container');
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
