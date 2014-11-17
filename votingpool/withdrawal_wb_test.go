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
	"sort"
	"testing"

	"github.com/conformal/btcnet"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
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

// Check that all outputs requested in a withdrawal match the outputs of the generated
// transaction(s).
func TestWithdrawalTxOutputs(t *testing.T) {
	teardown, mgr, pool := TstSetUp(t)
	store, storeTearDown := TstCreateTxStore(t)
	defer teardown()
	defer storeTearDown()

	// Create eligible inputs and the list of outputs we need to fulfil.
	eligible := TstCreateCredits(t, pool, []int64{2e6, 4e6}, store)
	outputs := []*OutputRequest{
		NewOutputRequest("foo", 1, "34eVkREKgvvGASZW7hkgE2uNc1yycntMK6", btcutil.Amount(3e6)),
		NewOutputRequest("foo", 2, "3PbExiaztsSYgh6zeMswC49hLUwhTQ86XG", btcutil.Amount(2e6)),
	}
	changeStart, err := pool.ChangeAddress(0, 0)
	if err != nil {
		t.Fatal(err)
	}

	w := newWithdrawal(0, outputs, eligible, changeStart, mgr.Net())
	if err := w.fulfilOutputs(); err != nil {
		t.Fatal(err)
	}

	if len(w.transactions) != 1 {
		t.Fatalf("Unexpected number of transactions; got %d, want 1", len(w.transactions))
	}

	tx := w.transactions[0]
	// The created tx should include both eligible credits, so we expect it to have
	// an input amount of 2e6+4e6 satoshis.
	inputAmount := eligible[0].Amount() + eligible[1].Amount()
	change := inputAmount - (outputs[0].amount + outputs[1].amount + calculateFee(tx))
	expectedOutputs := append(
		outputs, NewOutputRequest("foo", 3, changeStart.Addr().String(), change))
	checkTxOutputs(t, tx, expectedOutputs, mgr.Net())
}

// Check that withdrawal.status correctly states that no outputs were fulfilled when we
// don't have enough eligible credits for any of them.
func TestWithdrawalOutputsNoCreditsForAnyRequests(t *testing.T) {
	teardown, mgr, pool := TstSetUp(t)
	store, storeTearDown := TstCreateTxStore(t)
	defer teardown()
	defer storeTearDown()

	eligible := TstCreateCredits(t, pool, []int64{1e6}, store)
	outputs := []*OutputRequest{
		NewOutputRequest("foo", 1, "3Qt1EaKRD9g9FeL2DGkLLswhK1AKmmXFSe", btcutil.Amount(3e6))}
	changeStart, err := pool.ChangeAddress(0, 0)
	if err != nil {
		t.Fatal(err)
	}

	w := newWithdrawal(0, outputs, eligible, changeStart, mgr.Net())
	if err := w.fulfilOutputs(); err != nil {
		t.Fatal(err)
	}

	if len(w.transactions) != 0 {
		t.Fatalf("Unexpected number of transactions; got %d, want 0", len(w.transactions))
	}

	if len(w.status.outputs) != 1 {
		t.Fatalf("Unexpected number of outputs in WithdrawalStatus; got %d, want 1",
			len(w.status.outputs))
	}

	if w.status.outputs[0].status != "partial-" {
		t.Fatalf("Unexpected status for requested outputs; got '%s', want 'partial-'",
			w.status.outputs[0].status)
	}
}

// Check that some requested outputs are not fulfilled when we don't have credits for all
// of them.
func TestWithdrawalOutputsNotEnoughCreditsForAllRequests(t *testing.T) {
	teardown, mgr, pool := TstSetUp(t)
	store, storeTearDown := TstCreateTxStore(t)
	defer teardown()
	defer storeTearDown()

	// Create eligible inputs and the list of outputs we need to fulfil.
	eligible := TstCreateCredits(t, pool, []int64{2e6, 4e6}, store)
	out1 := NewOutputRequest("foo", 1, "34eVkREKgvvGASZW7hkgE2uNc1yycntMK6", btcutil.Amount(3e6))
	out2 := NewOutputRequest("foo", 2, "3PbExiaztsSYgh6zeMswC49hLUwhTQ86XG", btcutil.Amount(2e6))
	out3 := NewOutputRequest("foo", 3, "3Qt1EaKRD9g9FeL2DGkLLswhK1AKmmXFSe", btcutil.Amount(5e6))
	outputs := []*OutputRequest{out1, out2, out3}
	changeStart, err := pool.ChangeAddress(0, 0)
	if err != nil {
		t.Fatal(err)
	}

	w := newWithdrawal(0, outputs, eligible, changeStart, mgr.Net())
	if err := w.fulfilOutputs(); err != nil {
		t.Fatal(err)
	}

	tx := w.transactions[0]
	// The created tx should spend both eligible credits, so we expect it to have
	// an input amount of 2e6+4e6 satoshis.
	inputAmount := eligible[0].Amount() + eligible[1].Amount()
	// We expect it to include outputs for requests 1 and 2, plus a change output, but
	// output request #3 should not be there because we don't have enough credits.
	change := inputAmount - (out1.amount + out2.amount + calculateFee(tx))
	expectedOutputs := []*OutputRequest{out1, out2}
	sort.Sort(byOutBailmentID(expectedOutputs))
	expectedOutputs = append(
		expectedOutputs, NewOutputRequest("foo", 4, changeStart.Addr().String(), change))
	checkTxOutputs(t, tx, expectedOutputs, mgr.Net())

	// withdrawal.status should state that outputs 1 and 2 were successfully fulfilled,
	// and that output 3 was not.
	expectedStatuses := map[*OutputRequest]string{
		out1: "success", out2: "success", out3: "partial-"}
	for _, wOutput := range w.status.outputs {
		if wOutput.status != expectedStatuses[wOutput.request] {
			t.Fatalf("Unexpected status for %v; got '%s', want 'partial-'", wOutput.request,
				wOutput.status)
		}
	}
}

func TestWithdrawalOutputsNoChange(t *testing.T) {
	// TODO:
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

// pkScriptAddr parses the given pkScript and returns the address associated with it.
func pkScriptAddr(t *testing.T, pkScript []byte, net *btcnet.Params) string {
	_, addresses, _, err := btcscript.ExtractPkScriptAddrs(pkScript, net)
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 1 {
		t.Fatalf("Unexpected number of addresses in pkScript; got %d, want 1", len(addresses))
	}
	return addresses[0].String()
}

// checkTxOutputs checks that the outputs in the given tx match the given slice of
// OutputRequests.
func checkTxOutputs(t *testing.T, tx *btcwire.MsgTx, expectedOutputs []*OutputRequest, net *btcnet.Params) {
	nOutputs := len(expectedOutputs)
	if len(tx.TxOut) != nOutputs {
		t.Fatalf("Unexpected number of tx outputs; got %d, want %d", len(tx.TxOut), nOutputs)
	}

	for i, output := range expectedOutputs {
		txOut := tx.TxOut[i]
		gotAddr := pkScriptAddr(t, txOut.PkScript, net)
		if gotAddr != output.address {
			t.Fatalf(
				"Unexpected address for output %d; got %s, want %s", i, gotAddr, output.address)
		}
		gotAmount := btcutil.Amount(txOut.Value)
		if gotAmount != output.amount {
			t.Fatalf(
				"Unexpected amount for output %d; got %v, want %v", i, gotAmount, output.amount)
		}
	}
}
