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

// Helpers to create parameterized objects to use in tests.

package votingpool

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/conformal/btcnet"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcutil/hdkeychain"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwallet/walletdb"
	"github.com/conformal/btcwire"
)

var (
	// seed is the master seed used to create extended keys.
	seed           = bytes.Repeat([]byte{0x2a, 0x64, 0xdf, 0x08}, 8)
	pubPassphrase  = []byte("_DJr{fL4H0O}*-0\n:V1izc)(6BomK")
	privPassphrase = []byte("81lUHXnOMZ@?XXd7O9xyDIWIbXX-lj")
	uniqueCounter  = uint32(0)
)

func getUniqueID() uint32 {
	return atomic.AddUint32(&uniqueCounter, 1)
}

// createDecoratedTx creates a decoratedTx with the given input and output amounts.
func createDecoratedTx(t *testing.T, pool *Pool, store *txstore.Store, inputAmounts []int64,
	outputAmounts []int64) *decoratedTx {
	tx := newDecoratedTx()
	_, credits := TstCreateCredits(t, pool, inputAmounts, store)
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

// createTxWithInputAmounts returns a new decoratedTx containing just inputs
// with the given amounts.
func createTxWithInputAmounts(
	t *testing.T, pool *Pool, amounts []int64, store *txstore.Store) *decoratedTx {
	tx := newDecoratedTx()
	_, credits := TstCreateCredits(t, pool, amounts, store)
	for _, c := range credits {
		tx.addTxIn(c)
	}
	return tx
}

func createMsgTx(pkScript []byte, amts []int64) *btcwire.MsgTx {
	msgtx := &btcwire.MsgTx{
		Version: 1,
		TxIn: []*btcwire.TxIn{
			{
				PreviousOutPoint: btcwire.OutPoint{
					Hash:  btcwire.ShaHash{},
					Index: 0xffffffff,
				},
				SignatureScript: []byte{btcscript.OP_NOP},
				Sequence:        0xffffffff,
			},
		},
		LockTime: 0,
	}

	for _, amt := range amts {
		msgtx.AddTxOut(btcwire.NewTxOut(amt, pkScript))
	}
	return msgtx
}

func TstCreatePkScript(t *testing.T, pool *Pool, series uint32, branch Branch, index Index) []byte {
	script, err := pool.DepositScript(series, branch, index)
	if err != nil {
		t.Fatalf("Failed to create depositscript for series %d, branch %d, index %d: %v", series, branch, index, err)
	}

	mgr := pool.Manager()
	TstUnlockManager(t, mgr)
	defer mgr.Lock()
	// We need to pass the bsHeight, but currently if we just pass
	// anything > 0, then the ImportScript will be happy. It doesn't
	// save the value, but only uses it to check if it needs to update
	// the startblock.
	var bsHeight int32 = 1
	addr, err := mgr.ImportScript(script, &waddrmgr.BlockStamp{Height: bsHeight})
	if err != nil {
		t.Fatal(err)
	}

	pkScript, err := btcscript.PayToAddrScript(addr.Address())
	if err != nil {
		t.Fatal(err)
	}
	return pkScript
}

func TstCreateTxStore(t *testing.T) (store *txstore.Store, tearDown func()) {
	dir, err := ioutil.TempDir("", "tx.bin")
	if err != nil {
		t.Fatalf("Failed to create db file: %v", err)
	}
	s := txstore.New(dir)
	return s, func() { os.RemoveAll(dir) }
}

type TstSeriesDef struct {
	ReqSigs  uint32
	PubKeys  []string
	PrivKeys []string
	SeriesID uint32
}

// TstCreateSeries creates a new Series for every definition in the given slice
// of TstSeriesDef. If the definition includes any private keys, the Series is
// empowered with them.
func TstCreateSeries(t *testing.T, pool *Pool, definitions []TstSeriesDef) {
	// Unlock the manager in case we have any private keys to load.
	TstUnlockManager(t, pool.Manager())
	defer pool.Manager().Lock()

	for _, def := range definitions {
		err := pool.CreateSeries(CurrentVersion, def.SeriesID, def.ReqSigs, def.PubKeys)
		if err != nil {
			t.Fatalf("Cannot creates series %d: %v", def.SeriesID, err)
		}
		for _, key := range def.PrivKeys {
			if err := pool.EmpowerSeries(def.SeriesID, key); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TstCreatePkScripts(t *testing.T, pool *Pool, aRange AddressRange) [][]byte {
	var pkScripts [][]byte
	for index := aRange.StartIndex; index <= aRange.StopIndex; index++ {
		for branch := aRange.StartBranch; branch <= aRange.StopBranch; branch++ {

			pkScript := TstCreatePkScript(t, pool, aRange.SeriesID, branch, index)
			pkScripts = append(pkScripts, pkScript)
		}
	}
	return pkScripts
}

func TstCreateMasterKey(t *testing.T, seed []byte) *hdkeychain.ExtendedKey {
	key, err := hdkeychain.NewMaster(seed)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// createMasterKeys creates count master ExtendedKeys with unique seeds.
func createMasterKeys(t *testing.T, count int) []*hdkeychain.ExtendedKey {
	keys := make([]*hdkeychain.ExtendedKey, count)
	for i := range keys {
		keys[i] = TstCreateMasterKey(t, bytes.Repeat(uint32ToBytes(getUniqueID()), 4))
	}
	return keys
}

// TstCreateSeriesDef creates a TstSeriesDef with a unique SeriesID, the given
// reqSigs and the raw public/private keys extracted from the list of private
// keys. The new series will be empowered with all private keys.
func TstCreateSeriesDef(t *testing.T, reqSigs uint32, keys []*hdkeychain.ExtendedKey) TstSeriesDef {
	pubKeys := make([]string, len(keys))
	privKeys := make([]string, len(keys))
	for i, key := range keys {
		privKeys[i] = key.String()
		pubkey, _ := key.Neuter()
		pubKeys[i] = pubkey.String()
	}
	return TstSeriesDef{
		ReqSigs: reqSigs, SeriesID: getUniqueID(), PubKeys: pubKeys, PrivKeys: privKeys}
}

func TstCreatePoolAndTxStore(t *testing.T) (tearDown func(), pool *Pool, store *txstore.Store) {
	mgrTearDown, _, pool := TstCreatePool(t)
	store, storeTearDown := TstCreateTxStore(t)
	tearDown = func() {
		mgrTearDown()
		storeTearDown()
	}
	return tearDown, pool, store
}

// TstCreateCredits creates a new Series (with a unique ID) and a slice of
// credits locked to the series' address with branch==1 and index==0. The new
// Series will use a 2-of-3 configuration and will be empowered with all of its
// private keys.
func TstCreateCredits(
	t *testing.T, pool *Pool, amounts []int64, store *txstore.Store) (uint32, []CreditInterface) {
	masters := []*hdkeychain.ExtendedKey{
		TstCreateMasterKey(t, bytes.Repeat(uint32ToBytes(getUniqueID()), 4)),
		TstCreateMasterKey(t, bytes.Repeat(uint32ToBytes(getUniqueID()), 4)),
		TstCreateMasterKey(t, bytes.Repeat(uint32ToBytes(getUniqueID()), 4)),
	}
	def := TstCreateSeriesDef(t, 2, masters)
	TstCreateSeries(t, pool, []TstSeriesDef{def})
	return def.SeriesID, TstCreateCreditsOnSeries(t, pool, def.SeriesID, amounts, store)
}

// TstCreateCreditsOnSeries creates a slice of credits locked to the given
// series' address with branch==1 and index==0.
func TstCreateCreditsOnSeries(t *testing.T, pool *Pool, seriesID uint32, amounts []int64,
	store *txstore.Store) []CreditInterface {
	branch := Branch(1)
	idx := Index(0)
	pkScript := TstCreatePkScript(t, pool, seriesID, branch, idx)
	eligible := make([]CreditInterface, len(amounts))
	for i, credit := range TstCreateInputs(t, store, pkScript, amounts) {
		addr, err := pool.WithdrawalAddress(seriesID, branch, idx)
		if err != nil {
			t.Fatal(err)
		}
		eligible[i] = NewCredit(credit, *addr)
	}
	return eligible
}

// TstCreateInputs is a convenience function.  See TstCreateInputsOnBlock
// for a more flexible version.
func TstCreateInputs(t *testing.T, store *txstore.Store, pkScript []byte, amounts []int64) []txstore.Credit {
	blockTxIndex := 1 // XXX: hardcoded value.
	blockHeight := 10 // XXX: hardcoded value.
	return TstCreateInputsOnBlock(t, store, blockTxIndex, blockHeight, pkScript, amounts)
}

// TstCreateInputsOnBlock creates a number of inputs by creating a transaction
// with a number of outputs corresponding to the elements of the amounts slice.
//
// The transaction is added to a block and the index and blockheight must be
// specified.
func TstCreateInputsOnBlock(t *testing.T, s *txstore.Store,
	blockTxIndex, blockHeight int, pkScript []byte, amounts []int64) []txstore.Credit {
	msgTx := createMsgTx(pkScript, amounts)
	block := &txstore.Block{
		Height: int32(blockHeight),
	}

	tx := btcutil.NewTx(msgTx)
	tx.SetIndex(blockTxIndex)

	r, err := s.InsertTx(tx, block)
	if err != nil {
		t.Fatal("Failed to create inputs: ", err)
	}

	credits := make([]txstore.Credit, len(msgTx.TxOut))
	for i := range msgTx.TxOut {
		credit, err := r.AddCredit(uint32(i), false)
		if err != nil {
			t.Fatal("Failed to create inputs: ", err)
		}
		credits[i] = credit
	}
	return credits
}

// TstCreatePool creates a Pool on a fresh walletdb and returns it. It also
// returns the pool's waddrmgr.Manager (which uses the same walletdb, but with a
// different namespace) as a convenience, and a teardown function that closes
// the Manager and removes the directory used to store the database.
func TstCreatePool(t *testing.T) (tearDownFunc func(), mgr *waddrmgr.Manager, pool *Pool) {
	// XXX: This should be moved somewhere else eventually as not all of our
	// tests call this function, but they should all run in parallel.
	t.Parallel()

	// Create a new wallet DB and addr manager.
	dir, err := ioutil.TempDir("", "pool_test")
	if err != nil {
		t.Fatalf("Failed to create db dir: %v", err)
	}
	db, err := walletdb.Create("bdb", filepath.Join(dir, "wallet.db"))
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
	mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
	if err != nil {
		t.Fatalf("Failed to create addr manager DB namespace: %v", err)
	}
	var fastScrypt = &waddrmgr.Options{ScryptN: 16, ScryptR: 8, ScryptP: 1}
	mgr, err = waddrmgr.Create(mgrNamespace, seed, pubPassphrase, privPassphrase,
		&btcnet.MainNetParams, fastScrypt)
	if err != nil {
		t.Fatalf("Failed to create addr manager: %v", err)
	}

	// Create a walletdb for votingpools.
	vpNamespace, err := db.Namespace([]byte("votingpool"))
	if err != nil {
		t.Fatalf("Failed to create VotingPool DB namespace: %v", err)
	}
	pool, err = Create(vpNamespace, mgr, []byte{0x00})
	if err != nil {
		t.Fatalf("Voting Pool creation failed: %v", err)
	}
	tearDownFunc = func() {
		db.Close()
		mgr.Close()
		os.RemoveAll(dir)
	}
	return tearDownFunc, mgr, pool
}
