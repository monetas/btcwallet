package waddrmgr_test

import (
	"testing"

	"github.com/conformal/btcnet"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

var activeNet = &btcnet.TestNet3Params

func createInputs(t *testing.T, pkScript []byte, amounts []int64) []txstore.Credit {
	var inputs []txstore.Credit

	for _, amt := range amounts {
		input := createInput(t, pkScript, int64(amt))
		inputs = append(inputs, input...)
	}

	return inputs
}

func createInput(t *testing.T, pkScript []byte, amount int64) []txstore.Credit {
	tx := createMsgTx(pkScript, []int64{amount})
	eligible := inputsFromTx(t, tx, []uint32{0})
	return eligible
}

func createMsgTx(pkScript []byte, amts []int64) *btcutil.Tx {
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
	// This will create a TX with txIndex==TxIndexUnknown, which "is typically because
	// the transaction has not been inserted into a block". This doesn't seem to
	// be a problem for us but is worth noting.
	return btcutil.NewTx(msgtx)
}

func createVotingPoolPkScript(t *testing.T, mgr *waddrmgr.Manager, pool *waddrmgr.VotingPool, bsHeight int32, series, branch, index uint32) []byte {
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

func inputsFromTx(t *testing.T, tx *btcutil.Tx, indices []uint32) []txstore.Credit {
	s := txstore.New("/tmp/tx.bin")
	r, err := s.InsertTx(tx, nil)
	if err != nil {
		t.Fatal(err)
	}

	eligible := make([]txstore.Credit, len(indices))
	for i, idx := range indices {
		credit, err := r.AddCredit(idx, false)
		if err != nil {
			t.Fatal(err)
		}
		eligible[i] = credit
	}
	return eligible
}
