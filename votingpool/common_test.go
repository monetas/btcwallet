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
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
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

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

// TstCreateInputs is a convenience function.  See TstCreateInputsOnBlock
// for a more flexible version.
func TstCreateInputs(t *testing.T, store *txstore.Store, pkScript []byte, amounts []int64) []txstore.Credit {
	blockTxIndex := 1 // XXX: hardcoded value.
	blockHeight := 10 // XXX: hardcoded value.
	return TstCreateInputsOnBlock(t, store, blockTxIndex, blockHeight, pkScript, amounts)
}

// TstCreateInputsOnBlock creates a number of inputs by creating a
// transaction with a number of outputs corresponding to the elements
// of the amounts slice.
//
// The transaction is added to a block and the index and and
// blockheight must be specified.
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
		panic(err)
	}

	pkScript, err := btcscript.PayToAddrScript(addr.Address())
	if err != nil {
		panic(err)
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
	SeriesID uint32
}

func TstCreateSeries(t *testing.T, pool *Pool, definitions []TstSeriesDef) {
	for _, def := range definitions {
		err := pool.CreateSeries(CurrentVersion, def.SeriesID, def.ReqSigs, def.PubKeys)
		if err != nil {
			t.Fatalf("Cannot creates series %d: %v", def.SeriesID, err)
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

// TstCheckError ensures the passed error is a votingpool.Error with an error
// code that matches the passed error code.
func TstCheckError(t *testing.T, testName string, gotErr error, wantErrCode ErrorCode) {
	vpErr, ok := gotErr.(Error)
	if !ok {
		t.Errorf("%s: unexpected error type - got %T, want %T",
			testName, gotErr, Error{})
	}
	if vpErr.ErrorCode != wantErrCode {
		t.Errorf("%s: unexpected error code - got %s, want %s",
			testName, vpErr.ErrorCode, wantErrCode)
	}
}

// TstSetUp creates and returns a waddrmgr.Manager and a votingpool.Pool, each with their
// own walletdb namespace. It also returns a teardown function that closes the Manager and
// removes the directory created here to store the database.
// XXX: This should be renamed to TstCreatePool or something like that.
func TstSetUp(t *testing.T) (tearDownFunc func(), mgr *waddrmgr.Manager, pool *Pool) {
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

func TstUnlockManager(t *testing.T, mgr *waddrmgr.Manager) {
	if err := mgr.Unlock(privPassphrase); err != nil {
		t.Fatal(err)
	}
}

func createMasterKey(t *testing.T, seed []byte) *hdkeychain.ExtendedKey {
	key, err := hdkeychain.NewMaster(seed)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func createEmpoweredSeries(t *testing.T, pool *Pool) uint32 {
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
	TstUnlockManager(t, pool.Manager())
	defer pool.Manager().Lock()
	for _, key := range rawPrivKeys {
		if err := pool.EmpowerSeries(seriesID, key); err != nil {
			t.Fatal(err)
		}
	}
	return seriesID
}

// XXX: This needs a better name.
func TstCreateCredits(
	t *testing.T, pool *Pool, amounts []int64, store *txstore.Store) []CreditInterface {
	seriesID := createEmpoweredSeries(t, pool)

	// Finally create the Credit instances, locked to the voting pool's deposit
	// address with branch==1, index==0.
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

var (
	// seed is the master seed used to create extended keys.
	seed           = bytes.Repeat([]byte{0x2a, 0x64, 0xdf, 0x08}, 8)
	pubPassphrase  = []byte("_DJr{fL4H0O}*-0\n:V1izc)(6BomK")
	privPassphrase = []byte("81lUHXnOMZ@?XXd7O9xyDIWIbXX-lj")
)

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
