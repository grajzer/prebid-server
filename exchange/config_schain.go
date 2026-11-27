package exchange

import (
	"encoding/json"
	"strings"

	"github.com/buger/jsonparser"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v4/openrtb_ext"
	"github.com/prebid/prebid-server/v4/ortb"
)

// extractBidderSchains reads the bidder-specific supply chains of the selected stored imp config:
//
//	imp.ext.prebid.schains: [{"bidders": ["appnexus"], "schain": {...}}, ...]
//
// Every chain is expected to be complete (general nodes already included where needed),
// so it only has to be picked per bidder. Bidder names are matched case-insensitively,
// the first chain wins when a bidder is listed twice, chains without nodes are skipped.
func extractBidderSchains(ext json.RawMessage) map[string]openrtb2.SupplyChain {
	var root struct {
		Prebid struct {
			Schains []struct {
				Bidders []string             `json:"bidders"`
				Schain  openrtb2.SupplyChain `json:"schain"`
			} `json:"schains"`
		} `json:"prebid"`
	}
	if len(ext) == 0 || json.Unmarshal(ext, &root) != nil {
		return nil
	}

	out := make(map[string]openrtb2.SupplyChain)
	for _, entry := range root.Prebid.Schains {
		chain := entry.Schain
		chain.Nodes = withoutEmptyASI(chain.Nodes)
		if len(chain.Nodes) == 0 {
			continue
		}
		for _, bidder := range entry.Bidders {
			key := strings.ToLower(strings.TrimSpace(bidder))
			if key == "" {
				continue
			}
			if _, exists := out[key]; !exists {
				out[key] = chain
			}
		}
	}
	return out
}

// configSChainForBidder returns the bidder's own chain from the config or, if there is none, the general one.
func configSChainForBidder(general openrtb2.SupplyChain, byBidder map[string]openrtb2.SupplyChain, bidder string) openrtb2.SupplyChain {
	if chain, ok := byBidder[strings.ToLower(bidder)]; ok {
		return chain
	}
	return general
}

// extractSchainOverride reads imp.ext.prebid.schain_override of the selected stored imp config.
// When true, the config chain replaces the supply chain of the incoming request instead of being appended to it.
func extractSchainOverride(ext json.RawMessage) bool {
	override, err := jsonparser.GetBoolean(ext, "prebid", "schain_override")
	return err == nil && override
}

// applyConfigSChain adds the config chain to the bidder request:
//   - override: the config chain replaces any supply chain of the incoming request (both locations are cleared);
//   - if the incoming request already has a supply chain (e.g. the publisher node sent by the page),
//     the config nodes are appended after its nodes, ver/complete of the request chain are kept;
//   - otherwise the config chain is used as is.
//
// The chain is read from and written to the location used by the bidder's OpenRTB version:
// source.schain for 2.6, source.ext.schain for 2.5 (call it after the down conversion).
// Source is cloned first, because it is shared between the bidder requests.
func applyConfigSChain(reqWrapper *openrtb_ext.RequestWrapper, config openrtb2.SupplyChain, ortb26, override bool) error {
	if len(config.Nodes) == 0 {
		return nil
	}

	if reqWrapper.Source == nil {
		reqWrapper.Source = &openrtb2.Source{}
	} else {
		reqWrapper.Source = ortb.CloneSource(reqWrapper.Source)
	}

	if override {
		return setSChain(reqWrapper, ortb.CloneSChain(&config), ortb26)
	}

	// OpenRTB 2.6 location
	if reqWrapper.Source.SChain != nil {
		reqWrapper.Source.SChain.Nodes = append(reqWrapper.Source.SChain.Nodes, config.Nodes...)
		return nil
	}

	// OpenRTB 2.5 location
	sourceExt, err := reqWrapper.GetSourceExt()
	if err != nil {
		return err
	}
	if current := sourceExt.GetSChain(); current != nil {
		merged := ortb.CloneSChain(current)
		merged.Nodes = append(merged.Nodes, config.Nodes...)
		sourceExt.SetSChain(merged)
		return nil
	}

	// no chain in the request: the config chain as is
	return setSChain(reqWrapper, ortb.CloneSChain(&config), ortb26)
}

// setSChain writes the chain to the location of the bidder's OpenRTB version and removes it from the other one,
// so the bidder never gets two different chains.
func setSChain(reqWrapper *openrtb_ext.RequestWrapper, chain *openrtb2.SupplyChain, ortb26 bool) error {
	sourceExt, err := reqWrapper.GetSourceExt()
	if err != nil {
		return err
	}
	if ortb26 {
		reqWrapper.Source.SChain = chain
		if sourceExt.GetSChain() != nil {
			sourceExt.SetSChain(nil)
		}
	} else {
		reqWrapper.Source.SChain = nil
		sourceExt.SetSChain(chain)
	}
	return nil
}

// withoutEmptyASI drops nodes with an empty or whitespace-only asi (same rule as extractSchainWithStdlib).
func withoutEmptyASI(nodes []openrtb2.SupplyChainNode) []openrtb2.SupplyChainNode {
	out := make([]openrtb2.SupplyChainNode, 0, len(nodes))
	for _, n := range nodes {
		if strings.TrimSpace(n.ASI) != "" {
			out = append(out, n)
		}
	}
	return out
}
