package bidadjustment

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/buger/jsonparser"
)

const (
	// pbsRequestKeyPrefix is the Redis key prefix for logged PBS bid requests.
	pbsRequestKeyPrefix = "pbs_request:"
	// storedRequestLogTTL is how long a logged bid request is kept in Redis.
	storedRequestLogTTL = time.Hour
	// storedRequestLogSlots is the number of key variants per stored request ID (suffix 0-9).
	storedRequestLogSlots = 10
	// debugLogKeySuffix is appended to the request log key for the auction's debug data.
	debugLogKeySuffix = "-debug"
)

// RequestLogger writes a sample of incoming bid requests for configured stored request IDs to Redis.
type RequestLogger struct {
	storedRequestIDs map[string]struct{}
	sampleRate       int
	randIntN         func(n int) int
	write            func(content string, ttl time.Duration, key string)
}

// NewRequestLogger creates a RequestLogger that logs one in sampleRate requests whose stored
// request ID is in storedRequestIDs. It returns nil (a no-op logger) if no IDs are configured.
func NewRequestLogger(storedRequestIDs []string, sampleRate int) *RequestLogger {
	if len(storedRequestIDs) == 0 {
		return nil
	}
	if sampleRate < 1 {
		sampleRate = 1
	}
	ids := make(map[string]struct{}, len(storedRequestIDs))
	for _, id := range storedRequestIDs {
		if id != "" {
			ids[id] = struct{}{}
		}
	}
	return &RequestLogger{
		storedRequestIDs: ids,
		sampleRate:       sampleRate,
		randIntN:         rand.IntN,
		write: func(content string, ttl time.Duration, key string) {
			// Write in the background so Redis latency does not affect the auction.
			//fmt.Println("\nRedis key", key)
			go WriteToRedis(content, ttl, key)
		},
	}
}

var requestLogger atomic.Pointer[RequestLogger]

// SetRequestLogger sets the logger used by LogRawBidRequest. Called once at startup.
func SetRequestLogger(l *RequestLogger) {
	requestLogger.Store(l)
}

// RequestLogState carries the Redis key a request was logged under (empty if it was not
// sampled) from request parsing to the auction, so its debug data can be logged alongside.
type RequestLogState struct {
	Key string
}

type requestLogStateCtxKey struct{}

// WithRequestLogState returns a context carrying a new RequestLogState, and that state.
func WithRequestLogState(ctx context.Context) (context.Context, *RequestLogState) {
	state := &RequestLogState{}
	if requestLogger.Load() == nil {
		return ctx, state
	}
	return context.WithValue(ctx, requestLogStateCtxKey{}, state), state
}

// LogRawBidRequest logs the bid request body as received from the caller (before stored
// request data is merged in), using the logger set by SetRequestLogger. If the request is
// logged, the key is recorded in the RequestLogState carried by ctx (see WithRequestLogState).
func LogRawBidRequest(ctx context.Context, requestJSON []byte) {
	key := requestLogger.Load().LogRaw(requestJSON)
	if key == "" {
		return
	}
	if state, ok := ctx.Value(requestLogStateCtxKey{}).(*RequestLogState); ok {
		state.Key = key
	}
}

// LogRaw stores the raw bid request body in Redis under
// "pbs_request:prebid-placement-stored-request-<STORED_REQUEST_ID>-<0..9>" with a one hour TTL,
// if its imp[0].ext.prebid.storedrequest.id is configured and it is picked by sampling.
// It returns the key written, or "" if the request was not logged.
func (l *RequestLogger) LogRaw(requestJSON []byte) string {
	if l == nil || len(requestJSON) == 0 {
		return ""
	}
	storedRequestID := rawStoredRequestID(requestJSON)
	if _, ok := l.storedRequestIDs[storedRequestID]; !ok {
		return ""
	}
	if l.randIntN(l.sampleRate) != 0 {
		return ""
	}
	key := storedRequestLogKey(storedRequestID, l.randIntN(storedRequestLogSlots))
	l.write(string(requestJSON), storedRequestLogTTL, key)
	return key
}

// LogDebugData stores the auction's debug data (the response ext with debug info) in Redis
// under requestLogKey + "-debug" with a one hour TTL. It does nothing if requestLogKey is empty.
func LogDebugData(requestLogKey string, debugData interface{}) {
	if requestLogKey == "" {
		return
	}
	l := requestLogger.Load()
	if l == nil {
		return
	}
	data, err := json.Marshal(debugData)
	if err != nil {
		fmt.Printf("Error marshalling debug data for Redis request log: %v\n", err)
		return
	}
	l.write(string(data), storedRequestLogTTL, requestLogKey+debugLogKeySuffix)
}

// rawStoredRequestID returns imp[0].ext.prebid.storedrequest.id from a raw bid request,
// or "" if it is missing. Numeric IDs are accepted as well as strings.
func rawStoredRequestID(requestJSON []byte) string {
	value, dataType, _, err := jsonparser.Get(requestJSON, "imp", "[0]", "ext", "prebid", "storedrequest", "id")
	if err != nil {
		return ""
	}
	switch dataType {
	case jsonparser.String:
		id, err := jsonparser.ParseString(value)
		if err != nil {
			return ""
		}
		return id
	case jsonparser.Number:
		return string(value)
	default:
		return ""
	}
}

// storedRequestLogKey builds the Redis key for a logged bid request,
// e.g. "pbs_request:prebid-placement-stored-request-807-3".
func storedRequestLogKey(storedRequestID string, slot int) string {
	return fmt.Sprintf("%sprebid-placement-stored-request-%s-%d", pbsRequestKeyPrefix, storedRequestID, slot)
}
