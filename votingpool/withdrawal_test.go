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

package votingpool_test

import (
	"bytes"
	"testing"

	"github.com/conformal/btcutil"
	"github.com/conformal/btcutil/hdkeychain"
	vp "github.com/conformal/btcwallet/votingpool"
	"github.com/conformal/btcwire"
)

// XXX: This test could benefit from being split into smaller ones, but that won't be a
// trivial endeavour. Or maybe it should be turned into an example and all the checks here
// moved into separate, whitebox tests.
func TestWithdrawal(t *testing.T) {
	tearDown, pool, store := vp.TstCreatePoolAndTxStore(t)
	defer tearDown()

	masters := []*hdkeychain.ExtendedKey{
		vp.TstCreateMasterKey(t, bytes.Repeat([]byte{0x00, 0x01}, 16)),
		vp.TstCreateMasterKey(t, bytes.Repeat([]byte{0x02, 0x01}, 16)),
		vp.TstCreateMasterKey(t, bytes.Repeat([]byte{0x03, 0x01}, 16))}
	def := vp.TstCreateSeriesDef(t, 2, masters)
	vp.TstCreateSeries(t, pool, []vp.TstSeriesDef{def})
	// Create eligible inputs and the list of outputs we need to fulfil.
	eligible := vp.TstCreateCreditsOnSeries(t, pool, def.SeriesID, []int64{5e6, 4e6}, store)
	address1 := "34eVkREKgvvGASZW7hkgE2uNc1yycntMK6"
	address2 := "3PbExiaztsSYgh6zeMswC49hLUwhTQ86XG"
	outputs := []*vp.OutputRequest{
		vp.NewOutputRequest("foo", 1, address1, btcutil.Amount(4e6)),
		vp.NewOutputRequest("foo", 2, address2, btcutil.Amount(1e6)),
	}
	changeStart, err := pool.ChangeAddress(def.SeriesID, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Withdrawal() should fulfil the desired outputs spending from the given inputs.
	status, sigs, err := pool.Withdrawal(0, outputs, eligible, changeStart, store)
	if err != nil {
		t.Fatal(err)
	}

	// Check that all outputs were successfully fulfiled.
	checkWithdrawalOutputs(t, status, map[string]btcutil.Amount{address1: 4e6, address2: 1e6})

	// XXX: The ntxid is deterministic so we hardcode it here, but if the test is changed
	// in a way that causes the generated transactions to change (e.g. different
	// inputs/outputs), the ntxid will change too.
	ntxid := "dce3f6bca2288fc5f797c5b40b19cc57a064201f4074e424954a84648caf0ef0"
	txSigs := sigs[ntxid]

	// Finally we use SignMultiSigUTXO() to construct the SignatureScripts (using the raw
	// signatures).
	// Must unlock the manager first as signing involves looking up the redeem script,
	// which is stored encrypted.
	mgr := pool.Manager()
	vp.TstUnlockManager(t, mgr)
	sha, _ := btcwire.NewShaHashFromStr(ntxid)
	tx := store.UnminedTx(sha).MsgTx()
	pkScripts := make([][]byte, len(tx.TxIn))
	for i, txIn := range tx.TxIn {
		txOut, err := store.UnconfirmedSpent(txIn.PreviousOutPoint)
		if err != nil {
			t.Fatal(err)
		}
		pkScripts[i] = txOut.PkScript
		err = vp.SignMultiSigUTXO(mgr, tx, i, txOut.PkScript, txSigs[i], mgr.Net())
		if err != nil {
			t.Fatal(err)
		}
	}

	// Before we broadcast this transaction, let's make sure it's properly signed.
	if err = vp.ValidateSigScripts(tx, pkScripts); err != nil {
		t.Fatal(err)
	}
}

func checkWithdrawalOutputs(
	t *testing.T, wStatus *vp.WithdrawalStatus, amounts map[string]btcutil.Amount) {
	fulfiled := wStatus.Outputs()
	if len(fulfiled) != 2 {
		t.Fatalf("Unexpected number of outputs in WithdrawalStatus; got %d, want %d",
			len(fulfiled), 2)
	}
	for i, output := range fulfiled {
		addr := output.Address()
		amount, ok := amounts[addr]
		if !ok {
			t.Fatalf("Unexpected output addr: %s", addr)
		}

		if output.Status() != "success" {
			t.Fatalf(
				"Unexpected status for output %d; got '%s', want 'success'", i, output.Status())
		}

		outpoints := output.Outpoints()
		if len(outpoints) != 1 {
			t.Fatalf(
				"Unexpected number of outpoints for output %d; got %d, want 1", i, len(outpoints))
		}

		gotAmount := outpoints[0].Amount()
		if gotAmount != amount {
			t.Fatalf("Unexpected amount for output %d; got %v, want %v", i, gotAmount, amount)
		}
	}
}
