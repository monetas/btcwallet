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

func TestCreateError(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}

	// execute
	_, err = chroma.Create(chromaNamespace, mgr, []byte("nonsense"))

	if err == nil {
		t.Fatalf("Expected error, got none")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrHDKey)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestLoadError(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	_, err = chroma.Create(chromaNamespace, mgr, nil)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	testNS := chromaNamespace.(*chroma.TstNamespace)
	b := testNS.Tx.RootBucket().Bucket(chroma.KeyBucketName)
	err = b.Put(chroma.PrivKeyName, []byte("nonsense"))
	if err != nil {
		t.Fatalf("Chroma bucket update failed: %v", err)
	}

	// execute
	_, err = chroma.Load(chromaNamespace, mgr)

	// validate
	if err == nil {
		t.Fatalf("Expected error, got none")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrHDKey)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestNewAddressError(t *testing.T) {

	tests := []struct {
		desc string
		acct uint32
		err  chroma.ErrorCode
	}{
		{
			desc: "child key too big",
			acct: 1<<31 + 1,
			err:  chroma.ErrHDKey,
		},
		{
			desc: "account does not exist",
			acct: 2,
			err:  chroma.ErrAcct,
		},
		{
			desc: "sub key too big",
			acct: 0,
			err:  chroma.ErrHDKey,
		},
		{
			desc: "error on store",
			acct: 1,
			err:  chroma.ErrWriteDB,
		},
	}

	for _, test := range tests {

		// setup
		db, err := walletdb.Create("test")
		if err != nil {
			t.Fatalf("Failed to create wallet DB: %v", err)
		}
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
		chromaNamespace, err := db.Namespace([]byte("chroma"))
		if err != nil {
			t.Fatalf("Failed to create Chroma DB namespace: %v", err)
		}
		w, err := chroma.Create(chromaNamespace, mgr, seed)
		if err != nil {
			t.Fatalf("Chroma Wallet creation failed: %v", err)
		}
		testNS := chromaNamespace.(*chroma.TstNamespace)
		b := testNS.Tx.RootBucket().Bucket(chroma.AccountBucketName)
		err = b.Put(chroma.SerializeUint32(0), chroma.SerializeUint32(1<<31+1))
		if err != nil {
			t.Fatalf("Couldn't do a put", err)
		}
		tstBucket := b.(*chroma.TstBucket)
		tstBucket.ErrorAfter = 1

		// execute
		_, err = w.NewAddress(test.acct)

		// validate
		if err == nil {
			t.Fatalf("%v: Expected error, got none", test.desc)
		}
		rerr := err.(chroma.ChromaError)
		if rerr.ErrorCode != test.err {
			t.Fatalf("want %v, got %v", test.err, err)
		}

	}
}

func TestFetchColorId(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
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

func TestFetchColorIdError(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	cdStr := "EPOBC:00000000000000000000000000000000:0:1"
	cd, err := gochroma.NewColorDefinitionFromStr(cdStr)
	if err != nil {
		t.Fatalf("Definition creation failed: %v", err)
	}
	testNS := chromaNamespace.(*chroma.TstNamespace)
	b := testNS.Tx.RootBucket().Bucket(chroma.ColorDefinitionBucketName)
	tstBucket := b.(*chroma.TstBucket)
	tstBucket.ErrorAfter = 0

	// execute
	_, err = w.FetchColorId(cd)

	// validate
	if err == nil {
		t.Fatalf("expected err, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrWriteDB)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestNewAddress(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
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

	// validate
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

func TestNewUncoloredOutPointError1(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	blockReaderWriter := &TstBlockReaderWriter{
		rawTx: [][]byte{normalTx},
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

	// execute
	_, err = w.NewUncoloredOutPoint(b, outPoint)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrOutPointExists)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestNewUncoloredOutPointError2(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	blockReaderWriter := &TstBlockReaderWriter{}
	b := &gochroma.BlockExplorer{blockReaderWriter}
	shaHash, err := gochroma.NewShaHash(txHash)
	if err != nil {
		t.Fatalf("failed to convert hash %v: %v", txHash, err)
	}
	outPoint := btcwire.NewOutPoint(shaHash, 0)

	// execute
	_, err = w.NewUncoloredOutPoint(b, outPoint)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrBlockExplorer)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestNewUncoloredOutPointError3(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
	blockReaderWriter := &TstBlockReaderWriter{
		rawTx: [][]byte{normalTx},
	}
	b := &gochroma.BlockExplorer{blockReaderWriter}
	shaHash, err := gochroma.NewShaHash(txHash)
	if err != nil {
		t.Fatalf("failed to convert hash %v: %v", txHash, err)
	}
	outPoint := btcwire.NewOutPoint(shaHash, 0)
	testNS := chromaNamespace.(*chroma.TstNamespace)
	bucket := testNS.Tx.RootBucket().Bucket(chroma.ColorOutPointBucketName)
	tstBucket := bucket.(*chroma.TstBucket)
	tstBucket.ErrorAfter = 0

	// execute
	_, err = w.NewUncoloredOutPoint(b, outPoint)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrWriteDB)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestNewColorOutPoint(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
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

	// validate
	wantValue := gochroma.ColorValue(1)
	if cop.ColorValue != wantValue {
		t.Fatalf("unexpected value: got %d, want %d", cop.ColorValue, wantValue)
	}
	cin, err := cop.ColorIn()
	if err != nil {
		t.Fatal(err)
	}
	if cin.ColorValue != wantValue {
		t.Fatalf("Did not get value that we expected: got %d, want %d", cin.ColorValue, wantValue)
	}
}

func TestNewColorOutPointError(t *testing.T) {
	tests := []struct {
		desc   string
		rawTx  [][]byte
		bucket []byte
		err    chroma.ErrorCode
	}{
		{
			desc:   "fail on outpoint creation",
			rawTx:  [][]byte{},
			bucket: chroma.IdBucketName,
			err:    chroma.ErrWriteDB,
		},
		{
			desc:   "fail on color id fetch",
			rawTx:  [][]byte{genesisTx, genesisTx},
			bucket: chroma.ColorDefinitionBucketName,
			err:    chroma.ErrWriteDB,
		},
		{
			desc:   "fail on color value fetch",
			rawTx:  [][]byte{genesisTx},
			bucket: chroma.ScriptToAccountIndexBucketName,
			err:    chroma.ErrColor,
		},
		{
			desc:   "fail on color outpoint store",
			rawTx:  [][]byte{genesisTx, genesisTx},
			bucket: chroma.ColorOutPointBucketName,
			err:    chroma.ErrWriteDB,
		},
	}

	for _, test := range tests {
		// setup
		db, err := walletdb.Create("test")
		if err != nil {
			t.Errorf("%v: Failed to create wallet DB: %v", test.desc, err)
			continue
		}
		mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
		if err != nil {
			t.Errorf("%v: Failed to create addr manager DB namespace: %v", test.desc, err)
			continue
		}
		seed := make([]byte, 32)
		mgr, err := waddrmgr.Create(mgrNamespace, seed, []byte("test"),
			[]byte("test"), &btcnet.MainNetParams, fastScrypt)
		if err != nil {
			t.Errorf("%v: Failed to create addr manager: %v", test.desc, err)
			continue
		}
		chromaNamespace, err := db.Namespace([]byte("chroma"))
		if err != nil {
			t.Errorf("%v: Failed to create Chroma DB namespace: %v", test.desc, err)
			continue
		}
		w, err := chroma.Create(chromaNamespace, mgr, seed)
		if err != nil {
			t.Errorf("%v: Chroma Wallet creation failed: %v", test.desc, err)
			continue
		}
		blockReaderWriter := &TstBlockReaderWriter{
			txBlockHash: [][]byte{blockHash},
			block:       [][]byte{rawBlock},
			rawTx:       test.rawTx,
			txOutSpents: []bool{false},
		}
		b := &gochroma.BlockExplorer{blockReaderWriter}
		tx, err := btcutil.NewTxFromBytes(genesisTx)
		if err != nil {
			t.Errorf("%v: failed to get tx %v", test.desc, err)
			continue
		}
		outPoint := btcwire.NewOutPoint(tx.Sha(), 0)
		shaBytes := gochroma.BigEndianBytes(tx.Sha())
		txString := fmt.Sprintf("%x", shaBytes)
		colorStr := "SPOBC:" + txString + ":0:1"
		cd, err := gochroma.NewColorDefinitionFromStr(colorStr)
		if err != nil {
			t.Error(err)
			continue
		}
		testNS := chromaNamespace.(*chroma.TstNamespace)
		bucket := testNS.Tx.RootBucket().Bucket(test.bucket)
		tstBucket := bucket.(*chroma.TstBucket)
		tstBucket.ErrorAfter = 0

		// execute
		_, err = w.NewColorOutPoint(b, outPoint, cd)

		// validate
		if err == nil {
			t.Errorf("%v: expected error, got nil", test.desc)
			continue
		}
		rerr := err.(chroma.ChromaError)
		want := chroma.ErrorCode(test.err)
		if rerr.ErrorCode != want {
			t.Errorf("%v: want %v, got %v", test.desc, want, err)
			continue
		}
	}
}

func TestSignError(t *testing.T) {
	tests := []struct {
		desc   string
		lookup []byte
		script []byte
		acct   uint32
		index  uint32
		err    chroma.ErrorCode
	}{
		{
			desc:   "fail on lookup",
			lookup: nil,
			script: []byte("nonsense"),
			acct:   0,
			index:  0,
			err:    chroma.ErrScript,
		},
		{
			desc:   "fail on lookup",
			lookup: []byte{1},
			script: []byte{1},
			acct:   0,
			index:  0,
			err:    chroma.ErrScript,
		},
	}

	for _, test := range tests {
		// setup
		db, err := walletdb.Create("test")
		if err != nil {
			t.Errorf("%v: Failed to create wallet DB: %v", test.desc, err)
			continue
		}
		mgrNamespace, err := db.Namespace([]byte("waddrmgr"))
		if err != nil {
			t.Errorf("%v: Failed to create addr manager DB namespace: %v", test.desc, err)
			continue
		}
		seed := make([]byte, 32)
		mgr, err := waddrmgr.Create(mgrNamespace, seed, []byte("test"),
			[]byte("test"), &btcnet.MainNetParams, fastScrypt)
		if err != nil {
			t.Errorf("%v: Failed to create addr manager: %v", test.desc, err)
			continue
		}
		chromaNamespace, err := db.Namespace([]byte("chroma"))
		if err != nil {
			t.Errorf("%v: Failed to create Chroma DB namespace: %v", test.desc, err)
			continue
		}
		w, err := chroma.Create(chromaNamespace, mgr, seed)
		if err != nil {
			t.Errorf("%v: Chroma Wallet creation failed: %v", test.desc, err)
			continue
		}
		tx, err := btcutil.NewTxFromBytes(genesisTx)
		if err != nil {
			t.Errorf("%v: failed to get tx %v", test.desc, err)
			continue
		}
		acct := chroma.SerializeUint32(test.acct)
		index := chroma.SerializeUint32(test.index)
		testNS := chromaNamespace.(*chroma.TstNamespace)
		bucket := testNS.Tx.RootBucket().Bucket(chroma.ScriptToAccountIndexBucketName)
		err = bucket.Put(test.script, append(acct, index...))
		if err != nil {
			t.Errorf("%v: failed to put script %v", test.desc, err)
			continue
		}

		// execute
		err = w.Sign(test.lookup, tx.MsgTx(), 0)

		// validate
		if err == nil {
			t.Errorf("%v: Expected error, got nil", test.desc)
			continue
		}
		rerr := err.(chroma.ChromaError)
		want := chroma.ErrorCode(test.err)
		if rerr.ErrorCode != want {
			t.Errorf("%v: want %v, got %v", test.desc, want, err)
			continue
		}
	}
}

func TestIssueColor(t *testing.T) {
	// setup
	db, err := walletdb.Create("test")
	if err != nil {
		t.Fatalf("Failed to create wallet DB: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
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
	chromaNamespace, err := db.Namespace([]byte("chroma"))
	if err != nil {
		t.Fatalf("Failed to create Chroma DB namespace: %v", err)
	}
	w, err := chroma.Create(chromaNamespace, mgr, seed)
	if err != nil {
		t.Fatalf("Chroma Wallet creation failed: %v", err)
	}
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
