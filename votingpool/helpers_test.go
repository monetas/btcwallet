package votingpool_test

import (
	"testing"

	"github.com/conformal/btcnet"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/votingpool"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

var activeNet = &btcnet.TestNet3Params

func createInputs(t *testing.T, pkScript []byte, amounts []int64) ([]txstore.Credit, *txstore.Store) {
	msgTx := createMsgTx(pkScript, amounts)

	s := txstore.New("/tmp/tx.bin")

	// XXX: This duplicates the stuff lars did in one of his branches
	block := &txstore.Block{Height: int32(10)} // XXX: Hard-coded value warning
	tx := btcutil.NewTx(msgTx)
	tx.SetIndex(1) // XXX: Hard-coded value warning

	r, err := s.InsertTx(tx, block)
	if err != nil {
		t.Fatal(err)
	}

	eligible := make([]txstore.Credit, len(msgTx.TxOut))
	for i := range msgTx.TxOut {
		credit, err := r.AddCredit(uint32(i), false)
		if err != nil {
			t.Fatal(err)
		}
		eligible[i] = credit
	}
	return eligible, s
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

func createVotingPoolPkScript(t *testing.T, mgr *waddrmgr.Manager, pool *votingpool.VotingPool, bsHeight int32, series, branch, index uint32) []byte {
	script, err := pool.DepositScript(series, branch, index)
	if err != nil {
		t.Fatalf("Failed to create depositscript for series %d, branch %d, index %d: %v", series, branch, index, err)
	}

	if err = mgr.Unlock(privPassphrase); err != nil {
		t.Fatalf("Failed to unlock the address manager: %v", err)
	}
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
