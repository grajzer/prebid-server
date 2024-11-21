package bidadjustment

import (
	"encoding/json"
	"strings"
	"fmt"

	"github.com/prebid/prebid-server/v2/openrtb_ext"
	"github.com/prebid/openrtb/v20/openrtb2"
)

type ExtRequest struct {
	Prebid ExtRequestPrebid                   `json:"prebid"`
}

type ExtRequestPrebid struct {
	BidAdjustmentFactors map[string]float64   `json:"bidadjustmentfactors,omitempty"`
	BidFixedPrice float64                     `json:"bidfixedprice,omitempty"`
	Bidder map[string]json.RawMessage         `json:"bidder,omitempty"`
}

func getExtJSON(r *openrtb_ext.RequestWrapper) ExtRequest {
	var requestPrebid ExtRequest

	if len(r.Imp) > 0 {
		err := json.Unmarshal([]byte(r.Imp[0].Ext), &requestPrebid)
		if err != nil {
			fmt.Printf("Error unmarshaling JSON: %v", err)
		}
	}
	return requestPrebid;
}

func ApplyFixedPrice(r *openrtb_ext.RequestWrapper, response *openrtb2.BidResponse) *openrtb2.BidResponse {
	var bidFixedPrice = getExtJSON(r).Prebid.BidFixedPrice;
	var winningBidIdx = -1;
	var winningPrice float64 = 0;
	if (bidFixedPrice > 0) {
		for i, seatBid := range response.SeatBid { //seatBids {
			for _, bid := range seatBid.Bid {
				if (bid.Price) > winningPrice {
					winningPrice = bid.Price;
					winningBidIdx = i;
				}
			}
		}
		response.SeatBid = response.SeatBid[winningBidIdx : winningBidIdx + 1]
		response.SeatBid[0].Bid[0].Price = bidFixedPrice
	}
	return response;
}

func CustomAdjustement(r *openrtb_ext.RequestWrapper) map[string]float64 {
	return getExtBidAdjustmentFactors(getExtJSON(r));
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