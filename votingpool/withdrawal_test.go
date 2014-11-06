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
	"testing"

	"github.com/conformal/btcutil"
	"github.com/conformal/btcutil/hdkeychain"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/votingpool"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

var bsHeight int32 = 11112
var bs *waddrmgr.BlockStamp = &waddrmgr.BlockStamp{Height: bsHeight}

// XXX: This test could benefit from being split into smaller ones, but that won't be a
// trivial endeavour.
func TestWithdrawal(t *testing.T) {
	teardown, mgr, pool := setUp(t)
	store, storeTearDown := createTxStore(t)
	defer teardown()
	defer storeTearDown()

	// Create eligible inputs and the list of outputs we need to fulfil.
	eligible := createCredits(t, mgr, pool, []int64{5e6, 4e6}, store)
	address := "1MirQ9bwyQcGVJPwKUgapu5ouK2E2Ey4gX"
	outputs := []*votingpool.OutputRequest{
		votingpool.NewOutputRequest("foo", 1, address, btcutil.Amount(4e6)),
		votingpool.NewOutputRequest("foo", 2, address, btcutil.Amount(1e6)),
	}
	changeStart, err := pool.ChangeAddress(0, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Withdrawal() should fulfil the desired outputs spending from the given inputs.
	status, sigs, err := pool.Withdrawal(0, outputs, eligible, changeStart, store)
	if err != nil {
		t.Fatal(err)
	}

	// Check that all outputs were successfully fulfiled.
	fulfiled := status.Outputs()
	if len(fulfiled) != 2 {
		t.Fatalf("Unexpected number of outputs in WithdrawalStatus; got %d, want %d",
			len(fulfiled), 2)
	}
	for _, withdrawalOutput := range fulfiled {
		checkWithdrawalOutput(t, withdrawalOutput, address, "success", 1)
	}

	// Now check that the raw signatures are what we expect.
	if len(sigs) != 1 {
		t.Fatalf("Unexpected number of tx signature lists; got %d, want 1", len(sigs))
	}
	// XXX: The ntxid is deterministic so we hardcode it here, but if the test is changed
	// in a way that causes the generated transactions to change (e.g. different
	// inputs/outputs), the ntxid will change too.
	ntxid := "c47af4b04a82caa5c34bded7cf3869fbb690fd572c2b87f70c915892fa828235"
	txSigs := sigs[ntxid]
	// We should have 2 TxInSignatures entries as the transaction created by
	// votingpool.Withdrawal() should have two inputs.
	if len(txSigs) != 2 {
		t.Fatalf("Unexpected number of signature lists; got %d, want %d", len(txSigs), 2)
	}
	// And we should have 3 raw signatures as we have all 3 private keys for this
	// voting pool series loaded in the address manager.
	txInSigs := txSigs[0]
	if len(txInSigs) != 3 {
		t.Fatalf("Unexpected number of raw signatures; got %d, want %d", len(txInSigs), 3)
	}

	// Finally we use SignMultiSigUTXO() to construct the SignatureScripts (using the raw
	// signatures), and check that they are valid.
	sha, _ := btcwire.NewShaHashFromStr(ntxid)
	tx := store.UnminedTx(sha).MsgTx()
	for i, txIn := range tx.TxIn {
		txOut, err := store.UnconfirmedSpent(txIn.PreviousOutPoint)
		if err != nil {
			t.Fatal(err)
		}
		err = votingpool.SignMultiSigUTXO(mgr, tx, i, txOut.PkScript, txSigs[i], mgr.Net())
		if err != nil {
			t.Fatal(err)
		}
	}

	if err = votingpool.ValidateSigScripts(tx, store); err != nil {
		t.Fatal(err)
	}
}

func checkWithdrawalOutput(
	t *testing.T, withdrawalOutput *votingpool.WithdrawalOutput, address, status string,
	nOutpoints int) {
	if withdrawalOutput.Address() != address {
		t.Fatalf("Unexpected address; got %s, want %s", withdrawalOutput.Address(), address)
	}

	if withdrawalOutput.Status() != status {
		t.Fatalf("Unexpected status; got '%s', want '%s'", withdrawalOutput.Status(), status)
	}

	if len(withdrawalOutput.Outpoints()) != nOutpoints {
		t.Fatalf("Unexpected number of outpoints; got %d, want %d",
			len(withdrawalOutput.Outpoints()), nOutpoints)
	}
}

func createCredits(
	t *testing.T, mgr *waddrmgr.Manager, pool *votingpool.VotingPool, amounts []int64,
	store *txstore.Store) []txstore.Credit {
	// Create 3 master extended keys, as if we had 3 voting pool members.
	master1, _ := hdkeychain.NewMaster(seed)
	master2, _ := hdkeychain.NewMaster(append(seed, byte(0x01)))
	master3, _ := hdkeychain.NewMaster(append(seed, byte(0x02)))
	masters := []*hdkeychain.ExtendedKey{master1, master2, master3}
	rawPubKeys := make([]string, 3)
	for i, key := range masters {
		pubkey, _ := key.Neuter()
		rawPubKeys[i] = pubkey.String()
	}

	// Create a series with the master pubkeys of our voting pool members.
	reqSigs := uint32(2)
	seriesID := uint32(0)
	if err := pool.CreateSeries(1, seriesID, reqSigs, rawPubKeys); err != nil {
		t.Fatalf("Cannot creates series: %v", err)
	}

	idx := uint32(0)
	// Import the 0th child of our master keys into the address manager as we're going
	// to need them when signing the transactions later on.
	wifs := make([]string, 3)
	for i, master := range masters {
		child, _ := master.Child(idx)
		ecPrivKey, _ := child.ECPrivKey()
		wif, _ := btcutil.NewWIF(ecPrivKey, mgr.Net(), true)
		wifs[i] = wif.String()
	}
	importPrivateKeys(t, mgr, wifs, bs)

	// Finally create the Credit instances, locked to the voting pool's deposit
	// address with branch==0, index==0.
	branch := uint32(0)
	pkScript := createVotingPoolPkScript(t, mgr, pool, bsHeight, seriesID, branch, idx)
	return createInputs(t, pkScript, amounts, store)
}
