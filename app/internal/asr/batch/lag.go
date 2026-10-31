package batch

import (
	"time"

	"tomoru/internal/engine"
)

// What a recognizer over the network measures of it, as it sends its results: package asr tells
// from them whether the network is slow, the same way for every cloud recognizer, and says so
// (docs/recognition.md, step 6). A recognizer measures what it can: a result that took d after
// its audio was all sent, or a round trip; and sending audio that stalled d, a second or more.

// minStall is the shortest stall worth telling.
const minStall = time.Second

// Took is a result, or a round trip to the service, that took d.
func Took(d time.Duration) engine.Event { return engine.Event{Type: "lag", T: d.Seconds()} }

// Stalled is sending audio that took d, if a second or more: else no event (no Type).
func Stalled(d time.Duration) engine.Event {
	if d < minStall {
		return engine.Event{}
	}
	return engine.Event{Type: "lag", T: d.Seconds(), On: true}
}
