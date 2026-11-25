package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestLoggingValidate(t *testing.T) {
	assert.Empty(t, (&RequestLogging{SampleRate: 2000}).validate(nil))
	assert.Empty(t, (&RequestLogging{SampleRate: 0}).validate(nil), "no IDs configured, logging disabled")
	assert.Len(t, (&RequestLogging{StoredRequestIDs: []string{"807"}, SampleRate: 0}).validate(nil), 1)
}
