package info

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestInfoEvent(t *testing.T) {
	eventType := "test"
	ev1 := Event{
		EventType: eventType,
	}
	ev2 := NewEvent()
	ev1.DeepCopyInto(ev2)
	assert.Equal(t, eventType, ev2.EventType)
}
