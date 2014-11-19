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
	"bytes"
	"sort"
	"testing"

	"github.com/conformal/btcnet"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcutil/hdkeychain"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

func TestStoreTransactionsWithoutChangeOutput(t *testing.T) {
	tearDown, _, pool := TstSetUp(t)
	store, storeTearDown := TstCreateTxStore(t)
	defer tearDown()
	defer storeTearDown()

	tx := createDecoratedTx(t, pool, store, []int64{4e6}, []int64{3e6})
	if err := storeTransactions(store, []*decoratedTx{tx}); err != nil {
		t.Fatal(err)
	}

	// Since the tx we created above has no change output, there should be no unspent
	// outputs (credits) in the txstore.
	credits, err := store.UnspentOutputs()
	if err != nil {
		t.Fatal(err)
	}
	if len(credits) != 0 {
		t.Fatalf("Unexpected number of credits in txstore; got %d, want 0", len(credits))
	}
}

func TestStoreTransactionsWithChangeOutput(t *testing.T) {
	tearDown, _, pool := TstSetUp(t)
	store, storeTearDown := TstCreateTxStore(t)
	defer tearDown()
	defer storeTearDown()

	// This will create a transaction with two outputs spending the whole amount from the
	// single input.
	tx := createDecoratedTx(t, pool, store, []int64{4e6}, []int64{3e6, 1e6})

	// storeTransactions() will store the tx created above, with the second output as a
	// change output.
	tx.hasChange = true
	if err := storeTransactions(store, []*decoratedTx{tx}); err != nil {
		t.Fatal(err)
	}

	// Check that the tx was stored in the txstore.
	sha, err := tx.msgtx.TxSha()
	if err != nil {
		t.Fatal(err)
	}
	storedTx := lookupStoredTx(store, &sha)
	if storedTx == nil {
		t.Fatal("The new tx doesn't seem to have been stored")
	}
	ignoreChange := true
	gotAmount := storedTx.OutputAmount(ignoreChange)
	if gotAmount != btcutil.Amount(3e6) {
		t.Fatal("Unexpected output amount; got %v, want %v", gotAmount, btcutil.Amount(3e6))
	}
	debits, _ := storedTx.Debits()
	if debits.InputAmount() != btcutil.Amount(4e6) {
		t.Fatal("Unexpected input amount; got %v, want %v", debits.InputAmount(),
			btcutil.Amount(4e6))
	}

	// There should be one unspent output (credit) in the txstore, corresponding to the
	// change output in the tx we created above.
	credits, err := store.UnspentOutputs()
	if err != nil {
		t.Fatal(err)
	}
	if len(credits) != 1 {
		t.Fatalf("Unexpected number of credits in txstore; got %d, want 1", len(credits))
	}
	credit := credits[0]
	if !credit.Change() {
		t.Fatalf("Credit doesn't come from a change output as we expected")
	}
	changeOut := tx.msgtx.TxOut[1]
	if credit.TxOut() != changeOut {
		t.Fatalf("Credit's txOut (%s) doesn't match changeOut (%s)", credit.TxOut(), changeOut)
	}
}

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
	transactions := []*decoratedTx{&decoratedTx{msgtx: tx, inputs: credits}}
	sigs, err := getRawSigs(transactions)
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
	change := inputAmount - (outputs[0].amount + outputs[1].amount + tx.calculateFee())
	expectedOutputs := append(
		outputs, NewOutputRequest("foo", 3, changeStart.Addr().String(), change))
	checkTxOutputs(t, tx.msgtx, expectedOutputs, mgr.Net())
}

// Check that withdrawal.status correctly states that no outputs were fulfilled when we
// don't have enough eligible credits for any of them.
func TestFulfilOutputsNoSatisfiableOutputs(t *testing.T) {
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
func TestFulfilOutputsNotEnoughCreditsForAllRequests(t *testing.T) {
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
	change := inputAmount - (out1.amount + out2.amount + tx.calculateFee())
	expectedOutputs := []*OutputRequest{out1, out2}
	sort.Sort(byOutBailmentID(expectedOutputs))
	expectedOutputs = append(
		expectedOutputs, NewOutputRequest("foo", 4, changeStart.Addr().String(), change))
	checkTxOutputs(t, tx.msgtx, expectedOutputs, mgr.Net())

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

func TestAddChange(t *testing.T) {
	teardown, _, pool := TstSetUp(t)
	store, storeTearDown := TstCreateTxStore(t)
	defer teardown()
	defer storeTearDown()

	input, output, fee := int64(4e6), int64(3e6), int64(10)
	tx := createDecoratedTx(t, pool, store, []int64{input}, []int64{output})
	tx.calculateFee = func() btcutil.Amount {
		return btcutil.Amount(fee)
	}

	if !tx.addChange([]byte{}) {
		t.Fatal("tx.addChange() returned false, meaning it did not add a change output")
	}
	if len(tx.msgtx.TxOut) != 2 {
		t.Fatalf("Unexpected number of txouts; got %d, want 2", len(tx.msgtx.TxOut))
	}
	gotChange := tx.msgtx.TxOut[1].Value
	wantChange := input - output - fee
	if gotChange != wantChange {
		t.Fatalf("Unexpected change amount; got %v, want %v", gotChange, wantChange)
	}
}

// TestAddChangeNoChange checks that decoratedTx.addChange() does not add a
// change output when there's no satoshis left after paying all outputs+fees.
func TestAddChangeNoChange(t *testing.T) {
	teardown, _, pool := TstSetUp(t)
	store, storeTearDown := TstCreateTxStore(t)
	defer teardown()
	defer storeTearDown()

	input, output, fee := int64(4e6), int64(4e6), int64(0)
	tx := createDecoratedTx(t, pool, store, []int64{input}, []int64{output})
	tx.calculateFee = func() btcutil.Amount {
		return btcutil.Amount(fee)
	}

	if tx.addChange([]byte{}) {
		t.Fatal("tx.addChange() returned true, meaning it added a change output")
	}
	if len(tx.msgtx.TxOut) != 1 {
		t.Fatalf("Unexpected number of txouts; got %d, want 1", len(tx.msgtx.TxOut))
	}
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

// checkTxOutputs checks that the address and amount of every output in the given tx match the
// address and amount of every item in the slice of OutputRequests.
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

// createDecoratedTx creates a decoratedTx with the given input and output amounts.
func createDecoratedTx(t *testing.T, pool *Pool, store *txstore.Store, inputAmounts []int64,
	outputAmounts []int64) *decoratedTx {
	tx := newDecoratedTx()
	credits := TstCreateCredits(t, pool, inputAmounts, store)
	for _, c := range credits {
		tx.addTxIn(c)
	}
	net := pool.Manager().Net()
	for i, amount := range outputAmounts {
		request := NewOutputRequest(
			"server", uint32(i), "34eVkREKgvvGASZW7hkgE2uNc1yycntMK6", btcutil.Amount(amount))
		pkScript, _ := request.pkScript(net)
		tx.addTxOut(&WithdrawalOutput{request: request}, pkScript)
	}
	return tx
}

// lookupStoredTx returns the TxRecord from the given store whose SHA matches the
// given ShaHash.
func lookupStoredTx(store *txstore.Store, sha *btcwire.ShaHash) *txstore.TxRecord {
	for _, r := range store.Records() {
		if bytes.Equal(r.Tx().Sha()[:], sha[:]) {
			return r
		}
	}
	return nil
}
