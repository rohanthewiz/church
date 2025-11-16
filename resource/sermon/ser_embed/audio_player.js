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
    const loadingIndicator = document.getElementById('sermon-loading-indicator');

    // Set initial volume
    audio.volume = 0.9;

    // Track if error has been shown to avoid duplicate messages
    let errorShown = false;

    // Format time helper function
    function formatTime(seconds) {
        const mins = Math.floor(seconds / 60);
        const secs = Math.floor(seconds % 60);
        return mins.toString().padStart(2, '0') + ':' + secs.toString().padStart(2, '0');
    }

    // Function to display error message with path (without domain)
    function showAudioError(msg = 'could not be loaded.') {
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

        errorDiv.textContent = 'Audio file "' + filePath + '" ' + msg;
        errorDiv.style.display = 'block';
        if (playerDiv) {
            playerDiv.style.display = 'none';
        }
    }

    // Set total time when metadata is loaded
    audio.addEventListener('loadedmetadata', function() {
        setTimeout(function() {
            totalTimeDisplay.textContent = formatTime(audio.duration);
            // Check if duration is zero or invalid (file not found or can't be loaded)
            if (!audio.duration || audio.duration === 0 || isNaN(audio.duration)) {
                showAudioError('has an invalid duration');
                if (loadingIndicator) loadingIndicator.style.display = 'none';
            } else {
                // Audio loaded successfully, hide loading indicator
                if (loadingIndicator) loadingIndicator.style.display = 'none';
            }
        }, 2000); // give it a couple seconds to settle
    });

    // Handle audio loading errors (catches 404, 500, network errors, etc.)
    audio.addEventListener('error', function(e) {
        console.log('Audio error event:', e);
        showAudioError('had an error ' + e.error.code + ': ' + e.error.message);
        if (loadingIndicator) loadingIndicator.style.display = 'none';
    });

    // Handle source errors specifically
    const audioSource = audio.querySelector('source');
    if (audioSource) {
        audioSource.addEventListener('error', function(e) {
            console.log('Audio source error:', e);
            showAudioError('had an error loading the audio source: ' + e.target.src);
            if (loadingIndicator) loadingIndicator.style.display = 'none';
        });
    }

    // Set a timeout to check if the file loaded after a reasonable time
    setTimeout(function() {
        // If audio hasn't loaded metadata and no error was shown yet, it likely failed
        if (audio.readyState === 0 && !errorShown) {
            console.log('Audio load timeout - readyState:', audio.readyState);
            showAudioError('did not load within a reasonable time.');
            if (loadingIndicator) loadingIndicator.style.display = 'none';
        }
    }, 30000); // wait a few seconds

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
    updateVolumeUI(90);

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