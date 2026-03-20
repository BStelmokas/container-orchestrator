package events

import (
	"sync"
	"time"
)

// Event is one human-readable control-plane record.
type Event struct {
	Time     time.Time `json:"time"`
	Category string    `json:"category"`
	Message  string    `json:"message"`
}

// Recorder stores a bounded in-memory event history.
type Recorder struct {
	mu      sync.RWMutex
	events  []Event
	maxSize int
}

// NewRecorder constructs an event recorder with a fixed retention size.
func NewRecorder(maxSize int) *Recorder {
	if maxSize < 1 {
		maxSize = 1
	}

	return &Recorder{
		events:  make([]Event, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add appends one event and trims old history if necessary.
func (r *Recorder) Add(category, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, Event{
		Time:     time.Now(),
		Category: category,
		Message:  message,
	})

	if len(r.events) > r.maxSize {
		r.events = r.events[len(r.events)-r.maxSize:]
	}
}

// List returns the most recent events first.
func (r *Recorder) List(limit int) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit < 1 || limit > len(r.events) {
		limit = len(r.events)
	}

	result := make([]Event, 0, limit)
	for i := len(r.events) - 1; i >= 0 && len(result) < limit; i-- {
		result = append(result, r.events[i])
	}

	return result
}
