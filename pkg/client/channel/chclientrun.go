// +build !pprof

/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package channel

import (
	"github.com/blockchain/fabric-sdk-go/pkg/client/channel/invoke"
	"github.com/blockchain/fabric-sdk-go/pkg/client/common/discovery/greylist"
	"github.com/blockchain/fabric-sdk-go/pkg/common/providers/context"
	"github.com/blockchain/fabric-sdk-go/pkg/common/providers/fab"
)

type clientTally interface{} // nolint

func newClient(channelContext context.Channel, membership fab.ChannelMembership, eventService fab.EventService, greylistProvider *greylist.Filter) Client {
	channelClient := Client{
		membership:   membership,
		eventService: eventService,
		greylist:     greylistProvider,
		context:      channelContext,
	}
	return channelClient
}

func callQuery(cc *Client, request Request, options ...RequestOption) (Response, error) {
	return cc.InvokeHandler(invoke.NewQueryHandler(), request, options...)
}

func callExecute(cc *Client, request Request, options ...RequestOption) (Response, error) {
	return cc.InvokeHandler(invoke.NewExecuteHandler(), request, options...)
}
