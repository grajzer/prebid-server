package exchange

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v4/config"
	"github.com/prebid/prebid-server/v4/gdpr"
	"github.com/prebid/prebid-server/v4/metrics"
	"github.com/prebid/prebid-server/v4/openrtb_ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func node(asi, sid string) openrtb2.SupplyChainNode {
	return openrtb2.SupplyChainNode{ASI: asi, SID: sid, HP: openrtb2.Int8Ptr(1)}
}

func asis(chain *openrtb2.SupplyChain) []string {
	if chain == nil {
		return nil
	}
	out := make([]string, 0, len(chain.Nodes))
	for _, n := range chain.Nodes {
		out = append(out, n.ASI+"/"+n.SID)
	}
	return out
}

func TestExtractBidderSchains(t *testing.T) {
	ext := json.RawMessage(`{"prebid":{"schains":[
		{"bidders":["appnexus","Rubicon"],"schain":{"ver":"1.0","complete":1,"nodes":[{"asi":"a.com","sid":"1","hp":1}]}},
		{"bidders":["appnexus"],"schain":{"ver":"1.0","complete":1,"nodes":[{"asi":"dup.com","sid":"2","hp":1}]}},
		{"bidders":["yieldlab"],"schain":{"ver":"1.0","complete":1,"nodes":[{"asi":" ","sid":"3","hp":1}]}},
		{"bidders":[""],"schain":{"ver":"1.0","complete":1,"nodes":[{"asi":"x.com","sid":"4","hp":1}]}}
	]}}`)

	got := extractBidderSchains(ext)

	require.Len(t, got, 2)
	assert.Equal(t, []string{"a.com/1"}, asis(ptr(got["appnexus"])), "first chain wins for a duplicated bidder")
	assert.Equal(t, []string{"a.com/1"}, asis(ptr(got["rubicon"])), "bidder names are lower-cased")
	assert.NotContains(t, got, "yieldlab", "chain with only empty asi is skipped")

	assert.Nil(t, extractBidderSchains(nil))
	assert.Empty(t, extractBidderSchains(json.RawMessage(`{"prebid":{}}`)))
	assert.Nil(t, extractBidderSchains(json.RawMessage(`{bad`)))
}

func TestConfigSChainForBidder(t *testing.T) {
	general := openrtb2.SupplyChain{Ver: "1.0", Nodes: []openrtb2.SupplyChainNode{node("general.com", "1")}}
	byBidder := map[string]openrtb2.SupplyChain{"appnexus": {Ver: "1.0", Nodes: []openrtb2.SupplyChainNode{node("own.com", "2")}}}

	own := configSChainForBidder(general, byBidder, "AppNexus")
	other := configSChainForBidder(general, byBidder, "rubicon")
	assert.Equal(t, []string{"own.com/2"}, asis(&own))
	assert.Equal(t, []string{"general.com/1"}, asis(&other))
}

func TestAppendConfigSChain(t *testing.T) {
	config := openrtb2.SupplyChain{Ver: "1.0", Complete: 1, Nodes: []openrtb2.SupplyChainNode{node("intermediary.example", "int-1")}}
	requestChain := func() *openrtb2.SupplyChain {
		return &openrtb2.SupplyChain{Ver: "1.0", Complete: 1, Nodes: []openrtb2.SupplyChainNode{node("publisher.example", "pub-1")}}
	}

	t.Run("ortb26-request-chain-config-appended", func(t *testing.T) {
		shared := &openrtb2.Source{TID: "t", SChain: requestChain()}
		rw := &openrtb_ext.RequestWrapper{BidRequest: &openrtb2.BidRequest{Source: shared}}

		require.NoError(t, appendConfigSChain(rw, config, true))
		require.NoError(t, rw.RebuildRequest())

		assert.Equal(t, []string{"publisher.example/pub-1", "intermediary.example/int-1"}, asis(rw.Source.SChain))
		assert.Equal(t, []string{"publisher.example/pub-1"}, asis(shared.SChain), "shared source must not be modified")
	})

	t.Run("ortb25-request-chain-in-ext-config-appended", func(t *testing.T) {
		shared := &openrtb2.Source{TID: "t", Ext: json.RawMessage(`{"schain":{"ver":"1.0","complete":1,"nodes":[{"asi":"publisher.example","sid":"pub-1","hp":1}]},"other":1}`)}
		sharedExt := string(shared.Ext)
		rw := &openrtb_ext.RequestWrapper{BidRequest: &openrtb2.BidRequest{Source: shared}}

		require.NoError(t, appendConfigSChain(rw, config, false))
		require.NoError(t, rw.RebuildRequest())

		assert.Nil(t, rw.Source.SChain)
		assert.Equal(t, []string{"publisher.example/pub-1", "intermediary.example/int-1"}, asis(sourceExtSChain(t, rw.Source.Ext)))
		assert.Contains(t, string(rw.Source.Ext), `"other":1`, "other source.ext fields are kept")
		assert.Equal(t, sharedExt, string(shared.Ext), "shared source must not be modified")
	})

	t.Run("no-request-chain-ortb26", func(t *testing.T) {
		rw := &openrtb_ext.RequestWrapper{BidRequest: &openrtb2.BidRequest{Source: &openrtb2.Source{TID: "t"}}}

		require.NoError(t, appendConfigSChain(rw, config, true))
		require.NoError(t, rw.RebuildRequest())

		assert.Equal(t, []string{"intermediary.example/int-1"}, asis(rw.Source.SChain))
		assert.Empty(t, rw.Source.Ext)
	})

	t.Run("no-request-chain-ortb25", func(t *testing.T) {
		rw := &openrtb_ext.RequestWrapper{BidRequest: &openrtb2.BidRequest{Source: &openrtb2.Source{TID: "t"}}}

		require.NoError(t, appendConfigSChain(rw, config, false))
		require.NoError(t, rw.RebuildRequest())

		assert.Nil(t, rw.Source.SChain)
		chain := sourceExtSChain(t, rw.Source.Ext)
		assert.Equal(t, []string{"intermediary.example/int-1"}, asis(chain))
		assert.Equal(t, "1.0", chain.Ver)
		assert.Equal(t, int8(1), chain.Complete)
	})

	t.Run("no-source-in-request", func(t *testing.T) {
		rw := &openrtb_ext.RequestWrapper{BidRequest: &openrtb2.BidRequest{}}

		require.NoError(t, appendConfigSChain(rw, config, true))

		require.NotNil(t, rw.Source)
		assert.Equal(t, []string{"intermediary.example/int-1"}, asis(rw.Source.SChain))
	})

	t.Run("empty-config-nothing-changes", func(t *testing.T) {
		shared := &openrtb2.Source{TID: "t", SChain: requestChain()}
		rw := &openrtb_ext.RequestWrapper{BidRequest: &openrtb2.BidRequest{Source: shared}}

		require.NoError(t, appendConfigSChain(rw, openrtb2.SupplyChain{}, true))

		assert.Same(t, shared, rw.Source)
	})
}

// End to end through the request splitter: the page sends the publisher node, the config adds
// an intermediary node (general) and a bidder-specific chain; bidders on OpenRTB 2.5 and 2.6.
func TestCleanOpenRTBRequestsConfigSChain(t *testing.T) {
	req := &openrtb2.BidRequest{
		ID:   "r1",
		Site: &openrtb2.Site{},
		Source: &openrtb2.Source{
			TID: "testTID",
			// the request is normalized to OpenRTB 2.6 before the auction
			SChain: &openrtb2.SupplyChain{Ver: "1.0", Complete: 1, Nodes: []openrtb2.SupplyChainNode{node("publisher.example", "pub-1")}},
		},
		Imp: []openrtb2.Imp{{
			ID: "imp-1",
			Ext: json.RawMessage(`{"prebid":{
				"bidder":{"appnexus":{"placementId":1},"rubicon":{"accountId":1,"siteId":2,"zoneId":3},"yieldlab":{"adslotId":"1","supplyId":"2"}},
				"schain":{"ver":"1.0","complete":1,"nodes":[{"asi":"intermediary.example","sid":"int-1","hp":1}]},
				"schains":[{"bidders":["appnexus"],"schain":{"ver":"1.0","complete":1,"nodes":[
					{"asi":"intermediary.example","sid":"int-1","hp":1},{"asi":"reseller.example","sid":"res-1","hp":1}]}}]
			}}`),
		}},
	}

	auctionReq := AuctionRequest{
		BidRequestWrapper: &openrtb_ext.RequestWrapper{BidRequest: req},
		UserSyncs:         &emptyUsersync{},
		TCF2Config:        gdpr.NewTCF2Config(config.TCF2{}, config.AccountGDPR{}),
	}
	gdprPermissionsBuilder := fakePermissionsBuilder{
		permissions: &permissionsMock{allowAllBidders: true, passGeo: true, passID: true},
	}.Builder
	ortb26 := config.BidderInfo{OpenRTB: &config.OpenRTBInfo{Version: "2.6"}}
	reqSplitter := &requestSplitter{
		bidderToSyncerKey: map[string]string{},
		me:                &metrics.MetricsEngineMock{},
		privacyConfig:     config.Privacy{},
		gdprPermsBuilder:  gdprPermissionsBuilder,
		bidderInfo:        config.BidderInfos{"appnexus": config.BidderInfo{}, "rubicon": ortb26, "yieldlab": config.BidderInfo{}},
	}

	bidderRequests, _, errs := reqSplitter.cleanOpenRTBRequests(context.Background(), auctionReq, &openrtb_ext.ExtRequest{}, map[string]float64{})
	require.Empty(t, errs)
	require.Len(t, bidderRequests, 3)

	got := map[string]*openrtb2.Source{}
	for _, br := range bidderRequests {
		got[string(br.BidderName)] = br.BidRequest.Source
	}

	// appnexus, OpenRTB 2.5: own chain (append mode) after the page node, in source.ext.schain
	assert.Nil(t, got["appnexus"].SChain)
	assert.Equal(t, []string{"publisher.example/pub-1", "intermediary.example/int-1", "reseller.example/res-1"},
		asis(sourceExtSChain(t, got["appnexus"].Ext)))

	// rubicon, OpenRTB 2.6: general chain after the page node, in source.schain
	assert.Equal(t, []string{"publisher.example/pub-1", "intermediary.example/int-1"}, asis(got["rubicon"].SChain))

	// yieldlab, OpenRTB 2.5: general chain after the page node, in source.ext.schain
	assert.Nil(t, got["yieldlab"].SChain)
	assert.Equal(t, []string{"publisher.example/pub-1", "intermediary.example/int-1"}, asis(sourceExtSChain(t, got["yieldlab"].Ext)))

	// the original request is not modified
	assert.Equal(t, []string{"publisher.example/pub-1"}, asis(req.Source.SChain))
}

func sourceExtSChain(t *testing.T, ext json.RawMessage) *openrtb2.SupplyChain {
	t.Helper()
	var parsed struct {
		SChain *openrtb2.SupplyChain `json:"schain"`
	}
	require.NoError(t, json.Unmarshal(ext, &parsed))
	require.NotNil(t, parsed.SChain, "source.ext.schain is missing")
	return parsed.SChain
}

func ptr(c openrtb2.SupplyChain) *openrtb2.SupplyChain { return &c }
