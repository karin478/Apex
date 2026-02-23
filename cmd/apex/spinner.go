package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

var brailleFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner displays an animated spinner with a message on the current terminal line.
type Spinner struct {
	mu      sync.Mutex
	message string
	detail  string
	start   time.Time
	stop    chan struct{}
	done    chan struct{}
}

// NewSpinner creates and starts a spinner with the given message.
func NewSpinner(message string) *Spinner {
	s := &Spinner{
		message: message,
		start:   time.Now(),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go s.run()
	return s
}

// NewSpinnerWithDetail creates and starts a spinner with a message and detail
// (e.g. model name) that is shown alongside the elapsed time.
func NewSpinnerWithDetail(message, detail string) *Spinner {
	s := &Spinner{
		message: message,
		detail:  detail,
		start:   time.Now(),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go s.run()
	return s
}

// Update changes the spinner message while it's running.
func (s *Spinner) Update(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

// Stop halts the spinner and clears the line.
func (s *Spinner) Stop() {
	close(s.stop)
	<-s.done
}

func (s *Spinner) run() {
	defer close(s.done)
	tick := time.NewTicker(80 * time.Millisecond)
	defer tick.Stop()

	frame := 0
	for {
		select {
		case <-s.stop:
			// Clear the spinner line
			fmt.Printf("\r%s\r", strings.Repeat(" ", 100))
			return
		case <-tick.C:
			s.mu.Lock()
			msg := s.message
			detail := s.detail
			s.mu.Unlock()

			elapsed := time.Since(s.start).Seconds()
			var line string
			if detail != "" {
				line = fmt.Sprintf("\r  %s %s (%s · %.1fs)",
					styleSpinner.Render(brailleFrames[frame]),
					styleDim.Render(msg),
					styleDim.Render(detail),
					elapsed)
			} else {
				line = fmt.Sprintf("\r  %s %s (%.1fs)",
					styleSpinner.Render(brailleFrames[frame]),
					styleDim.Render(msg),
					elapsed)
			}
			fmt.Print(line)
			frame = (frame + 1) % len(brailleFrames)
		}
	}
}
