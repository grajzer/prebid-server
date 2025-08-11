package bidadjustment

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
	"github.com/prebid/prebid-server/v3/util/uuidutil"
)

type ExtRequest struct {
	Prebid ExtRequestPrebid `json:"prebid"`
}

type ExtRequestPrebid struct {
	BidAdjustmentFactors map[string]float64         `json:"bidadjustmentfactors,omitempty"`
	BidFixedPrice        float64                    `json:"bidfixedprice,omitempty"`
	BidderDebug          bool                       `json:"debug,omitempty"`
	Bidder               map[string]json.RawMessage `json:"bidder,omitempty"`
	StoredRequest        map[string]string          `json:"storedrequest,omitempty"`
}

func getExtJSON(r *openrtb_ext.RequestWrapper) ExtRequest {
	var requestPrebid ExtRequest

	if len(r.Imp) > 0 {
		err := json.Unmarshal([]byte(r.Imp[0].Ext), &requestPrebid)
		if err != nil {
			fmt.Printf("Error unmarshaling JSON: %v", err)
		}
	}
	return requestPrebid
}

func ApplyFixedPrice(r *openrtb_ext.RequestWrapper, response *openrtb2.BidResponse) *openrtb2.BidResponse {
	var bidFixedPrice = getExtJSON(r).Prebid.BidFixedPrice
	var winningBidIdx = -1
	var winningPrice float64 = 0

	if bidFixedPrice > 0 {
		for i, seatBid := range response.SeatBid { //seatBids {
			for _, bid := range seatBid.Bid {
				if (bid.Price) > winningPrice {
					winningPrice = bid.Price
					winningBidIdx = i
				}
			}
		}
		if winningBidIdx > -1 {
			response.SeatBid = response.SeatBid[winningBidIdx : winningBidIdx+1]
			response.SeatBid[0].Bid[0].Price = bidFixedPrice
		}
	}
	return response
}

func StoreToRedis(r *openrtb_ext.RequestWrapper, response *openrtb2.BidResponse) *openrtb2.BidResponse {

	//fmt.Println("\nRequest Domain:", r.Site.Domain)
	var domain = ""
	var placementId = ""
	var storedRequest = getExtJSON(r).Prebid.StoredRequest

	var winningBidIdx = -1
	var winningPrice float64 = 0

	for field, val := range storedRequest {
		if field == "id" {
			placementId = val
		}
	}

	if r.Site.Domain != "" {
		domain = r.Site.Domain
	}

	//err := PrintJSONIndented(response)
	//if err != nil {
	//	return nil
	//}

	//fmt.Print(response.SeatBid[0].Bid[0].AdM)

	//fmt.Println("\nREDIS KEY:", redisKey)
	if len(response.SeatBid) > 0 {

		jsonSeatBid, _ := GetJSONIndented(response.SeatBid)
		redisKey, errRedis := addToRedis(jsonSeatBid, 10*time.Minute, "")
		//redisKey = redisKey + ""
		if errRedis == nil {
			for i, seatBid := range response.SeatBid {

				jsonSingleSeat, _ := GetJSONIndented(seatBid)
				redisKeyBidder, _ := addToRedis(jsonSingleSeat, 24*time.Hour, seatBid.Seat)
				redisKeyBidder = redisKeyBidder + ""
				//fmt.Println("\nREDIS KEY:", redisKeyBidder)

				for _, bid := range seatBid.Bid {
					if (bid.Price) > winningPrice {
						winningPrice = bid.Price
						winningBidIdx = i
					}
				}
			}

			if winningBidIdx > -1 {
				response.SeatBid = response.SeatBid[winningBidIdx : winningBidIdx+1]
				response.SeatBid[0].Bid[0].AdM = "<VAST version=\"3.0\">\n<Ad>\n <Wrapper>\n   <AdSystem>TargetVideo wrapper</AdSystem>\n   <VASTAdTagURI><![CDATA[https://vid.tvserve.io/ads/bid?iu=/2/target-video/" + domain + "&bid_hash=" + redisKey + "&placement_id=" + placementId + "]]></VASTAdTagURI>\n   <Creatives></Creatives>\n </Wrapper>\n</Ad>\n</VAST>"
				response.SeatBid[0].Bid[0].NURL = "https://vid.tvserve.io/ads/bid?iu=/2/target-video/" + domain + "&bid_hash=" + redisKey + "&placement_id=" + placementId
				response.SeatBid[0].Bid[0].DealID = ""
				response.SeatBid[0].Bid[0].CID = ""
				//response.SeatBid[0].Bid[0].CrID = ""
				response.SeatBid[0].Seat = "targetVideo"
				//response.SeatBid[0].Bid[0].Ext = nil
			}
		}

	}

	return response
}

func CleanBidExtensions(response *openrtb2.BidResponse) *openrtb2.BidResponse {
	for i, seatBid := range response.SeatBid {
		for j, bid := range seatBid.Bid {
			if bid.Ext != nil {
				var extMap map[string]interface{}
				if err := json.Unmarshal(bid.Ext, &extMap); err == nil {
					delete(extMap, "origbidcpm")
					if cleanExt, err := json.Marshal(extMap); err == nil {
						response.SeatBid[i].Bid[j].Ext = cleanExt
					}
				}
			}
		}
	}
	return response
}

func CustomAdjustement(r *openrtb_ext.RequestWrapper) map[string]float64 {
	return getExtBidAdjustmentFactors(getExtJSON(r))
}

func ApplyDebugFlag(r *openrtb_ext.RequestWrapper, requestExtPrebid *openrtb_ext.ExtRequestPrebid) *openrtb_ext.ExtRequestPrebid {
	var bidderDebug = getExtJSON(r).Prebid.BidderDebug

	if bidderDebug {
		requestExtPrebid.Debug = true
	}

	return requestExtPrebid
}

func getExtBidAdjustmentFactors(requestExt ExtRequest) map[string]float64 {
	caseInsensitiveMap := make(map[string]float64, len(requestExt.Prebid.BidAdjustmentFactors))
	for bidder, bidAdjFactor := range requestExt.Prebid.BidAdjustmentFactors {
		if bidder == "*" || bidder == "all" {
			for b := range requestExt.Prebid.Bidder {
				caseInsensitiveMap[strings.ToLower(b)] = bidAdjFactor
			}
		} else {
			caseInsensitiveMap[strings.ToLower(bidder)] = bidAdjFactor
		}
	}
	return caseInsensitiveMap

}
func PrintJSONIndented(v interface{}) error {
	//jsonStr, err := ToJSONIndented(v)
	jsonString, err := GetJSONIndented(v)
	if err != nil {
		return nil
	}

	fmt.Print(jsonString)
	return nil
}

func GetJSONIndented(v interface{}) (string, error) {
	jsonData, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

func addToRedis(strVal string, expiration time.Duration, bidderKey string) (string, error) {
	// Create a new Redis client
	//redisClient := NewClient("ad-server-globalb.ay4fls.clustercfg.euc1.cache.amazonaws.com:6379", "", 0)
	redisClient := GetRedisClient()

	//Cluster name : ad-server-uid
	//redisClient := NewClient("ad-server-uid.ay4fls.clustercfg.euc1.cache.amazonaws.com:6379", "", 0)

	//Cluster name : staging
	//redisClient := NewClient("ad-server-stage.ay4fls.clustercfg.euc1.cache.amazonaws.com:6379", "", 0)

	// Generate a Redis key. If bidderKey is true, add a bidder as prefix for debug
	uuidGenerator := uuidutil.UUIDRandomGenerator{}
	uuidStr, _ := uuidGenerator.Generate()
	keyRedis := uuidStr
	if bidderKey != "" {
		keyRedis = fmt.Sprintf("%s-%d", "bidder-debug-"+bidderKey, rand.IntN(10))
	}

	//fmt.Println("\nRedis Key:", keyRedis)

	// Add a string with expiration time
	err := redisClient.AddStringWithExpiration(keyRedis, strVal, expiration)
	if err != nil {
		fmt.Printf("Error adding string with expiration to Redis: %v\n", err)
		return "", err
	}

	//fmt.Println("\nSuccessfully added strings to Redis")

	return keyRedis, err
}
