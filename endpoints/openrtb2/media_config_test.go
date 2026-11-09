package openrtb2

import (
	"encoding/json"
	"testing"

	"github.com/buger/jsonparser"
	"github.com/prebid/prebid-server/v4/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyMediaConfigOverride(t *testing.T) {
	testCases := []struct {
		name     string
		imp      string
		expected string
	}{
		{
			name: "override-applied-request-fields-kept",
			imp: `{"id":"1","video":{"plcmt":2,"maxduration":300,"mimes":["video/mp4","video/x-ms-wmv"],"w":640,"h":480},` +
				`"ext":{"prebid":{"media_config_override":true,"media_config":{"video":{"plcmt":1,"maxduration":30,"mimes":["video/mp4"],"skip":1}}}}}`,
			expected: `{"id":"1","video":{"plcmt":1,"maxduration":30,"mimes":["video/mp4"],"w":640,"h":480,"skip":1},` +
				`"ext":{"prebid":{"media_config_override":true,"media_config":{"video":{"plcmt":1,"maxduration":30,"mimes":["video/mp4"],"skip":1}}}}}`,
		},
		{
			name: "media-type-missing-in-request-not-created",
			imp: `{"id":"1","video":{"plcmt":2},` +
				`"ext":{"prebid":{"media_config_override":true,"media_config":{"video":{"plcmt":1},"banner":{"format":[{"w":300,"h":250}]}}}}}`,
			expected: `{"id":"1","video":{"plcmt":1},` +
				`"ext":{"prebid":{"media_config_override":true,"media_config":{"video":{"plcmt":1},"banner":{"format":[{"w":300,"h":250}]}}}}}`,
		},
		{
			name: "several-media-types",
			imp: `{"id":"1","video":{"plcmt":2},"banner":{"format":[{"w":728,"h":90}]},` +
				`"ext":{"prebid":{"media_config_override":true,"media_config":{"video":{"plcmt":1},"banner":{"format":[{"w":300,"h":250}]}}}}}`,
			expected: `{"id":"1","video":{"plcmt":1},"banner":{"format":[{"w":300,"h":250}]},` +
				`"ext":{"prebid":{"media_config_override":true,"media_config":{"video":{"plcmt":1},"banner":{"format":[{"w":300,"h":250}]}}}}}`,
		},
		{
			name: "null-removes-field",
			imp: `{"id":"1","video":{"plcmt":2,"skip":1},` +
				`"ext":{"prebid":{"media_config_override":true,"media_config":{"video":{"skip":null}}}}}`,
			expected: `{"id":"1","video":{"plcmt":2},` +
				`"ext":{"prebid":{"media_config_override":true,"media_config":{"video":{"skip":null}}}}}`,
		},
		{
			name: "flag-false-unchanged",
			imp: `{"id":"1","video":{"plcmt":2},` +
				`"ext":{"prebid":{"media_config_override":false,"media_config":{"video":{"plcmt":1}}}}}`,
			expected: `{"id":"1","video":{"plcmt":2},` +
				`"ext":{"prebid":{"media_config_override":false,"media_config":{"video":{"plcmt":1}}}}}`,
		},
		{
			name:     "flag-missing-unchanged",
			imp:      `{"id":"1","video":{"plcmt":2},"ext":{"prebid":{"media_config":{"video":{"plcmt":1}}}}}`,
			expected: `{"id":"1","video":{"plcmt":2},"ext":{"prebid":{"media_config":{"video":{"plcmt":1}}}}}`,
		},
		{
			name:     "media-config-missing-unchanged",
			imp:      `{"id":"1","video":{"plcmt":2},"ext":{"prebid":{"media_config_override":true}}}`,
			expected: `{"id":"1","video":{"plcmt":2},"ext":{"prebid":{"media_config_override":true}}}`,
		},
		{
			name:     "media-config-not-object-unchanged",
			imp:      `{"id":"1","video":{"plcmt":2},"ext":{"prebid":{"media_config_override":true,"media_config":[1,2]}}}`,
			expected: `{"id":"1","video":{"plcmt":2},"ext":{"prebid":{"media_config_override":true,"media_config":[1,2]}}}`,
		},
		{
			name:     "media-type-value-not-object-skipped",
			imp:      `{"id":"1","video":{"plcmt":2},"ext":{"prebid":{"media_config_override":true,"media_config":{"video":1}}}}`,
			expected: `{"id":"1","video":{"plcmt":2},"ext":{"prebid":{"media_config_override":true,"media_config":{"video":1}}}}`,
		},
		{
			name:     "no-ext-unchanged",
			imp:      `{"id":"1","video":{"plcmt":2}}`,
			expected: `{"id":"1","video":{"plcmt":2}}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := applyMediaConfigOverride([]byte(tc.imp))
			require.NoError(t, err)
			assert.JSONEq(t, tc.expected, string(actual))
		})
	}
}

// The incoming imp wins over the stored imp in the merge, the override must still win over both.
func TestProcessStoredRequestsAppliesMediaConfigOverride(t *testing.T) {
	deps := &endpointDeps{cfg: &config.Configuration{}}

	storedImps := map[string]json.RawMessage{
		"1064": json.RawMessage(`{"id":"1064","video":{"plcmt":3,"maxduration":60},` +
			`"ext":{"prebid":{"config_id":"359","bidder":{"appnexus":{"placement_id":1}},` +
			`"media_config_override":true,"media_config":{"video":{"plcmt":1,"maxduration":30}}}}}`),
	}
	request := `{"id":"r1","site":{"page":"https://example.com"},"imp":[{"id":"imp-1",` +
		`"video":{"plcmt":2,"maxduration":300,"mimes":["video/mp4"],"w":640,"h":480},` +
		`"ext":{"prebid":{"storedrequest":{"id":"1064"}}}}]}`

	impInfo, errs := parseImpInfo([]byte(request))
	require.Empty(t, errs)

	resolved, _, errs := deps.processStoredRequests(json.RawMessage(request), impInfo, nil, storedImps, "", false)
	require.Empty(t, errs)

	video, _, _, err := jsonparser.Get(resolved, "imp", "[0]", "video")
	require.NoError(t, err)
	assert.JSONEq(t, `{"plcmt":1,"maxduration":30,"mimes":["video/mp4"],"w":640,"h":480}`, string(video))
}
