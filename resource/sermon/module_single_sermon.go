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

	// Check if audio link exists
	if ser.AudioLink == "" {
		b.DivClass("audio-error", "style", `
			color: maroon;
			background: #fff5f5;
			border: 1px solid maroon;
			border-radius: 4px;
			padding: 12px 15px;
			margin: 20px 0;
			font-family: Arial, sans-serif;
		`).T("Audio file not found for this sermon.")
	} else {
		// Add error message container for runtime errors
		b.DivClass("audio-error", "id", "sermon-audio-error", "style", `
			display: none;
			color: maroon;
			background: #fff5f5;
			border: 1px solid maroon;
			border-radius: 4px;
			padding: 12px 15px;
			margin: 20px 0;
			font-family: Arial, sans-serif;
		`).R()
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
						b.SpanClass("volume-icon", "id", "sermon-volume-icon").T(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" style="width: 18px; height: 18px; vertical-align: middle;"><path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/></svg>`),
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

			// Track if error has been shown to avoid duplicate messages
			let errorShown = false;

			// Format time helper function
			function formatTime(seconds) {
				const mins = Math.floor(seconds / 60);
				const secs = Math.floor(seconds % 60);
				return mins.toString().padStart(2, '0') + ':' + secs.toString().padStart(2, '0');
			}

			// Function to display error message with path (without domain)
			function showAudioError() {
				if (errorShown) return; // Prevent duplicate error messages
				errorShown = true;

				const errorDiv = document.getElementById('sermon-audio-error');
				const playerDiv = document.querySelector('.sermon-audio-player');
				const audioSrc = audio.querySelector('source').src;

				// Extract path from URL (everything after domain and port)
				let filePath = audioSrc;
				try {
					const url = new URL(audioSrc);
					filePath = url.pathname + url.search + url.hash;
				} catch (e) {
					// If URL parsing fails, use the original src
					filePath = audioSrc;
				}

				errorDiv.textContent = 'Audio file "' + filePath + '" is not found.';
				errorDiv.style.display = 'block';
				if (playerDiv) {
					playerDiv.style.display = 'none';
				}
			}

			// Set total time when metadata is loaded
			audio.addEventListener('loadedmetadata', function() {
				totalTimeDisplay.textContent = formatTime(audio.duration);
				// Check if duration is zero or invalid (file not found or can't be loaded)
				if (!audio.duration || audio.duration === 0 || isNaN(audio.duration)) {
					showAudioError();
				}
			});

			// Handle audio loading errors (catches 404, 500, network errors, etc.)
			audio.addEventListener('error', function(e) {
				console.log('Audio error event:', e);
				showAudioError();
			});

			// Handle source errors specifically
			const audioSource = audio.querySelector('source');
			if (audioSource) {
				audioSource.addEventListener('error', function(e) {
					console.log('Audio source error:', e);
					showAudioError();
				});
			}

			// Set a timeout to check if the file loaded after a reasonable time
			setTimeout(function() {
				// If audio hasn't loaded metadata and no error was shown yet, it likely failed
				if (audio.readyState === 0 && !errorShown) {
					console.log('Audio load timeout - readyState:', audio.readyState);
					showAudioError();
				}
			}, 5000); // 5 second timeout

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

				// Update icon based on volume level (SVG speaker icons)
				if (value == 0) {
					// Muted speaker
					volumeIcon.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" style="width: 18px; height: 18px; vertical-align: middle;"><path d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z"/></svg>';
				} else if (value < 50) {
					// Low volume speaker
					volumeIcon.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" style="width: 18px; height: 18px; vertical-align: middle;"><path d="M7 9v6h4l5 5V4l-5 5H7z"/></svg>';
				} else {
					// High volume speaker
					volumeIcon.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" style="width: 18px; height: 18px; vertical-align: middle;"><path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/></svg>';
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
	}

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
