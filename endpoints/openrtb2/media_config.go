package openrtb2

import (
	"github.com/buger/jsonparser"
	jsonpatch "gopkg.in/evanphx/json-patch.v5"
)

// mediaConfigTypes are the imp media objects that ext.prebid.media_config may override.
var mediaConfigTypes = []string{"video", "banner", "audio", "native"}

// applyMediaConfigOverride applies ext.prebid.media_config.<type> on top of imp.<type>
// when ext.prebid.media_config_override is true (both come from the stored imp).
//
// Rules:
//   - fields from the config replace the fields of the request, other request fields are kept
//     (JSON Merge Patch semantics: arrays are replaced as a whole, null removes a field);
//   - a media type that is not present in the imp is not created;
//   - without the flag, or without media_config, the imp is returned unchanged.
func applyMediaConfigOverride(imp []byte) ([]byte, error) {
	enabled, err := jsonparser.GetBoolean(imp, "ext", "prebid", "media_config_override")
	if err != nil || !enabled {
		return imp, nil
	}

	cfg, cfgType, _, err := jsonparser.Get(imp, "ext", "prebid", "media_config")
	if err != nil || cfgType != jsonparser.Object {
		return imp, nil
	}

	for _, mediaType := range mediaConfigTypes {
		override, overrideType, _, err := jsonparser.Get(cfg, mediaType)
		if err != nil || overrideType != jsonparser.Object {
			continue // not configured
		}

		current, currentType, _, err := jsonparser.Get(imp, mediaType)
		if err != nil || currentType != jsonparser.Object {
			continue // not in the request, don't create it
		}

		merged, err := jsonpatch.MergePatch(current, override)
		if err != nil {
			return nil, err
		}

		if imp, err = jsonparser.Set(imp, merged, mediaType); err != nil {
			return nil, err
		}
	}

	return imp, nil
}
