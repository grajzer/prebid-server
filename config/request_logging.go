package config

import "fmt"

// RequestLogging configures sampled logging of incoming bid requests to Redis.
// Only requests whose ext.prebid.storedrequest.id is listed in StoredRequestIDs are
// considered, and of those only one in SampleRate is written.
type RequestLogging struct {
	// StoredRequestIDs lists the stored request IDs whose bid requests are logged.
	// An empty list disables request logging.
	StoredRequestIDs []string `mapstructure:"stored_request_ids"`
	// SampleRate logs one in SampleRate matching requests (e.g. 2000 = one in two thousand).
	SampleRate int `mapstructure:"sample_rate"`
}

func (cfg *RequestLogging) validate(errs []error) []error {
	if len(cfg.StoredRequestIDs) > 0 && cfg.SampleRate < 1 {
		errs = append(errs, fmt.Errorf("request_logging.sample_rate must be >= 1. Got %d", cfg.SampleRate))
	}
	return errs
}
