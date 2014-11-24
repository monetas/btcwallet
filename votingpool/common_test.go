/*
 * Copyright (c) 2014 Conformal Systems LLC <info@conformal.com>
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

package votingpool

import (
	"runtime"
	"testing"

	"github.com/conformal/btcutil"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

// TstCheckError ensures the passed error is a votingpool.Error with an error
// code that matches the passed error code.
func TstCheckError(t *testing.T, testName string, gotErr error, wantErrCode ErrorCode) {
	vpErr, ok := gotErr.(Error)
	if !ok {
		t.Errorf("%s: unexpected error type - got %T (%s), want %T",
			testName, gotErr, gotErr, Error{})
	}
	if vpErr.ErrorCode != wantErrCode {
		t.Errorf("%s: unexpected error code - got %s (%s), want %s",
			testName, vpErr.ErrorCode, vpErr, wantErrCode)
	}
}

func TstUnlockManager(t *testing.T, mgr *waddrmgr.Manager) {
	if err := mgr.Unlock(privPassphrase); err != nil {
		t.Fatal(err)
	}
}

// TstFakeCredit is a structure implementing the CreditInterface used
// for testing purposes.
//
// XXX(lars): we should maybe change all the value receivers to
// pointer receivers so we do not mix. That would mean we'd have to
// change the CreditInterface and implementations as well.
type TstFakeCredit struct {
	addr        WithdrawalAddress
	txid        *btcwire.ShaHash
	outputIndex uint32
	amount      btcutil.Amount
}

func (c TstFakeCredit) String() string {
	return ""
}

func (c TstFakeCredit) TxSha() *btcwire.ShaHash {
	return c.txid
}

func (c TstFakeCredit) OutputIndex() uint32 {
	return c.outputIndex
}

func (c TstFakeCredit) Address() WithdrawalAddress {
	return c.addr
}

func (c TstFakeCredit) Amount() btcutil.Amount {
	return c.amount
}

func (c TstFakeCredit) TxOut() *btcwire.TxOut {
	return nil
}

func (c TstFakeCredit) OutPoint() *btcwire.OutPoint {
	return &btcwire.OutPoint{Hash: *c.txid, Index: c.outputIndex}
}

func (c *TstFakeCredit) SetAmount(amount btcutil.Amount) *TstFakeCredit {
	c.amount = amount
	return c
}

func TstNewFakeCredit(t *testing.T, pool *Pool, series, index Index, branch Branch, txid []byte, outputIdx int) TstFakeCredit {
	var hash btcwire.ShaHash
	copy(hash[:], txid)
	addr, err := pool.WithdrawalAddress(uint32(series), branch, index)
	if err != nil {
		t.Fatalf("WithdrawalAddress failed: %v", err)
	}
	return TstFakeCredit{
		addr:        *addr,
		txid:        &hash,
		outputIndex: uint32(outputIdx),
	}
}

// Compile time check that TstFakeCredit implements the
// CreditInterface.
var _ CreditInterface = (*TstFakeCredit)(nil)
