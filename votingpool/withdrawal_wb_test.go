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
	"testing"

	"github.com/conformal/btcutil/hdkeychain"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

func TestTxInSigning(t *testing.T) {
	tearDown, mgr, pool := TstSetUp(t)
	store, storeTearDown := TstCreateTxStore(t)
	defer tearDown()
	defer storeTearDown()

	// Create two Credits and add them as inputs to a new transaction.
	tx := &btcwire.MsgTx{}
	credits := TstCreateCredits(t, pool, []int64{5e6, 4e6}, store)
	for _, c := range credits {
		tx.AddTxIn(btcwire.NewTxIn(c.OutPoint(), nil))
	}
	ntxid := Ntxid(tx)

	// Get the raw signatures for each of the inputs in our transaction.
	sigs, err := getRawSigs([]*btcwire.MsgTx{tx}, map[string][]CreditInterface{ntxid: credits})
	if err != nil {
		t.Fatal(err)
	}

	checkRawSigs(t, sigs[ntxid], credits)

	signTxAndValidate(t, mgr, tx, sigs[ntxid], credits)
}

func TestWithdrawalOutputs(t *testing.T) {
	// TODO: Check that the outputs in the constructed TX match the requested ones.
}

func TestSignMultiSigUTXOInvalidPkScript(t *testing.T) {
	// TODO:
}

func TestSignMultiSigUTXORedeemScriptNotFound(t *testing.T) {
	// TODO:
}

func TestSignMultiSigUTXORedeemScriptNotMultiSig(t *testing.T) {
	// TODO:
}

func TestSignMultiSigUTXONotEnoughSigs(t *testing.T) {
	// TODO:
}

// signTxAndValidate will construct the signature script for each input of the given
// transaction (using the given raw signatures and the pkScripts from credits) and execute
// those scripts to validate them.
func signTxAndValidate(t *testing.T, mgr *waddrmgr.Manager, tx *btcwire.MsgTx, txSigs TxSigs, credits []CreditInterface) {
	TstUnlockManager(t, mgr)
	defer mgr.Lock()
	pkScripts := make([][]byte, len(tx.TxIn))
	for i := range tx.TxIn {
		pkScript := credits[i].TxOut().PkScript
		err := SignMultiSigUTXO(mgr, tx, i, pkScript, txSigs[i], mgr.Net())
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := ValidateSigScripts(tx, pkScripts); err != nil {
		t.Fatal(err)
	}
}

// checkRawSigs checks that every signature list in txSigs has one signature for every
// private key loaded in our series.
func checkRawSigs(t *testing.T, txSigs TxSigs, credits []CreditInterface) {
	if len(txSigs) != len(credits) {
		t.Fatalf("Unexpected number of sig lists; got %d, want %d", len(txSigs), len(credits))
	}

	for i, txInSigs := range txSigs {
		series := credits[i].Address().Series()
		keysCount := countNonNil(series.privateKeys)
		if len(txInSigs) != keysCount {
			t.Fatalf("Unexpected number of sigs for input %d; got %d, want %d",
				i, len(txInSigs), keysCount)
		}
	}
}

func countNonNil(keys []*hdkeychain.ExtendedKey) int {
	count := 0
	for _, key := range keys {
		if key != nil {
			count++
		}
	}
	return count
}
