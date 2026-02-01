package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// This is a demo copy of the spinner from cmd/key-seeker/main.go
// Run the actual key-seeker for real testing.

func main() {
	fmt.Println("Spinner Demo (Ctrl+C to stop)")
	fmt.Println("Simulating 10 minute timeout with 5 second poll intervals...")
	fmt.Println()

	const pollTimeout = 10 * time.Minute
	startTime := time.Now()

	// Spinner runs at 100ms for smooth animation
	spinner := newSpinner("Waiting for TOTP response via SMS", pollTimeout)
	spinnerDone := make(chan struct{})

	go func() {
		spinnerTicker := time.NewTicker(100 * time.Millisecond)
		defer spinnerTicker.Stop()
		spinner.Start()
		for {
			select {
			case <-spinnerDone:
				return
			case <-spinnerTicker.C:
				spinner.Tick()
			}
		}
	}()

	// Simulate API poll every 5 seconds - updates elapsed time
	pollTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()

	for {
		select {
		case <-pollTicker.C:
			spinner.SetElapsed(time.Since(startTime))
			// Simulate: API poll would happen here
		}
	}
}

// spinner displays a Heroku-style Braille pattern animation with timer.
type spinner struct {
	message string
	frame   int
	frames  []rune
	isTTY   bool
	elapsed time.Duration
	timeout time.Duration
}

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

func newSpinner(message string, timeout time.Duration) *spinner {
	isTTY := isTTYSupported()
	return &spinner{
		message: message,
		frame:   0,
		frames:  spinnerFrames,
		isTTY:   isTTY,
		elapsed: 0,
		timeout: timeout,
	}
}

func isTTYSupported() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	term := os.Getenv("TERM")
	return term != "dumb"
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func (s *spinner) Start() {
	if s.isTTY {
		s.render()
	}
}

func (s *spinner) render() {
	timer := fmt.Sprintf(" (%s / %s)", formatDuration(s.elapsed), formatDuration(s.timeout))
	fmt.Printf("\r%c %s%s", s.frames[s.frame], s.message, timer)
}

func (s *spinner) Tick() {
	if !s.isTTY {
		return
	}
	s.frame = (s.frame + 1) % len(s.frames)
	s.render()
}

func (s *spinner) SetElapsed(elapsed time.Duration) {
	s.elapsed = elapsed
}

func (s *spinner) Stop() {
	if s.isTTY {
		clearLen := len(s.message) + 20
		fmt.Printf("\r%s\r", strings.Repeat(" ", clearLen))
	}
}
