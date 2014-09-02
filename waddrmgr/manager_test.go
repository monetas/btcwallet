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

package waddrmgr_test

import (
	"os"
	"testing"

	"github.com/monetas/btcnet"
	"github.com/monetas/btcutil"
	"github.com/monetas/btcwallet/waddrmgr"
)

var (
	// seed is the master seed used throughout the tests.
	seed = []byte{
		0x2a, 0x64, 0xdf, 0x08, 0x5e, 0xef, 0xed, 0xd8, 0xbf,
		0xdb, 0xb3, 0x31, 0x76, 0xb5, 0xba, 0x2e, 0x62, 0xe8,
		0xbe, 0x8b, 0x56, 0xc8, 0x83, 0x77, 0x95, 0x59, 0x8b,
		0xb6, 0xc4, 0x40, 0xc0, 0x64,
	}

	pubPassphrase  = []byte("pub")
	privPassphrase = []byte("priv")
)

// testContext is used to store context information about a running test which
// is passed into helper functions.  The useSpends field indicates whether or
// not the spend data should be empty or figure it out based on the specific
// test blocks provided.  This is needed because the first loop where the blocks
// are inserted, the tests are running against the latest block and therefore
// none of the outputs can be spent yet.  However, on subsequent runs, all
// blocks have been inserted and therefore some of the transaction outputs are
// spent.
type testContext struct {
	t             *testing.T
	manager       *waddrmgr.Manager
	account       uint32
	unlocked      bool
	uniqueCounter uint32
}

type expectedAddr struct {
	address     string
	addressHash []byte
	internal    bool
	compressed  bool
	imported    bool
	syncStatus  waddrmgr.SyncStatus
	privKey     string
}

func testAddress(tc *testContext, gotAddr, wantAddr waddrmgr.ManagedAddress) bool {
	//fmt.Printf("hash %x\n", addr.AddrHash())
	//fmt.Println("addr", addr.Address())
	//fmt.Println("internal", addr.Internal())
	//fmt.Println("compressed", addr.Compressed())
	//fmt.Println("imported", addr.Imported())
	//fmt.Println("syncstatus", addr.SyncStatus())

	//switch addr := addr.(type) {
	//case waddrmgr.ManagedPubKeyAddress:
	//	fmt.Print("eprv ")
	//	fmt.Println(addr.ExportPrivKey())
	//	fmt.Println("epub", addr.ExportPubKey())
	//	fmt.Print("priv ")
	//	fmt.Println(addr.PrivKey())
	//	fmt.Println("pubk", addr.PubKey())
	//case waddrmgr.ManagedScriptAddress:
	//	fmt.Print("script ")
	//	fmt.Println(btcscript.DisasmString(addr.Script()))
	//	fmt.Print("script class")
	//	fmt.Println(addr.ScriptClass())
	//	fmt.Print("addresses")
	//	fmt.Println(addr.Addresses())
	//	fmt.Print("required sigs")
	//	fmt.Println(addr.RequiredSigs())
	//}
	return true
}

func testNextExternalAddresses(tc *testContext) bool {
	addrs, err := tc.manager.NextExternalAddresses(tc.account, 5)
	if err != nil {
		tc.t.Errorf("NextExternalAddress (%d): unexpected "+
			"error: %v", tc.account, err)

	}

	for i := 0; i < len(addrs); i++ {
		testAddress(tc, addrs[i], addrs[i]) // TODO(davec): Fix...
	}

	return true
}

func testNextInternalAddresses(tc *testContext) bool {
	addrs, err := tc.manager.NextInternalAddresses(tc.account, 5)
	if err != nil {
		tc.t.Errorf("NextExternalAddress (%d): unexpected "+
			"error: %v", tc.account, err)

	}

	for i := 0; i < len(addrs); i++ {
		testAddress(tc, addrs[i], addrs[i]) // TODO(davec): Fix...
	}

	return true
}

func testManagerAPI(tc *testContext) {
	//testNextExternalAddresses(tc)
}

func TestCreate(t *testing.T) {
	// Create a new manager.
	mgrName := "mgrcreatetest.bin"
	os.Remove(mgrName)
	mgr, err := waddrmgr.Create(mgrName, seed, pubPassphrase, privPassphrase,
		&btcnet.MainNetParams)
	if err != nil {
		t.Errorf("Create: %v", err)
		return
	}
	defer os.Remove(mgrName)
	defer mgr.Close()

	// Ensure attempting to create an already existing manager gives error.
	wantErr := waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrAlreadyExists}
	_, err = waddrmgr.Create(mgrName, seed, pubPassphrase, privPassphrase,
		&btcnet.MainNetParams)
	merr, ok := err.(waddrmgr.ManagerError)
	if !ok {
		t.Errorf("Create: did not receive expected error type - "+
			"got %T, want %T", err, wantErr)
	} else if merr.ErrorCode != wantErr.ErrorCode {
		t.Errorf("Create: did not receive expected error code - "+
			"got %v, want %v", merr.ErrorCode, wantErr.ErrorCode)
	}

	// Perform all of the API tests against the created manager.
	testManagerAPI(&testContext{
		t:       t,
		manager: mgr,
		account: 0,
	})
}

func TestOpen(t *testing.T) {
	// Ensure attempting to open a nonexistent manager gives error.
	mgrName := "mgropentest.bin"
	wantErr := waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrNoExist}
	os.Remove(mgrName)
	_, err := waddrmgr.Open(mgrName, pubPassphrase, &btcnet.MainNetParams)
	merr, ok := err.(waddrmgr.ManagerError)
	if !ok {
		t.Errorf("Open: did not receive expected error type - "+
			"got %T, want %T", err, wantErr)
	} else if merr.ErrorCode != wantErr.ErrorCode {
		t.Errorf("Open: did not receive expected error code - "+
			"got %v, want %v", merr.ErrorCode, wantErr.ErrorCode)
	}

	// Create a new manager and immediately close it.
	os.Remove(mgrName)
	mgr, err := waddrmgr.Create(mgrName, seed, pubPassphrase, privPassphrase,
		&btcnet.MainNetParams)
	if err != nil {
		t.Errorf("Create: %v", err)
		return
	}
	defer os.Remove(mgrName)
	mgr.Close()

	// Open existing manager and repeat all manager tests against it.
	mgr, err = waddrmgr.Open(mgrName, pubPassphrase, &btcnet.MainNetParams)
	if err != nil {
		t.Errorf("Open: %v", err)
		return
	}
	defer mgr.Close()

	// Perform all of the API tests against the opened manager.
	testManagerAPI(&testContext{
		t:       t,
		manager: mgr,
		account: 0,
	})
}

func reverse(inKeys []*btcutil.AddressPubKey) []*btcutil.AddressPubKey {
	revKeys := make([]*btcutil.AddressPubKey, len(inKeys))
	max := len(inKeys)
	for i := range inKeys {
		revKeys[i] = inKeys[max-i-1]
	}
	return revKeys
}
