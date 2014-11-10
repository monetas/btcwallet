package votingpool_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/votingpool"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

// createInputs is a convenience function.  See createInputsOnBlock
// for a more flexible version.
func createInputs(t *testing.T, store *txstore.Store, pkScript []byte, amounts []int64) []txstore.Credit {
	blockTxIndex := 1 // XXX: hardcoded value.
	blockHeight := 10 // XXX: hardcoded value.
	return createInputsOnBlock(t, store, blockTxIndex, blockHeight, pkScript, amounts)
}

// createInputOnBlock creates a number of inputs by creating a
// transaction with a number of outputs corresponding to the elements
// of the amounts slice.
//
// The transaction is added to a block and the index and and
// blockheight must be specified.
func createInputsOnBlock(t *testing.T, s *txstore.Store,
	blockTxIndex, blockHeight int,
	pkScript []byte, amounts []int64) []txstore.Credit {
	msgTx := createMsgTx(pkScript, amounts)
	block := &txstore.Block{
		Height: int32(blockHeight),
	}

	tx := btcutil.NewTx(msgTx)
	tx.SetIndex(blockTxIndex)

	r, err := s.InsertTx(tx, block)
	if err != nil {
		t.Fatal("createInputsStore, InserTx failed: ", err)
	}

	credits := make([]txstore.Credit, len(msgTx.TxOut))
	for i := range msgTx.TxOut {
		credit, err := r.AddCredit(uint32(i), false)
		if err != nil {
			t.Fatal("createInputsStore, AddCredit failed: ", err)
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

func createVotingPoolPkScript(t *testing.T, mgr *waddrmgr.Manager, pool *votingpool.VotingPool, series uint32, branch votingpool.Branch, index votingpool.Index) []byte {
	script, err := pool.DepositScript(series, branch, index)
	if err != nil {
		t.Fatalf("Failed to create depositscript for series %d, branch %d, index %d: %v", series, branch, index, err)
	}

	if err = mgr.Unlock(privPassphrase); err != nil {
		t.Fatalf("Failed to unlock the address manager: %v", err)
	}
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

func importPrivateKeys(t *testing.T, mgr *waddrmgr.Manager, privKeys []string, bs *waddrmgr.BlockStamp) {
	// XXX: Should we lock the manager once we're done importing?
	if err := mgr.Unlock(privPassphrase); err != nil {
		t.Fatal(err)
	}

	for _, key := range privKeys {
		wif, err := btcutil.DecodeWIF(key)
		if err != nil {
			t.Fatal(err)
		}

		_, err = mgr.ImportPrivateKey(wif, bs)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func createTxStore(t *testing.T) (store *txstore.Store, tearDown func()) {
	dir, err := ioutil.TempDir("", "tx.bin")
	if err != nil {
		t.Fatalf("Failed to create db file: %v", err)
	}
	s := txstore.New(dir)
	return s, func() { os.RemoveAll(dir) }
}

type seriesDef struct {
	reqSigs  uint32
	pubKeys  []string
	seriesID uint32
}

func createSeries(t *testing.T, pool *votingpool.VotingPool,
	definitions []seriesDef) {

	for _, def := range definitions {
		if err := pool.CreateSeries(version, def.seriesID, def.reqSigs, def.pubKeys); err != nil {
			t.Fatalf("Cannot creates series %d: %v", def.seriesID, err)
		}
	}
}

func createPkScripts(t *testing.T, mgr *waddrmgr.Manager,
	pool *votingpool.VotingPool, aRange votingpool.AddressRange) [][]byte {

	var pkScripts [][]byte
	for index := aRange.StartIndex; index <= aRange.StopIndex; index++ {
		for branch := aRange.StartBranch; branch <= aRange.StopBranch; branch++ {

			pkScript := createVotingPoolPkScript(t, mgr, pool, aRange.SeriesID, branch, index)
			pkScripts = append(pkScripts, pkScript)
		}
	}
	return pkScripts
}
