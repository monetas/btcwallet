package chroma_test

import (
	"fmt"
	"testing"

	"github.com/monetas/btcnet"
	"github.com/monetas/btcutil"
	"github.com/monetas/btcwallet/chroma"
	"github.com/monetas/btcwallet/waddrmgr"
	"github.com/monetas/btcwallet/walletdb"
	"github.com/monetas/btcwire"
	"github.com/monetas/gochroma"
)

var fastScrypt = &waddrmgr.Options{
	ScryptN: 16,
	ScryptR: 8,
	ScryptP: 1,
}

func TestCreateAndLoad(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
	defer db.Close()
	mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
	if err != nil {
		t.Fatalf("Failed to create addr manager DB namespace: %v", err)
	}
	seed := make([]byte, 32)
	mgr, err := waddrmgr.Create(mgrNamespace, seed, []byte("test"),
		[]byte("test"), &btcnet.MainNetParams, fastScrypt)
	if err != nil {
		t.Fatalf("Failed to create addr manager: %v", err)
	}
	defer mgr.Close()
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}

	// execute
	w, err := chroma.Create(chromaNamespace, mgr, nil)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("Chroma Wallet closing failed: %v", err)
	}
	w2, err := chroma.Load(chromaNamespace, mgr)
	if err != nil {
		t.Fatalf("Chroma Wallet load failed: %v", err)
	}
	err = w2.Close()
	if err != nil {
		t.Fatalf("Chroma Wallet closing failed: %v", err)
	}
}

func TestFetchColorId(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
	defer db.Close()
	mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
	if err != nil {
		t.Fatalf("Failed to create addr manager DB namespace: %v", err)
	}
	seed := make([]byte, 32)
	mgr, err := waddrmgr.Create(mgrNamespace, seed, []byte("test"),
		[]byte("test"), &btcnet.MainNetParams, fastScrypt)
	if err != nil {
		t.Fatalf("Failed to create addr manager: %v", err)
	}
	defer mgr.Close()
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	defer w.Close()
	cdStr := "EPOBC:00000000000000000000000000000000:0:1"
	cd, err := gochroma.NewColorDefinitionFromStr(cdStr)
	if err != nil {
		t.Fatalf("Definition creation failed: %v", err)
	}

	// execute
	cid, err := w.FetchColorId(cd)
	if err != nil {
		t.Fatalf("Chroma Wallet color adding failed: %v", err)
	}

	// validate
	if cid[0] != 1 {
		t.Fatalf("Chroma Wallet color id different than expected: want %v, got %v", 1, cid[0])
	}
}

func TestNewAddress(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
	defer db.Close()
	mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
	if err != nil {
		t.Fatalf("Failed to create addr manager DB namespace: %v", err)
	}
	seed := make([]byte, 32)
	mgr, err := waddrmgr.Create(mgrNamespace, seed, []byte("test"),
		[]byte("test"), &btcnet.MainNetParams, fastScrypt)
	if err != nil {
		t.Fatalf("Failed to create addr manager: %v", err)
	}
	defer mgr.Close()
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	defer w.Close()
	cdStr := "EPOBC:00000000000000000000000000000000:0:1"
	cd, err := gochroma.NewColorDefinitionFromStr(cdStr)
	if err != nil {
		t.Fatalf("Definition creation failed: %v", err)
	}
	_, err = w.FetchColorId(cd)
	if err != nil {
		t.Fatalf("Chroma Wallet color adding failed: %v", err)
	}

	// execute
	uAddr, err := w.NewUncoloredAddress()
	if err != nil {
		t.Fatalf("Cannot get uncolored addr: %v", err)
	}
	iAddr, err := w.NewIssuingAddress()
	if err != nil {
		t.Fatalf("Cannot get issuing addr: %v", err)
	}
	cAddr, err := w.NewColorAddress(cd)
	if err != nil {
		t.Fatalf("Cannot get color addr: %v", err)
	}

	// validate
	got := uAddr.String()
	want := "1PsDrx5SoYdDqEiBAJgszq2nUryjBunApJ"
	if got != want {
		t.Fatalf("unexpected addr: want %v, got %v", want, got)
	}
	got = iAddr.String()
	want = "154qYY2tRoDytLBYVTJZWRUMgZZ1sBQBaE"
	if got != want {
		t.Fatalf("unexpected addr: want %v, got %v", want, got)
	}
	got = cAddr.String()
	want = "1DGraPworg3JGNVRFRMgZ6AYmenwdTo3ay"
	if got != want {
		t.Fatalf("unexpected addr: want %v, got %v", want, got)
	}
}

func TestNewUncoloredOutPoint(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
	defer db.Close()
	mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
	if err != nil {
		t.Fatalf("Failed to create addr manager DB namespace: %v", err)
	}
	seed := make([]byte, 32)
	mgr, err := waddrmgr.Create(mgrNamespace, seed, []byte("test"),
		[]byte("test"), &btcnet.MainNetParams, fastScrypt)
	if err != nil {
		t.Fatalf("Failed to create addr manager: %v", err)
	}
	defer mgr.Close()
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	defer w.Close()
	blockReaderWriter := &TstBlockReaderWriter{
		rawTx: [][]byte{normalTx},
	}
	b := &gochroma.BlockExplorer{blockReaderWriter}
	shaHash, err := gochroma.NewShaHash(txHash)
	if err != nil {
		t.Fatalf("failed to convert hash %v: %v", txHash, err)
	}
	outPoint := btcwire.NewOutPoint(shaHash, 0)

	// execute
	cop, err := w.NewUncoloredOutPoint(b, outPoint)
	if err != nil {
		t.Fatal(err)
	}

	// Verify
	value := cop.ColorValue
	wantValue := gochroma.ColorValue(100000000)
	if value != wantValue {
		t.Fatalf("Did not get value that we expected: got %d, want %d", value, wantValue)
	}
	outPointGot, err := cop.OutPoint()
	if err != nil {
		t.Fatal(err)
	}
	if outPointGot.Index != outPoint.Index {
		t.Fatalf("Did not get value that we expected: got %d, want %d", outPointGot.Index, outPoint.Index)
	}
}

func TestNewColorOutPoint(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
	defer db.Close()
	mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
	if err != nil {
		t.Fatalf("Failed to create addr manager DB namespace: %v", err)
	}
	seed := make([]byte, 32)
	mgr, err := waddrmgr.Create(mgrNamespace, seed, []byte("test"),
		[]byte("test"), &btcnet.MainNetParams, fastScrypt)
	if err != nil {
		t.Fatalf("Failed to create addr manager: %v", err)
	}
	defer mgr.Close()
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	defer w.Close()
	blockReaderWriter := &TstBlockReaderWriter{
		txBlockHash: [][]byte{blockHash},
		block:       [][]byte{rawBlock},
		rawTx:       [][]byte{genesisTx, genesisTx},
		txOutSpents: []bool{false},
	}
	b := &gochroma.BlockExplorer{blockReaderWriter}
	tx, err := btcutil.NewTxFromBytes(genesisTx)
	if err != nil {
		t.Fatalf("failed to get tx %v", err)
	}
	outPoint := btcwire.NewOutPoint(tx.Sha(), 0)
	shaBytes := gochroma.BigEndianBytes(tx.Sha())
	txString := fmt.Sprintf("%x", shaBytes)
	colorStr := "SPOBC:" + txString + ":0:1"
	cd, err := gochroma.NewColorDefinitionFromStr(colorStr)
	if err != nil {
		t.Fatal(err)
	}

	// execute
	cop, err := w.NewColorOutPoint(b, outPoint, cd)
	if err != nil {
		t.Fatal(err)
	}

	// Verify
	wantValue := gochroma.ColorValue(1)
	if cop.ColorValue != wantValue {
		t.Fatalf("Did not get value that we expected: got %d, want %d", cop.ColorValue, wantValue)
	}
	cin, err := cop.ColorIn()
	if err != nil {
		t.Fatal(err)
	}
	if cin.ColorValue != wantValue {
		t.Fatalf("Did not get value that we expected: got %d, want %d", cin.ColorValue, wantValue)
	}
}

func TestIssueColor(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
	defer db.Close()
	mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
	if err != nil {
		t.Fatalf("Failed to create addr manager DB namespace: %v", err)
	}
	seed := make([]byte, 32)
	mgr, err := waddrmgr.Create(mgrNamespace, seed, []byte("test"),
		[]byte("test"), &btcnet.MainNetParams, fastScrypt)
	if err != nil {
		t.Fatalf("Failed to create addr manager: %v", err)
	}
	defer mgr.Close()
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	defer w.Close()
	txHash2 := make([]byte, 32)
	blockReaderWriter := &TstBlockReaderWriter{
		block:       [][]byte{rawBlock},
		txBlockHash: [][]byte{blockHash},
		rawTx:       [][]byte{specialTx, specialTx, specialTx, specialTx, specialTx, specialTx, specialTx},
		txOutSpents: []bool{false, false, false},
		sendHash:    [][]byte{txHash2},
	}
	b := &gochroma.BlockExplorer{blockReaderWriter}
	shaHash, err := gochroma.NewShaHash(txHash)
	if err != nil {
		t.Fatalf("failed to convert hash %v: %v", txHash, err)
	}
	outPoint := btcwire.NewOutPoint(shaHash, 0)
	_, err = w.NewUncoloredOutPoint(b, outPoint)
	if err != nil {
		t.Fatal(err)
	}
	kernelCode := "EPOBC"
	kernel, err := gochroma.GetColorKernel(kernelCode)
	if err != nil {
		t.Fatal(err)
	}
	value := gochroma.ColorValue(1000)

	// execute
	cd, err := w.IssueColor(b, kernel, value, int64(10000))
	if err != nil {
		t.Fatal(err)
	}

	// validate
	gotStr := cd.HashString()
	wantStr := fmt.Sprintf("%v:%x:0", kernelCode, txHash2)
	if gotStr != wantStr {
		t.Fatalf("color definition different than expected: want %v, got %v",
			wantStr, gotStr)
	}
}

func TestColorBalance(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
	defer db.Close()
	mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
	if err != nil {
		t.Fatalf("Failed to create addr manager DB namespace: %v", err)
	}
	seed := make([]byte, 32)
	mgr, err := waddrmgr.Create(mgrNamespace, seed, []byte("test"),
		[]byte("test"), &btcnet.MainNetParams, fastScrypt)
	if err != nil {
		t.Fatalf("Failed to create addr manager: %v", err)
	}
	defer mgr.Close()
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	defer w.Close()
	blockReaderWriter := &TstBlockReaderWriter{
		txBlockHash: [][]byte{blockHash},
		block:       [][]byte{rawBlock},
		rawTx:       [][]byte{genesisTx, genesisTx},
		txOutSpents: []bool{false},
	}
	b := &gochroma.BlockExplorer{blockReaderWriter}
	tx, err := btcutil.NewTxFromBytes(genesisTx)
	if err != nil {
		t.Fatalf("failed to get tx %v", err)
	}
	outPoint := btcwire.NewOutPoint(tx.Sha(), 0)
	shaBytes := gochroma.BigEndianBytes(tx.Sha())
	txString := fmt.Sprintf("%x", shaBytes)
	colorStr := "SPOBC:" + txString + ":0:1"
	cd, err := gochroma.NewColorDefinitionFromStr(colorStr)
	if err != nil {
		t.Fatal(err)
	}
	cid, err := w.FetchColorId(cd)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.NewColorOutPoint(b, outPoint, cd)
	if err != nil {
		t.Fatal(err)
	}

	// execute
	balance, err := w.ColorBalance(cid)
	if err != nil {
		t.Fatal(err)
	}

	// validate
	want := gochroma.ColorValue(1)
	if *balance != want {
		t.Fatalf("balance not what we wanted: want %v, got %v", want, balance)
	}
}

func TestAllColors(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
	defer db.Close()
	mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
	if err != nil {
		t.Fatalf("Failed to create addr manager DB namespace: %v", err)
	}
	seed := make([]byte, 32)
	mgr, err := waddrmgr.Create(mgrNamespace, seed, []byte("test"),
		[]byte("test"), &btcnet.MainNetParams, fastScrypt)
	if err != nil {
		t.Fatalf("Failed to create addr manager: %v", err)
	}
	defer mgr.Close()
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	defer w.Close()
	num := 10
	for i := 0; i < num; i++ {
		var kernel string
		if i < 5 {
			kernel = "SPOBC"
		} else {
			kernel = "EPOBC"
		}
		defStr := fmt.Sprintf("%v:%x:0:1", kernel, seed)
		cd, err := gochroma.NewColorDefinitionFromStr(defStr)
		if err != nil {
			t.Fatal(err)
		}
		_, err = w.FetchColorId(cd)
		seed[0] += 1
	}

	// execute
	cds, err := w.AllColors()
	if err != nil {
		t.Fatal(err)
	}

	// validate
	if len(cds) != num {
		t.Fatalf("different number of color defs: want %d, got %d", num, len(cds))
	}
}

func TestSend(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
	defer db.Close()
	mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
	if err != nil {
		t.Fatalf("Failed to create addr manager DB namespace: %v", err)
	}
	seed := make([]byte, 32)
	mgr, err := waddrmgr.Create(mgrNamespace, seed, []byte("test"),
		[]byte("test"), &btcnet.MainNetParams, fastScrypt)
	if err != nil {
		t.Fatalf("Failed to create addr manager: %v", err)
	}
	defer mgr.Close()
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	defer w.Close()
	txHash2 := make([]byte, 32)
	txHash3 := make([]byte, 32)
	txHash4 := make([]byte, 32)
	txHash3[0] = 1
	txHash4[0] = 2
	blockReaderWriter := &TstBlockReaderWriter{
		txBlockHash: [][]byte{blockHash},
		block:       [][]byte{rawBlock},
		rawTx:       [][]byte{specialTx, specialTx, specialTx, niceTx, niceTx, niceTx, niceTx, niceTx, niceTx, niceTx, niceTx, niceTx, niceTx},
		txOutSpents: []bool{false, false, false, false, false, false, false, false},
		sendHash:    [][]byte{txHash2, txHash3},
	}
	b := &gochroma.BlockExplorer{blockReaderWriter}
	shaHash, err := gochroma.NewShaHash(txHash)
	if err != nil {
		t.Fatalf("failed to convert hash %v: %v", txHash, err)
	}
	outPoint := btcwire.NewOutPoint(shaHash, 0)
	_, err = w.NewUncoloredOutPoint(b, outPoint)
	if err != nil {
		t.Fatal(err)
	}
	shaHash, err = gochroma.NewShaHash(txHash4)
	if err != nil {
		t.Fatalf("failed to convert hash %v: %v", txHash, err)
	}
	outPoint = btcwire.NewOutPoint(shaHash, 0)
	_, err = w.NewUncoloredOutPoint(b, outPoint)
	if err != nil {
		t.Fatal(err)
	}
	kernelCode := "SPOBC"
	kernel, err := gochroma.GetColorKernel(kernelCode)
	if err != nil {
		t.Fatal(err)
	}
	value := gochroma.ColorValue(1)
	cd, err := w.IssueColor(b, kernel, value, int64(10000))
	if err != nil {
		t.Fatal(err)
	}
	addr, err := w.NewColorAddress(cd)
	if err != nil {
		t.Fatal(err)
	}
	value2 := gochroma.ColorValue(1)
	addrMap := map[btcutil.Address]gochroma.ColorValue{addr: value2}
	fee := int64(100)

	// execute
	tx, err := w.Send(b, cd, addrMap, fee)

	// validate
	if err != nil {
		t.Fatal(err)
	}
	if len(tx.TxIn) != 2 {
		t.Fatalf("expected more inputs: want %d, got %d", 2, len(tx.TxIn))
	}
	if len(tx.TxOut) != 2 {
		t.Fatalf("expected more outputs: want %d, got %d", 2, len(tx.TxOut))
	}
	spobc := kernel.(*gochroma.SPOBC)
	want := spobc.MinimumSatoshi
	if tx.TxOut[0].Value != want {
		t.Fatalf("unexpected output at 0: want %d, got %d", want, tx.TxOut[0].Value)
	}
	want = 20000 - want - fee
	if tx.TxOut[1].Value != want {
		t.Fatalf("unexpected output at 1: want %d, got %d", want, tx.TxOut[1].Value)
	}
}
