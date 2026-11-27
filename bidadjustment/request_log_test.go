package bidadjustment

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type loggedWrite struct {
	content string
	ttl     time.Duration
	key     string
}

func newTestRequestLogger(ids []string, sampleRate int, randIntN func(int) int) (*RequestLogger, *[]loggedWrite) {
	writes := &[]loggedWrite{}
	l := NewRequestLogger(ids, sampleRate)
	if l != nil {
		l.randIntN = randIntN
		l.write = func(content string, ttl time.Duration, key string) {
			*writes = append(*writes, loggedWrite{content, ttl, key})
		}
	}
	return l, writes
}

const rawRequest807 = `{"id":"req-1","imp":[{"id":"imp-1","ext":{"prebid":{"storedrequest":{"id":"807"}}}}]}`

func TestStoredRequestLogKey(t *testing.T) {
	assert.Equal(t, "pbs_request:prebid-placement-stored-request-807-3", storedRequestLogKey("807", 3))
}

func TestRawStoredRequestID(t *testing.T) {
	assert.Equal(t, "807", rawStoredRequestID([]byte(rawRequest807)))
	assert.Equal(t, "1071", rawStoredRequestID([]byte(`{"imp":[{"ext":{"prebid":{"storedrequest":{"id":1071}}}}]}`)))
	assert.Equal(t, "", rawStoredRequestID([]byte(`{"id":"req-1","imp":[]}`)))
	assert.Equal(t, "", rawStoredRequestID([]byte(`not json`)))
}

func TestRequestLoggerLogsRawRequestUnchanged(t *testing.T) {
	var sampledN []int
	l, writes := newTestRequestLogger([]string{"807", "912"}, 2000, func(n int) int {
		sampledN = append(sampledN, n)
		if n == storedRequestLogSlots {
			return 7
		}
		return 0
	})

	key := l.LogRaw([]byte(rawRequest807))

	assert.Equal(t, "pbs_request:prebid-placement-stored-request-807-7", key)
	assert.Equal(t, []int{2000, 10}, sampledN)
	if assert.Len(t, *writes, 1) {
		w := (*writes)[0]
		assert.Equal(t, "pbs_request:prebid-placement-stored-request-807-7", w.key)
		assert.Equal(t, time.Hour, w.ttl)
		assert.Equal(t, rawRequest807, w.content)
	}
}

func TestRequestLoggerSkipsWhenNotSampled(t *testing.T) {
	l, writes := newTestRequestLogger([]string{"807"}, 2000, func(int) int { return 1 })
	assert.Equal(t, "", l.LogRaw([]byte(rawRequest807)))
	assert.Empty(t, *writes)
}

func TestRequestLoggerSkipsUnconfiguredStoredRequest(t *testing.T) {
	l, writes := newTestRequestLogger([]string{"999"}, 1, func(int) int { return 0 })
	l.LogRaw([]byte(rawRequest807))
	l.LogRaw([]byte(`{"id":"no-imps"}`))
	assert.Empty(t, *writes)
}

func TestRequestLoggerDisabledWithoutStoredRequestIDs(t *testing.T) {
	assert.Nil(t, NewRequestLogger(nil, 2000))
	SetRequestLogger(nil)
	ctx, state := WithRequestLogState(context.Background())
	assert.NotPanics(t, func() {
		LogRawBidRequest(ctx, []byte(rawRequest807))
		LogDebugData("pbs_request:prebid-placement-stored-request-807-1", map[string]string{"a": "b"})
	})
	assert.Equal(t, "", state.Key)
}

func TestLogRawBidRequestRecordsKeyAndLogsDebugData(t *testing.T) {
	l, writes := newTestRequestLogger([]string{"807"}, 2000, func(n int) int {
		if n == storedRequestLogSlots {
			return 4
		}
		return 0
	})
	SetRequestLogger(l)
	defer SetRequestLogger(nil)

	ctx, state := WithRequestLogState(context.Background())
	LogRawBidRequest(ctx, []byte(rawRequest807))
	assert.Equal(t, "pbs_request:prebid-placement-stored-request-807-4", state.Key)

	LogDebugData(state.Key, map[string]any{"debug": map[string]any{"httpcalls": map[string]any{}}})
	LogDebugData("", map[string]any{"ignored": true})

	if assert.Len(t, *writes, 2) {
		assert.Equal(t, "pbs_request:prebid-placement-stored-request-807-4", (*writes)[0].key)
		w := (*writes)[1]
		assert.Equal(t, "pbs_request:prebid-placement-stored-request-807-4-debug", w.key)
		assert.Equal(t, time.Hour, w.ttl)
		assert.JSONEq(t, `{"debug":{"httpcalls":{}}}`, w.content)
	}
}

func TestLogRawBidRequestNotSampledLeavesKeyEmpty(t *testing.T) {
	l, _ := newTestRequestLogger([]string{"807"}, 2000, func(int) int { return 1 })
	SetRequestLogger(l)
	defer SetRequestLogger(nil)

	ctx, state := WithRequestLogState(context.Background())
	LogRawBidRequest(ctx, []byte(rawRequest807))
	assert.Equal(t, "", state.Key)
}
