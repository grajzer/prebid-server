package bidadjustment

import (
	"encoding/json"
	"strings"
	"fmt"

	"github.com/prebid/prebid-server/v2/openrtb_ext"
)

type ExtRequest struct {
	Prebid ExtRequestPrebid                   `json:"prebid"`
}

type ExtRequestPrebid struct {
	BidAdjustmentFactors map[string]float64    `json:"bidadjustmentfactors,omitempty"`
	Bidder map[string]json.RawMessage          `json:"bidder,omitempty"`
}

func CustomAdjustement(r *openrtb_ext.RequestWrapper) map[string]float64 {
	var requestPrebid ExtRequest

	if len(r.Imp) > 0 {
		err := json.Unmarshal([]byte(r.Imp[0].Ext), &requestPrebid)
		if err != nil {
			fmt.Printf("Error unmarshaling JSON: %v", err)
		}
	}

	return getExtBidAdjustmentFactors(requestPrebid);
}

func getExtBidAdjustmentFactors(requestExt ExtRequest) map[string]float64 {
	caseInsensitiveMap := make(map[string]float64, len(requestExt.Prebid.BidAdjustmentFactors))
	for bidder, bidAdjFactor := range requestExt.Prebid.BidAdjustmentFactors {
		if (bidder == "*" || bidder == "all") {
			for b := range requestExt.Prebid.Bidder {
				caseInsensitiveMap[strings.ToLower(b)] = bidAdjFactor
			}
		} else {
			caseInsensitiveMap[strings.ToLower(bidder)] = bidAdjFactor
		}
	}
	return caseInsensitiveMap
}