/*
 * Copyright (c) 2013, 2014 Conformal Systems LLC <info@conformal.com>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

// This file implements the notification handlers for btcd-side notifications.

package main

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/monetas/btcjson"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
	"github.com/monetas/btcwallet/txstore"
	"github.com/monetas/btcwallet/wallet"
	"github.com/conformal/btcwire"
	"github.com/monetas/btcws"
)

func parseBlock(block *btcws.BlockDetails) (*txstore.Block, int, error) {
	if block == nil {
		return nil, btcutil.TxIndexUnknown, nil
	}
	blksha, err := btcwire.NewShaHashFromStr(block.Hash)
	if err != nil {
		return nil, btcutil.TxIndexUnknown, err
	}
	b := &txstore.Block{
		Height: block.Height,
		Hash:   *blksha,
		Time:   time.Unix(block.Time, 0),
	}
	return b, block.Index, nil
}

type notificationHandler func(btcjson.Cmd) error

var notificationHandlers = map[string]notificationHandler{
	btcws.BlockConnectedNtfnMethod:    NtfnBlockConnected,
	btcws.BlockDisconnectedNtfnMethod: NtfnBlockDisconnected,
	btcws.RecvTxNtfnMethod:            NtfnRecvTx,
	btcws.RedeemingTxNtfnMethod:       NtfnRedeemingTx,
	btcws.RescanProgressNtfnMethod:    NtfnRescanProgress,
}

// NtfnRecvTx handles the btcws.RecvTxNtfn notification.
func NtfnRecvTx(n btcjson.Cmd) error {
	rtx, ok := n.(*btcws.RecvTxNtfn)
	if !ok {
		return fmt.Errorf("%v handler: unexpected type", n.Method())
	}

	bs, err := GetCurBlock()
	if err != nil {
		return fmt.Errorf("%v handler: cannot get current block: %v", n.Method(), err)
	}

	rawTx, err := hex.DecodeString(rtx.HexTx)
	if err != nil {
		return fmt.Errorf("%v handler: bad hexstring: %v", n.Method(), err)
	}
	tx, err := btcutil.NewTxFromBytes(rawTx)
	if err != nil {
		return fmt.Errorf("%v handler: bad transaction bytes: %v", n.Method(), err)
	}

	block, txIdx, err := parseBlock(rtx.Block)
	if err != nil {
		return fmt.Errorf("%v handler: bad block: %v", n.Method(), err)
	}
	tx.SetIndex(txIdx)

	// For transactions originating from this wallet, the sent tx history should
	// be recorded before the received history.  If wallet created this tx, wait
	// for the sent history to finish being recorded before continuing.
	//
	// TODO(jrick) this is wrong due to tx malleability.  Cannot safely use the
	// txsha as an identifier.
	req := SendTxHistSyncRequest{
		txsha:    *tx.Sha(),
		response: make(chan SendTxHistSyncResponse),
	}
	SendTxHistSyncChans.access <- req
	resp := <-req.response
	if resp.ok {
		// Wait until send history has been recorded.
		<-resp.c
		SendTxHistSyncChans.remove <- *tx.Sha()
	}

	// For every output, find all accounts handling that output address (if any)
	// and record the received txout.
	for outIdx, txout := range tx.MsgTx().TxOut {
		var accounts []*Account
		// Errors don't matter here.  If addrs is nil, the range below
		// does nothing.
		_, addrs, _, _ := btcscript.ExtractPkScriptAddrs(txout.PkScript,
			activeNet.Params)
		for _, addr := range addrs {
			a, err := AcctMgr.AccountByAddress(addr)
			if err != nil {
				continue
			}
			accounts = append(accounts, a)
		}

		for _, a := range accounts {
			txr, err := a.TxStore.InsertTx(tx, block)
			if err != nil {
				return err
			}
			cred, err := txr.AddCredit(uint32(outIdx), false)
			if err != nil {
				return err
			}
			AcctMgr.ds.ScheduleTxStoreWrite(a)

			// Notify frontends of tx.  If the tx is unconfirmed, it is always
			// notified and the outpoint is marked as notified.  If the outpoint
			// has already been notified and is now in a block, a txmined notifiction
			// should be sent once to let frontends that all previous send/recvs
			// for this unconfirmed tx are now confirmed.
			op := *cred.OutPoint()
			previouslyNotifiedReq := NotifiedRecvTxRequest{
				op:       op,
				response: make(chan NotifiedRecvTxResponse),
			}
			NotifiedRecvTxChans.access <- previouslyNotifiedReq
			if <-previouslyNotifiedReq.response {
				NotifiedRecvTxChans.remove <- op
			} else {
				// Notify frontends of new recv tx and mark as notified.
				NotifiedRecvTxChans.add <- op

				ltr, err := cred.ToJSON(a.Name(), bs.Height, a.Wallet.Net())
				if err != nil {
					return err
				}
				NotifyNewTxDetails(allClients, a.Name(), ltr)
			}

			// Notify frontends of new account balance.
			confirmed := a.CalculateBalance(1)
			unconfirmed := a.CalculateBalance(0) - confirmed
			NotifyWalletBalance(allClients, a.name, confirmed)
			NotifyWalletBalanceUnconfirmed(allClients, a.name, unconfirmed)
		}
	}

	return nil
}

// NtfnBlockConnected handles btcd notifications resulting from newly
// connected blocks to the main blockchain.
//
// TODO(jrick): Send block time with notification.  This will be used
// to mark wallet files with a possibly-better earliest block height,
// and will greatly reduce rescan times for wallets created with an
// out of sync btcd.
func NtfnBlockConnected(n btcjson.Cmd) error {
	bcn, ok := n.(*btcws.BlockConnectedNtfn)
	if !ok {
		return fmt.Errorf("%v handler: unexpected type", n.Method())
	}
	hash, err := btcwire.NewShaHashFromStr(bcn.Hash)
	if err != nil {
		return fmt.Errorf("%v handler: invalid hash string", n.Method())
	}

	// Update the blockstamp for the newly-connected block.
	bs := &wallet.BlockStamp{
		Height: bcn.Height,
		Hash:   *hash,
	}
	curBlock.Lock()
	curBlock.BlockStamp = *bs
	curBlock.Unlock()

	// btcd notifies btcwallet about transactions first, and then sends
	// the new block notification.  New balance notifications for txs
	// in blocks are therefore sent here after all tx notifications
	// have arrived and finished being processed by the handlers.
	workers := NotifyBalanceRequest{
		block: *hash,
		wg:    make(chan *sync.WaitGroup),
	}
	NotifyBalanceSyncerChans.access <- workers
	if wg := <-workers.wg; wg != nil {
		wg.Wait()
		NotifyBalanceSyncerChans.remove <- *hash
	}
	AcctMgr.BlockNotify(bs)

	// Pass notification to frontends too.
	marshaled, err := n.MarshalJSON()
	// The parsed notification is expected to be marshalable.
	if err != nil {
		panic(err)
	}
	allClients <- marshaled

	return nil
}

// NtfnBlockDisconnected handles btcd notifications resulting from
// blocks disconnected from the main chain in the event of a chain
// switch and notifies frontends of the new blockchain height.
func NtfnBlockDisconnected(n btcjson.Cmd) error {
	bdn, ok := n.(*btcws.BlockDisconnectedNtfn)
	if !ok {
		return fmt.Errorf("%v handler: unexpected type", n.Method())
	}
	hash, err := btcwire.NewShaHashFromStr(bdn.Hash)
	if err != nil {
		return fmt.Errorf("%v handler: invalid hash string", n.Method())
	}

	// Rollback Utxo and Tx data stores.
	if err = AcctMgr.Rollback(bdn.Height, hash); err != nil {
		return err
	}

	// Pass notification to frontends too.
	marshaled, err := n.MarshalJSON()
	// A btcws.BlockDisconnectedNtfn is expected to marshal without error.
	// If it does, it indicates that one of its struct fields is of a
	// non-marshalable type.
	if err != nil {
		panic(err)
	}
	allClients <- marshaled

	return nil
}

// NtfnRedeemingTx handles btcd redeemingtx notifications resulting from a
// transaction spending a watched outpoint.
func NtfnRedeemingTx(n btcjson.Cmd) error {
	cn, ok := n.(*btcws.RedeemingTxNtfn)
	if !ok {
		return fmt.Errorf("%v handler: unexpected type", n.Method())
	}

	rawTx, err := hex.DecodeString(cn.HexTx)
	if err != nil {
		return fmt.Errorf("%v handler: bad hexstring: %v", n.Method(), err)
	}
	tx, err := btcutil.NewTxFromBytes(rawTx)
	if err != nil {
		return fmt.Errorf("%v handler: bad transaction bytes: %v", n.Method(), err)
	}

	block, txIdx, err := parseBlock(cn.Block)
	if err != nil {
		return fmt.Errorf("%v handler: bad block: %v", n.Method(), err)
	}
	tx.SetIndex(txIdx)
	return AcctMgr.RecordSpendingTx(tx, block)
}

// NtfnRescanProgress handles btcd rescanprogress notifications resulting
// from a partially completed rescan.
func NtfnRescanProgress(n btcjson.Cmd) error {
	cn, ok := n.(*btcws.RescanProgressNtfn)
	if !ok {
		return fmt.Errorf("%v handler: unexpected type", n.Method())
	}

	// Notify the rescan manager of the completed partial progress for
	// the current rescan.
	AcctMgr.rm.MarkProgress(cn.LastProcessed)

	return nil
}
