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
	"flag"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cirocosta/go-monero/pkg/rpc/daemon"
)

const Version string = "0.1.2"

// Monero network settings
const BlockTime int = 120

var listenAddress, ProxyToUse, DaemonUrl, RpcLogin string

var RpcPassword, RpcUsername string

func main() {
	fmt.Println("\x1b[1m" + `
         __          ___  __    __    __        __     __
|\  /|  /  \  |\  | |    |  \  /  \  |  \ |    /  \   /   | /
| \/ | |    | | \ | |___ |__/ |    | |__/ |   |    | |    |/
|    | |    | |  \| |    | \  |    | |  \ |   |    | |    |\
|    |  \__/  |   | |___ |  \  \__/  |__/ |__  \__/   \__ | \

                 The Trustless Block Explorer` + "\x1b[0m")
	fmt.Println("\n\nStarting MoneroBlock v" + Version)

	flag.StringVar(&listenAddress, "bind", "127.0.0.1:31312", "Address and port to bind.")
	flag.StringVar(&ProxyToUse, "proxy", "none", "Proxy to use. Should start with socks5://, socks4:// or http:// .")
	flag.StringVar(&RpcLogin, "rpc-login", "none", "Required if daemon has login enabled.")
	flag.StringVar(&DaemonUrl, "daemon", "127.0.0.1:18081", "The Monero daemon URL. Please note that using a third-party daemon might harm privacy if you do not use a proxy.")
	flag.Parse()

	if RpcLogin != "none" {
		splLogin := strings.Split(RpcLogin, ":")
		if len(splLogin) < 2 {
			fmt.Println("rpc-login flag is not valid. Ignoring it.")
			RpcLogin = "none"
		} else {
			RpcPassword = splLogin[0]
			RpcUsername = splLogin[1]
		}
	}

	if !strings.HasPrefix(DaemonUrl, "http://") {
		DaemonUrl = "http://" + DaemonUrl
	}

	InitPages()
	StartDaemonRpc()

	http.HandleFunc("/style.css", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/css")
		res.Write(StyleSheetPage)

	})
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		var blocksList string
		for _, e := range RecentBlocks {
			CheckString(e.Hash)
			blocksList = "<tr class=\"tr\"><td>" + strconv.FormatUint(e.Height, 10) + "</td><td>" + strconv.FormatUint(uint64(e.NumTxes), 10) + "</td><td><a class=\"monos\" href=\"/block?id=" + e.Hash + "\">" + e.Hash + "</a></td><td>" + FormatTimeAgo(time.Now().Unix()-e.Timestamp) + "</td></tr>" + blocksList
		}

		info := GetInfo()
		pageOut := strings.Replace(MainPage, "$blocks", blocksList, 1)

		pageOut = strings.Replace(pageOut, "$diff", strconv.FormatUint(info.Difficulty/1000/1000, 10)+" M", 1)
		pageOut = strings.Replace(pageOut, "$txnr", strconv.FormatUint(info.TxCount/1000, 10)+"k", 1)
		pageOut = strings.Replace(pageOut, "$hashrate", strconv.FormatUint(info.Difficulty/1000/1000/uint64(BlockTime), 10)+" MH/s", 1)

		res.Header().Set("Content-Type", "text/html")
		res.Write([]byte(pageOut))
	})
	http.HandleFunc("/search", func(res http.ResponseWriter, req *http.Request) {
		sParams := req.URL.Query()["q"]
		if len(sParams) == 0 {
			res.Write([]byte("ERROR: Missing search query"))
			return
		}
		searchParam := sParams[0]
		searchType := GetSearchType(searchParam)

		if !CheckString2(searchParam) {
			res.Write([]byte("Transaction or block not found"))
			return
		}

		if searchType == 2 {
			res.Write([]byte("Transaction or block not found"))
		} else if searchType == 0 {
			res.Write(RedirectToUrl("/tx?id=" + searchParam))
		} else if searchType == 1 {
			res.Write(RedirectToUrl("/block?id=" + searchParam))
		}
		res.Header().Set("Content-Type", "text/html")
	})
	http.HandleFunc("/tx", func(res http.ResponseWriter, req *http.Request) {
		sParams := req.URL.Query()["id"]
		if len(sParams) == 0 {
			res.Write([]byte("ERROR: Missing transaction ID"))
			return
		}
		txid := sParams[0]
		if !CheckString2(txid) {
			res.Write([]byte("Transaction not found"))
			return
		}

		exists, txData := GetTransaction(txid)
		if !exists {
			res.Write([]byte("ERROR: Transaction not found"))
			return
		}

		confirmations := strconv.FormatUint(GetHeight()-txData.BlockHeight, 10)
		if txData.BlockHeight == 0 {
			confirmations = "0"
		}
		CheckString(txData.TxHash)

		o := strings.Replace(TxPage, "$confirmations", confirmations, 1)
		height := strconv.FormatUint(txData.BlockHeight, 10)
		o = strings.Replace(o, "$height", height, 2)
		txSize := strconv.FormatFloat(float64(len(txData.AsHex))/2/100, 'f', 2, 64)
		o = strings.Replace(o, "$size", txSize, 1)
		o = strings.Replace(o, "$txid", txData.TxHash, 1)

		o = strings.Replace(o, "$doublespend", strconv.FormatBool(txData.DoubleSpendSeen), 1)
		formattedTime := time.Unix(txData.BlockTimestamp, 0).Format("2006-01-02 15:04")
		o = strings.Replace(o, "$timestamp", formattedTime, 1)

		res.Write([]byte(o))
	})
	http.HandleFunc("/block", func(res http.ResponseWriter, req *http.Request) {
		sParams := req.URL.Query()["id"]
		if len(sParams) == 0 {
			res.Write([]byte("ERROR: Missing block ID"))
			return
		}
		blockId := sParams[0]
		isNumber, _ := regexp.Match(`^\d+$`, []byte(blockId))

		var params daemon.GetBlockRequestParameters
		if isNumber {
			blockNum, _ := strconv.ParseUint(blockId, 10, 64)
			params = daemon.GetBlockRequestParameters{Height: blockNum}

		} else {
			params = daemon.GetBlockRequestParameters{Hash: blockId}
		}
		if !CheckString2(blockId) {
			res.Write([]byte("Block not found"))
			return
		}

		var blockD *daemon.GetBlockResult
		exists, blockD, blockBody := GetBlock(params)
		blockData := blockD.BlockHeader

		if !exists {
			res.Write([]byte("ERROR: Block not found"))
			return
		}
		CheckString(blockData.Hash)
		CheckString(blockD.MinerTxHash)
		transactions := append(blockBody.TxHashes, blockD.MinerTxHash)
		o := strings.Replace(BlockPage, "$blocknum", strconv.FormatUint(blockData.Height, 10), 1)
		o = strings.Replace(o, "$hash", blockData.Hash, 1)
		o = strings.Replace(o, "$numtxes", strconv.FormatUint(uint64(blockData.NumTxes), 10), 1)
		o = strings.Replace(o, "$size", strconv.FormatFloat(float64(blockData.BlockSize)/1000, 'f', 1, 64), 1)
		o = strings.Replace(o, "$timestamp", time.Unix(blockData.Timestamp, 0).Format("2006-01-02 15:04"), 1)
		o = strings.Replace(o, "$diff", strconv.FormatUint(blockData.Difficulty/1000/1000, 10), 1)
		o = strings.Replace(o, "$reward", strconv.FormatFloat(float64(blockData.Reward)/math.Pow(10, 12), 'f', 5, 64), 1)
		o = strings.Replace(o, "$minerTx", blockD.MinerTxHash, 2)

		var txList string = ""
		for i, e := range transactions {
			CheckString(e)
			txList += `<tr class="tr">
			<td>` + strconv.FormatInt(int64(i+1), 10) + `</td>
			<td><a href="/tx?id=` + e + `" class="monob">` + e + `</a></td>
			</tr>`
		}
		o = strings.Replace(o, "$txList", txList, 1)

		res.Write([]byte(o))
	})

	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  80 * time.Second,
		Addr:         listenAddress,
	}
	paddedAddress := listenAddress + "/"
	for {
		if len(paddedAddress) < 16 {
			paddedAddress += " "
		} else {
			break
		}
	}
	fmt.Println("Server listening at http://" + listenAddress + "/")
	srv.ListenAndServe()

}

func RedirectToUrl(url string) []byte {
	return []byte("<!DOCTYPE HTML><body><meta http-equiv=\"refresh\" content=\"0;url='" + url + "'\"/></body>")
}

func FormatTimeAgo(t int64) string {
	if t > 60*60*24*365*2 {
		return strconv.FormatInt(t/60/60/24/365, 10) + " y"

	}
	if t > 60*60*24 {
		return strconv.FormatInt(t/60/60/24, 10) + " d"
	} else if t > 60*60 {
		return strconv.FormatInt(t/60/60, 10) + " h"
	} else if t > 60 {
		return strconv.FormatInt(t/60, 10) + " min"
	} else {
		return strconv.FormatInt(t, 10) + " sec"
	}
}

func CheckString(s string) {
	if strings.Contains(s, "<") || strings.Contains(s, ">") || strings.Contains(s, "&") {
		panic("Daemon sent unsafe data (containing characters '<' '>' '&'). It should not be trusted.")
	}
}
func CheckString2(s string) bool {
	if strings.Contains(s, "<") || strings.Contains(s, ">") || strings.Contains(s, "&") {
		return false
	}
	return true
}
