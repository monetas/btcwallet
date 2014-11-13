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
	"github.com/conformal/btcwallet/txstore"
	vp "github.com/conformal/btcwallet/votingpool"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

var bsHeight int32 = 11112
var bs = &waddrmgr.BlockStamp{Height: bsHeight}

// XXX: This test could benefit from being split into smaller ones, but that won't be a
// trivial endeavour.
func TestWithdrawal(t *testing.T) {
	teardown, mgr, pool := vp.TstSetUp(t)
	store, storeTearDown := vp.TstCreateTxStore(t)
	defer teardown()
	defer storeTearDown()

	// Create eligible inputs and the list of outputs we need to fulfil.
	eligible := createCredits(t, mgr, pool, []int64{5e6, 4e6}, store)
	address := "1MirQ9bwyQcGVJPwKUgapu5ouK2E2Ey4gX"
	outputs := []*vp.OutputRequest{
		vp.NewOutputRequest("foo", 1, address, btcutil.Amount(4e6)),
		vp.NewOutputRequest("foo", 2, address, btcutil.Amount(1e6)),
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
	ntxid := "3c78a70c0b1e78f56e7dba1fffa31a7240062ee4b66b2b8a45ee63485beb97b2"
	txSigs := sigs[ntxid]
	// We should have 2 TxInSignatures entries as the transaction created by
	// vp.Withdrawal() should have two inputs.
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
		err = vp.SignMultiSigUTXO(mgr, tx, i, txOut.PkScript, txSigs[i], mgr.Net())
		if err != nil {
			t.Fatal(err)
		}
	}

	if err = vp.ValidateSigScripts(tx, store); err != nil {
		t.Fatal(err)
	}
}

func checkWithdrawalOutput(
	t *testing.T, withdrawalOutput *vp.WithdrawalOutput, address, status string,
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

func createMasterKey(t *testing.T, seed []byte) *hdkeychain.ExtendedKey {
	key, err := hdkeychain.NewMaster(seed)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func createCredits(
	t *testing.T, mgr *waddrmgr.Manager, pool *vp.Pool, amounts []int64,
	store *txstore.Store) []vp.CreditInterface {
	// Create 3 master extended keys, as if we had 3 voting pool members.
	masters := []*hdkeychain.ExtendedKey{
		createMasterKey(t, bytes.Repeat([]byte{0x00, 0x01}, 16)),
		createMasterKey(t, bytes.Repeat([]byte{0x02, 0x01}, 16)),
		createMasterKey(t, bytes.Repeat([]byte{0x03, 0x01}, 16)),
	}
	rawPubKeys := make([]string, 3)
	rawPrivKeys := make([]string, 3)
	for i, key := range masters {
		rawPrivKeys[i] = key.String()
		pubkey, _ := key.Neuter()
		rawPubKeys[i] = pubkey.String()
	}

	// Create a series with the master pubkeys of our voting pool members, also empowering
	// it with all corresponding private keys.
	reqSigs := uint32(2)
	seriesID := uint32(0)
	if err := pool.CreateSeries(1, seriesID, reqSigs, rawPubKeys); err != nil {
		t.Fatalf("Cannot creates series: %v", err)
	}
	vp.TstUnlockManager(t, mgr)
	for _, key := range rawPrivKeys {
		if err := pool.EmpowerSeries(seriesID, key); err != nil {
			t.Fatal(err)
		}
	}

	// Finally create the Credit instances, locked to the voting pool's deposit
	// address with branch==1, index==0.
	branch := vp.Branch(1)
	idx := vp.Index(0)
	pkScript := vp.TstCreatePkScript(t, mgr, pool, seriesID, branch, idx)
	eligible := make([]vp.CreditInterface, len(amounts))
	for i, credit := range vp.TstCreateInputs(t, store, pkScript, amounts) {
		addr, err := pool.WithdrawalAddress(seriesID, branch, idx)
		if err != nil {
			t.Fatal(err)
		}
		eligible[i] = vp.NewCredit(credit, *addr)
	}
	return eligible
}
