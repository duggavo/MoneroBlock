// Copyright (c) 2022, duggavo
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without modification, are
// permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this list of
//    conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice, this list
//    of conditions and the following disclaimer in the documentation and/or other
//    materials provided with the distribution.
//
// 3. Neither the name of the copyright holder nor the names of its contributors may be
//    used to endorse or promote products derived from this software without specific
//    prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
// THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"regexp"

	"github.com/cirocosta/go-monero/pkg/rpc"
	"github.com/cirocosta/go-monero/pkg/rpc/daemon"
)

var client *daemon.Client
var bgCtx = context.Background()

func StartDaemonRpc() {
	var rpc_client *rpc.Client
	var err error

	if ProxyToUse == "none" {
		rpc_client, err = rpc.NewClient(DaemonUrl)

	} else {
		url_proxy, _ := url.Parse(ProxyToUse)

		transport := http.Transport{}
		transport.Proxy = http.ProxyURL(url_proxy)
		http_proxy_client := &http.Client{Transport: &transport}

		rpc_client, err = rpc.NewClient(DaemonUrl, rpc.WithHTTPClient(http_proxy_client))

	}

	if err != nil {
		panic(err)
	}
	client = daemon.NewClient(rpc_client)
	go UpdateCache()
}

func GetHeight() uint64 {
	height, err := client.GetHeight(bgCtx)
	if err != nil {
		panic(err)
	}
	return height.Height
}

func GetInfo() *daemon.GetInfoResult {
	info, err := client.GetInfo(bgCtx)
	if err != nil {
		panic(err)
	}
	return info
}

func GetRecentBlocks() *daemon.GetBlockHeadersRangeResult {
	height := GetHeight()
	r, err := client.GetBlockHeadersRange(bgCtx, height-51, height-1)
	if err != nil {
		panic(err)
	}
	return r
}

func GetBlockByHeight(height uint64) (isValid bool, result daemon.BlockHeader) {
	r, err := client.GetBlockHeaderByHeight(bgCtx, height)
	if err != nil {
		panic(err)
	}
	if r.Status != "OK" {
		return false, daemon.BlockHeader{}
	} else {
		return true, r.BlockHeader
	}
}

func GetBlockByHash(hash string) (isValid bool, result daemon.BlockHeader) {
	r, err := client.GetBlockHeaderByHash(bgCtx, []string{hash})
	if err != nil {
		panic(err)
	}
	if r.Status != "OK" {
		return false, daemon.BlockHeader{}
	} else {
		return true, r.BlockHeaders[0]
	}
}
func GetBlock(params daemon.GetBlockRequestParameters) (isValid bool, result *daemon.GetBlockResult, ParsedJSON daemon.GetBlockResultJSON) {
	r, err := client.GetBlock(bgCtx, params)
	if err != nil {
		panic(err)
	}

	if r.Status != "OK" {
		return false, &daemon.GetBlockResult{}, daemon.GetBlockResultJSON{}
	} else {
		var ParsedJSON daemon.GetBlockResultJSON
		json.Unmarshal([]byte(r.JSON), &ParsedJSON)

		return true, r, ParsedJSON
	}
}

func GetTransaction(txHash string) (isValid bool, result daemon.GetTransactionsResultTransaction) {
	r, err := client.GetTransactions(bgCtx, []string{txHash})
	if err != nil {
		panic(err)
	}
	if len(r.Txs) == 0 {
		return false, daemon.GetTransactionsResultTransaction{}
	}
	return true, r.Txs[0]
}

var RecentBlocks []daemon.BlockHeader

func UpdateCache() {
	for {
		RecentBlocks = GetRecentBlocks().Headers
		time.Sleep(5 * time.Second)
	}
}

func GetSearchType(query string) int {
	isHex, _ := regexp.Match(`^[0-9a-f]{64}$`, []byte(query))
	isNumber, _ := regexp.Match(`^\d+$`, []byte(query))
	if isHex {

		isValidTx, _ := GetTransaction(query)
		if isValidTx {
			return 0
		} else {
			return 1
		}
	} else if isNumber {
		return 1
	} else {
		return 2
	}
}
