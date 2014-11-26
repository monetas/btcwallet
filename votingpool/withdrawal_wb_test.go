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
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

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
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

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
		t.Fatalf("Unexpected output amount; got %v, want %v", gotAmount, btcutil.Amount(3e6))
	}
	debits, _ := storedTx.Debits()
	if debits.InputAmount() != btcutil.Amount(4e6) {
		t.Fatalf("Unexpected input amount; got %v, want %v", debits.InputAmount(),
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

func TestGetRawSigs(t *testing.T) {
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

	tx := createTxWithInputAmounts(t, pool, []int64{5e6, 4e6}, store)

	sigs, err := getRawSigs([]*decoratedTx{tx})
	if err != nil {
		t.Fatal(err)
	}

	txSigs := sigs[tx.Ntxid()]
	if len(txSigs) != len(tx.inputs) {
		t.Fatalf("Unexpected number of sig lists; got %d, want %d", len(txSigs), len(tx.inputs))
	}

	checkNonEmptySigsForPrivKeys(t, txSigs, tx.inputs[0].Address().Series().privateKeys)

	// Since we have all the necessary signatures (m-of-n), we construct the
	// sigsnature scripts and execute them to make sure the raw signatures are
	// valid.
	signTxAndValidate(t, pool.Manager(), tx.msgtx, txSigs, tx.inputs)
}

func TestGetRawSigsOnlyOnePrivKeyAvailable(t *testing.T) {
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

	tx := createTxWithInputAmounts(t, pool, []int64{5e6, 4e6}, store)
	// Remove all private keys but the first one from the credit's series.
	series := tx.inputs[0].Address().Series()
	for i := range series.privateKeys[1:] {
		series.privateKeys[i] = nil
	}

	sigs, err := getRawSigs([]*decoratedTx{tx})
	if err != nil {
		t.Fatal(err)
	}

	txSigs := sigs[tx.Ntxid()]
	if len(txSigs) != len(tx.inputs) {
		t.Fatalf("Unexpected number of sig lists; got %d, want %d", len(txSigs), len(tx.inputs))
	}

	checkNonEmptySigsForPrivKeys(t, txSigs, series.privateKeys)
}

func TestGetRawSigsUnparseableRedeemScript(t *testing.T) {
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

	tx := createTxWithInputAmounts(t, pool, []int64{5e6, 4e6}, store)
	// Change the redeem script for one of our tx inputs, to force an error in
	// getRawSigs().
	tx.inputs[0].Address().script = []byte{0x01}

	_, err := getRawSigs([]*decoratedTx{tx})

	TstCheckError(t, "", err, ErrRawSigning)
}

func TestGetRawSigsInvalidAddrBranch(t *testing.T) {
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

	tx := createTxWithInputAmounts(t, pool, []int64{5e6, 4e6}, store)
	// Change the branch of our input's address to an invalid value, to force
	// an error in getRawSigs().
	tx.inputs[0].Address().branch = Branch(999)

	_, err := getRawSigs([]*decoratedTx{tx})

	TstCheckError(t, "", err, ErrInvalidBranch)
}

// Check that all outputs requested in a withdrawal match the outputs of the generated
// transaction(s).
func TestWithdrawalTxOutputs(t *testing.T) {
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

	// Create eligible inputs and the list of outputs we need to fulfil.
	seriesID, eligible := TstCreateCredits(t, pool, []int64{2e6, 4e6}, store)
	outputs := []*OutputRequest{
		NewOutputRequest("foo", 1, "34eVkREKgvvGASZW7hkgE2uNc1yycntMK6", btcutil.Amount(3e6)),
		NewOutputRequest("foo", 2, "3PbExiaztsSYgh6zeMswC49hLUwhTQ86XG", btcutil.Amount(2e6)),
	}
	changeStart, err := pool.ChangeAddress(seriesID, 0)
	if err != nil {
		t.Fatal(err)
	}

	w := newWithdrawal(0, outputs, eligible, changeStart, pool.Manager().Net())
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
	checkMsgTxOutputs(t, tx.msgtx, expectedOutputs, pool.Manager().Net())
}

// Check that withdrawal.status correctly states that no outputs were fulfilled when we
// don't have enough eligible credits for any of them.
func TestFulfilOutputsNoSatisfiableOutputs(t *testing.T) {
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

	seriesID, eligible := TstCreateCredits(t, pool, []int64{1e6}, store)
	outputs := []*OutputRequest{
		NewOutputRequest("foo", 1, "3Qt1EaKRD9g9FeL2DGkLLswhK1AKmmXFSe", btcutil.Amount(3e6))}
	changeStart, err := pool.ChangeAddress(seriesID, 0)
	if err != nil {
		t.Fatal(err)
	}

	w := newWithdrawal(0, outputs, eligible, changeStart, pool.Manager().Net())
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
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

	// Create eligible inputs and the list of outputs we need to fulfil.
	seriesID, eligible := TstCreateCredits(t, pool, []int64{2e6, 4e6}, store)
	out1 := NewOutputRequest("foo", 1, "34eVkREKgvvGASZW7hkgE2uNc1yycntMK6", btcutil.Amount(3e6))
	out2 := NewOutputRequest("foo", 2, "3PbExiaztsSYgh6zeMswC49hLUwhTQ86XG", btcutil.Amount(2e6))
	out3 := NewOutputRequest("foo", 3, "3Qt1EaKRD9g9FeL2DGkLLswhK1AKmmXFSe", btcutil.Amount(5e6))
	outputs := []*OutputRequest{out1, out2, out3}
	changeStart, err := pool.ChangeAddress(seriesID, 0)
	if err != nil {
		t.Fatal(err)
	}

	w := newWithdrawal(0, outputs, eligible, changeStart, pool.Manager().Net())
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
	checkMsgTxOutputs(t, tx.msgtx, expectedOutputs, pool.Manager().Net())

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
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

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
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

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

// TestRollbackLastOutput tests the case where we rollback one output
// and one input, such that sum(in) >= sum(out) + fee.
func TestRollbackLastOutput(t *testing.T) {
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

	tx := createDecoratedTx(t, pool, store, []int64{3, 3, 2, 1, 3}, []int64{3, 3, 2, 2})
	initialInputs := tx.inputs
	initialOutputs := tx.outputs

	tx.calculateFee = func() btcutil.Amount {
		return btcutil.Amount(1)
	}
	removedInputs, removedOutput, err := tx.rollBackLastOutput()
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	// The above rollBackLastOutput() call should have removed the last output
	// and the last input.
	lastOutput := initialOutputs[len(initialOutputs)-1]
	if removedOutput != lastOutput {
		t.Fatalf("Wrong rolled back output; got %s want %s", removedOutput, lastOutput)
	}
	if len(removedInputs) != 1 {
		t.Fatalf("Unexpected number of inputs removed; got %d, want 1", len(removedInputs))
	}
	lastInput := initialInputs[len(initialInputs)-1]
	if removedInputs[0] != lastInput {
		t.Fatalf("Wrong rolled back input; got %s want %s", removedInputs[0], lastInput)
	}

	// Now check that the inputs and outputs left in the tx match what we
	// expect.
	checkTxOutputs(t, tx, initialOutputs[:len(initialOutputs)-1], pool.Manager().Net())
	checkTxInputs(t, tx, initialInputs[:len(initialInputs)-1])
}

// TestRollbackLastOutputEdgeCase where we roll back one output but no
// inputs, such that sum(in) >= sum(out) + fee.
func TestRollbackLastOutputNoInputsRolledBack(t *testing.T) {
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

	tx := createDecoratedTx(t, pool, store, []int64{4}, []int64{2, 3})
	initialInputs := tx.inputs
	initialOutputs := tx.outputs

	tx.calculateFee = func() btcutil.Amount {
		return btcutil.Amount(1)
	}
	removedInputs, removedOutput, err := tx.rollBackLastOutput()
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	// The above rollBackLastOutput() call should have removed the
	// last output but no inputs.
	lastOutput := initialOutputs[len(initialOutputs)-1]
	if removedOutput != lastOutput {
		t.Fatalf("Wrong output; got %d want %d", removedOutput, lastOutput)
	}
	if len(removedInputs) != 0 {
		t.Fatalf("Expected no removed inputs, but got %d inputs", len(removedInputs))
	}

	// Now check that the inputs and outputs left in the tx match what we
	// expect.
	checkTxOutputs(t, tx, initialOutputs[:len(initialOutputs)-1], pool.Manager().Net())
	checkTxInputs(t, tx, initialInputs)
}

// TODO: Check that tx.outputTotal is updated
func TestPopOutput(t *testing.T) {
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()
	net := pool.Manager().Net()

	tx := createDecoratedTx(t, pool, store, []int64{}, []int64{1, 2})
	outputs := tx.outputs
	// Make sure we have created the transaction with the expected
	// outputs.
	checkTxOutputs(t, tx, outputs, net)
	remainingTxOut := tx.msgtx.TxOut[0]
	remainingWithdrawalOutput := tx.outputs[0]
	wantPoppedWithdrawalOutput := tx.outputs[1]

	// Pop!
	gotPoppedWithdrawalOutput := tx.popOutput()

	// Check the popped output looks correct.
	if gotPoppedWithdrawalOutput != wantPoppedWithdrawalOutput {
		t.Fatalf("Popped output wrong; got %v, want %v",
			gotPoppedWithdrawalOutput, wantPoppedWithdrawalOutput)
	}
	// And that the remaining output is correct.
	checkTxOutputs(t, tx, []*WithdrawalOutput{remainingWithdrawalOutput}, net)

	// Make sure that the remaining output is really the right one.
	if tx.msgtx.TxOut[0] != remainingTxOut {
		t.Fatalf("Wrong TxOut: got %v, want %v", tx.msgtx.TxOut[0], remainingTxOut)
	}
	if tx.outputs[0] != remainingWithdrawalOutput {
		t.Fatalf("Wrong WithdrawalOutput: got %v, want %v",
			tx.outputs[0], remainingWithdrawalOutput)
	}
}

// TODO: Check that tx.inputTotal is updated
func TestPopInput(t *testing.T) {
	tearDown, pool, store := TstCreatePoolAndTxStore(t)
	defer tearDown()

	tx := createDecoratedTx(t, pool, store, []int64{1, 2}, []int64{})
	inputs := tx.inputs
	// Make sure we have created the transaction with the expected inputs
	checkTxInputs(t, tx, inputs)

	remainingTxIn := tx.msgtx.TxIn[0]
	remainingCreditInterface := tx.inputs[0]
	wantPoppedCreditInterface := tx.inputs[1]

	// Pop!
	gotPoppedCreditInterface := tx.popInput()

	// Check the popped input looks correct.
	if gotPoppedCreditInterface != wantPoppedCreditInterface {
		t.Fatalf("Popped input wrong; got %v, want %v",
			gotPoppedCreditInterface, wantPoppedCreditInterface)
	}
	checkTxInputs(t, tx, inputs[0:1])

	// Make sure that the remaining input is really the right one.
	if tx.msgtx.TxIn[0] != remainingTxIn {
		t.Fatalf("Wrong TxIn: got %v, want %v", tx.msgtx.TxIn[0], remainingTxIn)
	}
	if tx.inputs[0] != remainingCreditInterface {
		t.Fatalf("Wrong input: got %v, want %v", tx.inputs[0], remainingCreditInterface)
	}
}

// TestRollBackLastOutputInsufficientOutputs checks that
// rollBackLastOutput returns an error if there are less than two
// outputs in the transaction.
func TestRollBackLastOutputInsufficientOutputs(t *testing.T) {
	tx := newDecoratedTx()
	_, _, err := tx.rollBackLastOutput()
	TstCheckError(t, "", err, ErrPreconditionNotMet)

	output := &WithdrawalOutput{request: &OutputRequest{amount: btcutil.Amount(3)}}
	tx.addTxOut(output, []byte{})
	_, _, err = tx.rollBackLastOutput()
	TstCheckError(t, "", err, ErrPreconditionNotMet)
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

// checkNonEmptySigsForPrivKeys checks that every signature list in txSigs has
// one non-empty signature for every non-nil private key in the given list. This
// is to make sure every signature list matches the specification at
// http://opentransactions.org/wiki/index.php/Siglist.
func checkNonEmptySigsForPrivKeys(t *testing.T, txSigs TxSigs, privKeys []*hdkeychain.ExtendedKey) {
	for _, txInSigs := range txSigs {
		if len(txInSigs) != len(privKeys) {
			t.Fatalf("Number of items in sig list (%d) does not match number of privkeys (%d)",
				len(txInSigs), len(privKeys))
		}
		for sigIdx, sig := range txInSigs {
			key := privKeys[sigIdx]
			if bytes.Equal(sig, []byte{}) && key != nil {
				t.Fatalf("Empty signature (idx=%d) but key (%s) is available",
					sigIdx, key.String())
			} else if !bytes.Equal(sig, []byte{}) && key == nil {
				t.Fatalf("Signature not empty (idx=%d) but key is not available", sigIdx)
			}
		}
	}
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

// checkTxOutputs ensures that tx.outputs match the given outputs and that the
// address and amount of the items in tx.msgtx.TxOut match the address and
// amount of the given outputs.
func checkTxOutputs(t *testing.T, tx *decoratedTx, outputs []*WithdrawalOutput, net *btcnet.Params) {
	nOutputs := len(outputs)
	if len(tx.outputs) != nOutputs {
		t.Fatalf("Wrong number of outputs in tx; got %d, want %d", len(tx.outputs), nOutputs)
	}
	for i, output := range tx.outputs {
		if output != outputs[i] {
			t.Fatalf("Unexpected output; got %d, want %d", output, outputs[i])
		}
	}
	outputRequests := make([]*OutputRequest, nOutputs)
	for i, output := range outputs {
		outputRequests[i] = output.request
	}
	checkMsgTxOutputs(t, tx.msgtx, outputRequests, net)
}

// checkMsgTxOutputs checks that the address and amount of every output in the
// given msgtx match the address and amount of every item in the slice of
// OutputRequests.
func checkMsgTxOutputs(t *testing.T, msgtx *btcwire.MsgTx, outputs []*OutputRequest, net *btcnet.Params) {
	nOutputs := len(outputs)
	if len(msgtx.TxOut) != nOutputs {
		t.Fatalf("Unexpected number of TxOuts; got %d, want %d", len(msgtx.TxOut), nOutputs)
	}
	for i, output := range outputs {
		txOut := msgtx.TxOut[i]
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

// checkTxInputs ensures that tx.inputs match the given inputs and that the
// outpoints of the items in tx.msgtx.TxIn match the outpoints of the given
// inputs.
func checkTxInputs(t *testing.T, tx *decoratedTx, inputs []CreditInterface) {
	if len(tx.inputs) != len(inputs) {
		t.Fatalf("Wrong number of inputs in tx; got %d, want %d", len(tx.inputs), len(inputs))
	}
	for i, input := range tx.inputs {
		if input != inputs[i] {
			t.Fatalf("Unexpected input; got %s, want %s", input, inputs[i])
		}
	}

	if len(tx.msgtx.TxIn) != len(inputs) {
		t.Fatalf("Wrong number of inputs in msgtx.TxIn; got %d, want %d",
			len(tx.msgtx.TxIn), len(inputs))
	}
	for i, input := range tx.msgtx.TxIn {
		if input.PreviousOutPoint != *inputs[i].OutPoint() {
			t.Fatalf("Unexpected TxIn outpoint; got %v, want %v",
				input.PreviousOutPoint, *inputs[i].OutPoint())
		}
	}
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
