/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fabricselection

import (
	"fmt"
	"strings"
	"testing"
	"time"

	clientmocks "github.com/blockchain/fabric-sdk-go/pkg/client/common/mocks"
	"github.com/blockchain/fabric-sdk-go/pkg/client/common/selection/options"
	contextAPI "github.com/blockchain/fabric-sdk-go/pkg/common/providers/context"
	fab "github.com/blockchain/fabric-sdk-go/pkg/common/providers/fab"
	discmocks "github.com/blockchain/fabric-sdk-go/pkg/fab/discovery/mocks"
	"github.com/blockchain/fabric-sdk-go/pkg/fab/mocks"
	mspmocks "github.com/blockchain/fabric-sdk-go/pkg/msp/test/mockmsp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	channelID = "testchannel"
	cc1       = "cc1"
	cc1Col1   = "cc1col1"
	cc1Col2   = "cc1col2"
	cc2       = "cc2"
	cc2Col1   = "cc2col1"

	mspID1       = "Org1MSP"
	peer1Org1URL = "peer1.org1.com:9999"
	peer2Org1URL = "peer2.org1.com:9999"

	mspID2       = "Org2MSP"
	peer1Org2URL = "peer1.org2.com:9999"
	peer2Org2URL = "peer2.org2.com:9999"

	mspID3       = "Org3MSP"
	peer1Org3URL = "peer1.org3.com:9999"
	peer2Org3URL = "peer2.org3.com:9999"
)

var (
	peer1Org1 = mocks.NewMockPeer("p11", peer1Org1URL)
	peer2Org1 = mocks.NewMockPeer("p12", peer2Org1URL)
	peer1Org2 = mocks.NewMockPeer("p21", peer1Org2URL)
	peer2Org2 = mocks.NewMockPeer("p22", peer2Org2URL)
	peer1Org3 = mocks.NewMockPeer("p31", peer1Org3URL)
	peer2Org3 = mocks.NewMockPeer("p32", peer2Org3URL)

	channelPeers = []fab.ChannelPeer{
		{NetworkPeer: newPeerConfig(peer1Org1URL, mspID1)},
		{NetworkPeer: newPeerConfig(peer2Org1URL, mspID1)},
		{NetworkPeer: newPeerConfig(peer1Org2URL, mspID2)},
		{NetworkPeer: newPeerConfig(peer2Org2URL, mspID2)},
		{NetworkPeer: newPeerConfig(peer1Org3URL, mspID3)},
		{NetworkPeer: newPeerConfig(peer2Org3URL, mspID3)},
	}

	peer1Org1Endpoint = &discmocks.MockDiscoveryPeerEndpoint{
		MSPID:        mspID1,
		Endpoint:     peer1Org1URL,
		LedgerHeight: 1000,
	}
	peer2Org1Endpoint = &discmocks.MockDiscoveryPeerEndpoint{
		MSPID:        mspID1,
		Endpoint:     peer2Org1URL,
		LedgerHeight: 1002,
	}
	peer1Org2Endpoint = &discmocks.MockDiscoveryPeerEndpoint{
		MSPID:        mspID2,
		Endpoint:     peer1Org2URL,
		LedgerHeight: 1001,
	}
	peer2Org2Endpoint = &discmocks.MockDiscoveryPeerEndpoint{
		MSPID:        mspID2,
		Endpoint:     peer2Org2URL,
		LedgerHeight: 1003,
	}
	peer1Org3Endpoint = &discmocks.MockDiscoveryPeerEndpoint{
		MSPID:        mspID3,
		Endpoint:     peer1Org3URL,
		LedgerHeight: 1000,
	}
	peer2Org3Endpoint = &discmocks.MockDiscoveryPeerEndpoint{
		MSPID:        mspID3,
		Endpoint:     peer2Org3URL,
		LedgerHeight: 1003,
	}

	cc1ChaincodeCall = &fab.ChaincodeCall{
		ID:          cc1,
		Collections: []string{cc1Col1, cc1Col2},
	}
	cc2ChaincodeCall = &fab.ChaincodeCall{
		ID:          cc2,
		Collections: []string{cc2Col1},
	}
)

func TestSelection(t *testing.T) {
	ctx := mocks.NewMockContext(mspmocks.NewMockSigningIdentity("test", mspID1))
	config := &config{
		EndpointConfig: mocks.NewMockEndpointConfig(),
		peers:          channelPeers,
	}
	ctx.SetEndpointConfig(config)

	discClient := clientmocks.NewMockDiscoveryClient()

	clientProvider = func(ctx contextAPI.Client) (discoveryClient, error) {
		return discClient, nil
	}

	service, err := New(
		ctx, channelID,
		mocks.NewMockDiscoveryService(nil, peer1Org1, peer2Org1, peer1Org2, peer2Org2, peer1Org3, peer2Org3),
		WithRefreshInterval(100*time.Millisecond),
		WithResponseTimeout(10*time.Millisecond),
	)
	require.NoError(t, err)
	defer service.Close()

	t.Run("Error", func(t *testing.T) {
		// Error condition
		discClient.SetResponses(
			&clientmocks.MockDiscoverEndpointResponse{
				PeerEndpoints: []*discmocks.MockDiscoveryPeerEndpoint{},
				Error:         fmt.Errorf("simulated response error"),
			},
		)
		endorsers, err := service.GetEndorsersForChaincode([]*fab.ChaincodeCall{{ID: cc1}})
		assert.Error(t, err)
		assert.Equal(t, 0, len(endorsers))
	})

	t.Run("CCtoCC", func(t *testing.T) {
		discClient.SetResponses(
			&clientmocks.MockDiscoverEndpointResponse{
				PeerEndpoints: []*discmocks.MockDiscoveryPeerEndpoint{
					peer2Org1Endpoint, peer2Org3Endpoint, peer2Org2Endpoint,
					peer1Org1Endpoint, peer1Org2Endpoint, peer1Org3Endpoint,
				},
			},
		)

		// Wait for cache to refresh
		time.Sleep(200 * time.Millisecond)

		endorsers, err := service.GetEndorsersForChaincode([]*fab.ChaincodeCall{cc1ChaincodeCall, cc2ChaincodeCall})

		assert.NoError(t, err)
		assert.Equalf(t, 6, len(endorsers), "Expecting 6 endorser")
	})

	t.Run("Peer Filter", func(t *testing.T) {
		endorsers, err := service.GetEndorsersForChaincode([]*fab.ChaincodeCall{{ID: cc1}},
			options.WithPeerFilter(func(peer fab.Peer) bool {
				return peer.(fab.PeerState).BlockHeight() > 1001
			}),
		)

		assert.NoError(t, err)
		assert.Equalf(t, 3, len(endorsers), "Expecting 3 endorser")

		// Ensure the endorsers all have a block height > 1001 and they are returned in descending order of block height
		lastBlockHeight := uint64(9999999)
		for _, endorser := range endorsers {
			blockHeight := endorser.(fab.PeerState).BlockHeight()
			assert.Truef(t, blockHeight > 1001, "Expecting block height to be > 1001")
			assert.Truef(t, blockHeight <= lastBlockHeight, "Expecting endorsers to be returned in order of descending block height. Block Height: %d, Last Block Height: %d", blockHeight, lastBlockHeight)
			lastBlockHeight = blockHeight
		}
	})

	t.Run("Priority Selector", func(t *testing.T) {
		endorsers, err := service.GetEndorsersForChaincode([]*fab.ChaincodeCall{{ID: cc1}},
			options.WithPrioritySelector(func(peer1, peer2 fab.Peer) int {
				// Return peers in alphabetical order
				if peer1.URL() < peer2.URL() {
					return -1
				}
				if peer1.URL() > peer2.URL() {
					return 1
				}
				return 0
			}),
		)

		assert.NoError(t, err)
		assert.Equalf(t, 6, len(endorsers), "Expecting 6 endorser")

		var lastURL string
		for _, endorser := range endorsers {
			if lastURL != "" {
				assert.Truef(t, endorser.URL() <= lastURL, "Expecting endorsers in alphabetical order")
			}
			lastURL = endorser.URL()
		}
	})
}

func TestWithDiscoveryFilter(t *testing.T) {
	ctx := mocks.NewMockContext(mspmocks.NewMockSigningIdentity("test", mspID1))
	config := &config{
		EndpointConfig: mocks.NewMockEndpointConfig(),
		peers:          channelPeers,
	}
	ctx.SetEndpointConfig(config)

	discClient := clientmocks.NewMockDiscoveryClient()
	clientProvider = func(ctx contextAPI.Client) (discoveryClient, error) {
		return discClient, nil
	}

	discClient.SetResponses(
		&clientmocks.MockDiscoverEndpointResponse{
			PeerEndpoints: []*discmocks.MockDiscoveryPeerEndpoint{
				peer2Org1Endpoint, peer2Org3Endpoint, peer2Org2Endpoint,
				peer1Org1Endpoint, peer1Org2Endpoint, peer1Org3Endpoint,
			},
		},
	)

	t.Run("Error", func(t *testing.T) {
		expectedDiscoveryErrMsg := "simulated discovery service error"
		service, err := New(
			ctx, channelID,
			mocks.NewMockDiscoveryService(fmt.Errorf(expectedDiscoveryErrMsg)),
			WithRefreshInterval(500*time.Millisecond),
			WithResponseTimeout(2*time.Second),
		)
		require.NoError(t, err)
		defer service.Close()

		_, err = service.GetEndorsersForChaincode([]*fab.ChaincodeCall{cc1ChaincodeCall})
		assert.Truef(t, strings.Contains(err.Error(), expectedDiscoveryErrMsg), "expected error due to discovery error")
	})

	t.Run("Peers Down", func(t *testing.T) {
		service, err := New(
			ctx, channelID,
			mocks.NewMockDiscoveryService(nil, peer1Org1, peer2Org1, peer2Org2, peer2Org3),
			WithRefreshInterval(500*time.Millisecond),
			WithResponseTimeout(2*time.Second),
		)
		require.NoError(t, err)
		defer service.Close()

		endorsers, err := service.GetEndorsersForChaincode([]*fab.ChaincodeCall{cc1ChaincodeCall})
		assert.NoError(t, err)
		assert.Equalf(t, 4, len(endorsers), "Expecting 4 endorser")
	})

	t.Run("Peer Filter", func(t *testing.T) {
		service, err := New(
			ctx, channelID,
			mocks.NewMockDiscoveryService(nil, peer1Org1, peer2Org1, peer2Org2, peer2Org3),
			WithRefreshInterval(500*time.Millisecond),
			WithResponseTimeout(2*time.Second),
		)
		require.NoError(t, err)
		defer service.Close()

		endorsers, err := service.GetEndorsersForChaincode([]*fab.ChaincodeCall{cc1ChaincodeCall},
			options.WithPeerFilter(func(peer fab.Peer) bool {
				return peer.(fab.PeerState).BlockHeight() > 1001
			}))
		assert.NoError(t, err)
		assert.Equalf(t, 3, len(endorsers), "Expecting 3 endorser")
	})
}

type config struct {
	fab.EndpointConfig
	peers []fab.ChannelPeer
}

func (c *config) ChannelPeers(name string) ([]fab.ChannelPeer, bool) {
	if len(c.peers) == 0 {
		return nil, false
	}
	return c.peers, true
}

func (c *config) PeerConfig(nameOrURL string) (*fab.PeerConfig, bool) {
	for _, peer := range c.peers {
		if peer.URL == nameOrURL {
			return &peer.NetworkPeer.PeerConfig, true
		}
	}
	return nil, false
}

func newPeerConfig(url, mspID string) fab.NetworkPeer {
	return fab.NetworkPeer{
		PeerConfig: fab.PeerConfig{
			URL: url,
		},
		MSPID: mspID,
	}
}
