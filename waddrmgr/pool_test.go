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
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"testing"

	"github.com/monetas/btcnet"
	"github.com/monetas/btcutil"
	"github.com/monetas/btcutil/hdkeychain"
	"github.com/monetas/btcwallet/waddrmgr"
)

const (
	privKey0 = "xprv9s21ZrQH143K2j9PK4CXkCu8sgxkpUxCF7p1KVwiV5tdnkeYzJXReUkxz5iB2FUzTXC1L15abCDG4RMxSYT5zhm67uvsnLYxuDhZfoFcB6a"
	privKey1 = "xprv9s21ZrQH143K4PtW77ATQAKAGk7KAFFCzxFuAcWduoMEeQhCgWpuYWQvMGZknqdispUbgLZV1YPqFCbpzMJij8tSZ5xPSaZqPbchojeNuq7"
	privKey2 = "xprv9s21ZrQH143K27XboWxXZGU5j7VZ9SqVBnmMQPKTbddiWAhuNzeLynKHaZTAti6N454tVUUcvy6u15DfuW68NCBUxry6ZsHHzqoA8UtzdMn"
	privKey3 = "xprv9s21ZrQH143K2vb4DGQymRejLcZSksBHTYLxB7Stg1c7Lk9JxgEUGZTozwUKxoEWJPoGSdGnJY1TW7LNFQCWrpZjDdEXJeqJuDde6BmdD4P"
	privKey4 = "xprv9s21ZrQH143K4JNmRvWeLc1PggzusKcDYV1y8fAMNDdb9Rm5X1AvGHizxEdhTVR3sc62XvifC6dLAXMuQesX1y6999xnDwQ3aVno8KviU9d"
	privKey5 = "xprv9s21ZrQH143K3dxrqESqeHZ7pSwM6Uq77ssQADSBs7qdFs6dyRWmRcPyLUTQRpgB3EduNhJuWkCGG2LHjuUisw8KKfXJpPqYJ1MSPrZpe1z"
	privKey6 = "xprv9s21ZrQH143K2nE8ENAMNksTTVxPrMxFNWUuwThMy2bcH9LHTtQDXSNq2pTNcbuq36n5A3J9pbXVqnq5LDXvqniFRLN299kW7Svnxsx9tQv"
	privKey7 = "xprv9s21ZrQH143K3p93xF1oFeB6ey5ruUesWjuPxA9Z2R5wf6BLYfGXz7fg7NavWkQ2cx3Vm8w2HV9uKpSprNNHnenGeW9XhYDPSjwS9hyCs33"
	privKey8 = "xprv9s21ZrQH143K3WxnnvPZ8SDGXndASvLTFwMLBVzNCVgs9rzP6rXgW92DLvozdyBm8T9bSQvrFm1jMpTJrRE6w1KY5tshFeDk9Nn3K6V5FYX"

	pubKey0 = "xpub661MyMwAqRbcFDDrR5jY7LqsRioFDwg3cLjc7tML3RRcfYyhXqqgCH5SqMSQdpQ1Xh8EtVwcfm8psD8zXKPcRaCVSY4GCqbb3aMEs27GitE"
	pubKey1 = "xpub661MyMwAqRbcGsxyD8hTmJFtpmwoZhy4NBBVxzvFU8tDXD2ME49A6JjQCYgbpSUpHGP1q4S2S1Pxv2EqTjwfERS5pc9Q2yeLkPFzSgRpjs9"
	pubKey2 = "xpub661MyMwAqRbcEbc4uYVXvQQpH9L3YuZLZ1gxCmj59yAhNy33vXxbXadmRpx5YZEupNSqWRrR7PqU6duS2FiVCGEiugBEa5zuEAjsyLJjKCh"
	pubKey3 = "xpub661MyMwAqRbcFQfXKHwz8ZbTtePwAKu8pmGYyVrWEM96DYUTWDYipMnHrFcemZHn13jcRMfsNU3UWQUudiaE7mhkWCHGFRMavF167DQM4Va"
	pubKey4 = "xpub661MyMwAqRbcGnTEXx3ehjx8EiqQGnL4uhwZw3ZxvZAa2E6E4YVAp63UoVtvm2vMDDF8BdPpcarcf7PWcEKvzHhxzAYw1zG23C2egeh82AR"
	pubKey5 = "xpub661MyMwAqRbcG83KwFyr1RVrNUmqVwYxV6nzxbqoRTNc8fRnWxq1yQiTBifTHhevcEM9ucZ1TqFS7Kv17Gd81cesv6RDrrvYS9SLPjPXhV5"
	pubKey6 = "xpub661MyMwAqRbcFGJbLPhMjtpC1XntFpg6jjQWjr6yXN8b9wfS1RiU5EhJt5L7qoFuidYawc3XJoLjT2PcjVpXryS3hn1WmSPCyvQDNuKsfgM"
	pubKey7 = "xpub661MyMwAqRbcGJDX4GYocn7qCzvMJwNisxpzkYZAakcvXtWV6CanXuz9xdfe5kTptFMJ4hDt2iTiT11zyN14u8R5zLvoZ1gnEVqNLxp1r3v"
	pubKey8 = "xpub661MyMwAqRbcG13FtwvZVaA15pTerP4JdAGvytPykqDr2fKXePqw3wLhCALPAixsE176jFkc2ac9K3tnF4KwaTRKUqFF5apWD6XL9LHCu7E"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func setUp(t *testing.T) (tearDownFunc func(), mgr *waddrmgr.Manager, pool *waddrmgr.VotingPool) {
	t.Parallel()

	// Create a new manager.
	// We create the file and immediately delete it, as the waddrmgr
	// needs to be doing the creating.
	file, err := ioutil.TempDir("", "pool_test")
	if err != nil {
		t.Fatalf("Failed to create db file: %v", err)
	}
	os.Remove(file)
	mgr, err = waddrmgr.Create(file, seed, pubPassphrase, privPassphrase,
		&btcnet.MainNetParams)
	if err != nil {
		t.Fatalf("Failed to create Manager: %v", err)
	}
	pool, err = mgr.CreateVotingPool([]byte{0x00})
	if err != nil {
		t.Fatalf("Voting Pool creation failed: %v", err)
	}
	tearDownFunc = func() {
		os.Remove(file)
		mgr.Close()
	}
	return tearDownFunc, mgr, pool
}

func TestLoadVotingPoolAndDepositScript(t *testing.T) {
	tearDown, manager, _ := setUp(t)
	defer tearDown()

	// setup
	poolID := "test"
	pubKeys := []string{pubKey0, pubKey1, pubKey2}
	err := manager.LoadVotingPoolAndCreateSeries(poolID, 0, pubKeys, 2)
	if err != nil {
		t.Fatalf("Failed to create voting pool and series: %v", err)
	}

	// execute
	script, err := manager.LoadVotingPoolAndDepositScript(poolID, 0, 0, 0)
	if err != nil {
		t.Fatalf("Failed to get deposit script: %v", err)
	}

	// validate
	strScript := hex.EncodeToString(script)
	want := "5221035e94da75731a2153b20909017f62fcd49474c45f3b46282c0dafa8b40a3a312b2102e983a53dd20b7746dd100dfd2925b777436fc1ab1dd319433798924a5ce143e32102908d52a548ee9ef6b2d0ea67a3781a0381bc3570ad623564451e63757ff9393253ae"
	if want != strScript {
		t.Fatalf("Failed to get the right deposit script. Got %v, want %v",
			strScript, want)
	}
}

func TestLoadVotingPoolAndCreateSeries(t *testing.T) {
	tearDown, manager, _ := setUp(t)
	defer tearDown()

	poolID := "test"

	// first time, the voting pool is created
	pubKeys := []string{pubKey0, pubKey1, pubKey2}
	err := manager.LoadVotingPoolAndCreateSeries(poolID, 0, pubKeys, 2)
	if err != nil {
		t.Fatalf("Creating voting pool and Creating series failed: %v", err)
	}

	// create another series where the voting pool is loaded this time
	pubKeys = []string{pubKey3, pubKey4, pubKey5}
	err = manager.LoadVotingPoolAndCreateSeries(poolID, 1, pubKeys, 2)

	if err != nil {
		t.Fatalf("Loading voting pool and Creating series failed: %v", err)
	}
}

func TestLoadVotingPoolAndReplaceSeries(t *testing.T) {
	tearDown, manager, _ := setUp(t)
	defer tearDown()

	// setup
	poolID := "test"
	pubKeys := []string{pubKey0, pubKey1, pubKey2}
	err := manager.LoadVotingPoolAndCreateSeries(poolID, 0, pubKeys, 2)
	if err != nil {
		t.Fatalf("Failed to create voting pool and series: %v", err)
	}

	pubKeys = []string{pubKey3, pubKey4, pubKey5}
	err = manager.LoadVotingPoolAndReplaceSeries(poolID, 0, pubKeys, 2)
	if err != nil {
		t.Fatalf("Failed to replace series: %v", err)
	}
}

func TestLoadVotingPoolAndEmpowerSeries(t *testing.T) {
	tearDown, manager, _ := setUp(t)
	defer tearDown()

	// setup
	poolID := "test"
	pubKeys := []string{pubKey0, pubKey1, pubKey2}
	err := manager.LoadVotingPoolAndCreateSeries(poolID, 0, pubKeys, 2)
	if err != nil {
		t.Fatalf("Creating voting pool and Creating series failed: %v", err)
	}

	err = manager.LoadVotingPoolAndEmpowerSeries(poolID, 0, privKey0)
	if err != nil {
		t.Fatalf("Load voting pool and Empower series failed: %v", err)
	}
}

func TestDepositScriptAddress(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	tests := []struct {
		pubKeys []string
		series  uint32
		reqSigs uint32
		// map of branch:address (we only check the branch index at 0)
		addresses map[uint32]string
	}{
		{
			pubKeys: []string{pubKey0, pubKey1, pubKey2},
			series:  0,
			reqSigs: 2,
			addresses: map[uint32]string{
				0: "3Hb4xcebcKg4DiETJfwjh8sF4uDw9rqtVC",
				1: "34eVkREKgvvGASZW7hkgE2uNc1yycntMK6",
				2: "3Qt1EaKRD9g9FeL2DGkLLswhK1AKmmXFSe",
				3: "3PbExiaztsSYgh6zeMswC49hLUwhTQ86XG",
			},
		},
	}

	for i, test := range tests {
		if err := pool.CreateSeries(test.series, test.pubKeys, test.reqSigs); err != nil {
			t.Fatalf("Cannot creates series %v", test.series)
		}
		for branch, expectedAddress := range test.addresses {
			addr, err := pool.DepositScriptAddress(test.series, branch, 0)
			if err != nil {
				t.Fatalf("Failed to get DepositScriptAddress #%d: %v", i, err)
			}
			address := addr.Address().EncodeAddress()
			if expectedAddress != address {
				t.Errorf("DepositScript #%d returned the wrong deposit script. Got %v, want %v",
					i, address, expectedAddress)
			}
		}
	}
}

func TestDepositScriptAddressForNonExistentSeries(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	if _, err := pool.DepositScriptAddress(0, 0, 0); err == nil {
		t.Fatalf("Expected an error, got none")
	} else {
		rerr := err.(waddrmgr.ManagerError)
		if waddrmgr.ErrSeriesNotExists != rerr.ErrorCode {
			t.Errorf("Got %v, want ErrSeriesNotExists", rerr.ErrorCode)
		}
	}
}

func TestDepositScriptAddressForHardenedPubKey(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()
	if err := pool.CreateSeries(0, []string{pubKey0, pubKey1, pubKey2}, 2); err != nil {
		t.Fatalf("Cannot creates series")
	}

	// Ask for a DepositScriptAddress using an index for a hardened child, which should
	// fail as we use the extended public keys to derive childs.
	_, err := pool.DepositScriptAddress(0, 0, uint32(hdkeychain.HardenedKeyStart+1))

	if err == nil {
		t.Fatalf("Expected an error, got none")
	} else {
		rerr := err.(waddrmgr.ManagerError)
		if waddrmgr.ErrKeyChain != rerr.ErrorCode {
			t.Errorf("Got %v, want ErrKeyChain", rerr.ErrorCode)
		}
	}
}

func TestCreateVotingPool(t *testing.T) {
	tearDown, mgr, pool := setUp(t)
	defer tearDown()

	pool2, err := mgr.LoadVotingPool(pool.ID)
	if err != nil {
		t.Errorf("Error loading VotingPool: %v", err)
	}
	if !bytes.Equal(pool2.ID, pool.ID) {
		t.Errorf("Voting pool obtained from DB does not match the created one")
	}
}

func TestCreateSeries(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	tests := []struct {
		in      []string
		series  uint32
		reqSigs uint32
	}{
		{
			in:      []string{pubKey0, pubKey1, pubKey2},
			series:  0,
			reqSigs: 2,
		},
		{
			in:      []string{pubKey0, pubKey1, pubKey2, pubKey3, pubKey4},
			series:  1,
			reqSigs: 3,
		},
		{
			in: []string{pubKey0, pubKey1, pubKey2, pubKey3, pubKey4,
				pubKey5, pubKey6},
			series:  2,
			reqSigs: 4,
		},
		{
			in: []string{pubKey0, pubKey1, pubKey2, pubKey3, pubKey4,
				pubKey5, pubKey6, pubKey7, pubKey8},
			series:  3,
			reqSigs: 5,
		},
	}

	t.Logf("CreateSeries: Running %d tests", len(tests))
	for testNum, test := range tests {
		err := pool.CreateSeries(uint32(test.series), test.in[:], test.reqSigs)
		if err != nil {
			t.Errorf("%d: Cannot create series %d", testNum, test.series)
		}
		if !pool.ExistsSeriesTestsOnly(test.series) {
			t.Errorf("%d: Series %d not in database", testNum, test.series)
		}
	}
}

func TestPutSeriesErrors(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	tests := []struct {
		pubKeys []string
		reqSigs uint32
		err     waddrmgr.ManagerError
		msg     string
	}{
		{
			pubKeys: []string{pubKey0},
			err:     waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrTooFewPublicKeys},
			msg:     "Should return error when passed too few pubkeys",
		},
		{
			pubKeys: []string{pubKey0, pubKey1, pubKey2},
			reqSigs: 5,
			err:     waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrTooManyReqSignatures},
			msg:     "Should return error when reqSigs > len(pubKeys)",
		},
		{
			pubKeys: []string{pubKey0, pubKey1, pubKey2, pubKey0},
			err:     waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrKeyDuplicate},
			msg:     "Should return error when passed duplicate pubkeys",
		},
		{
			pubKeys: []string{"invalidxpub1", "invalidxpub2", "invalidxpub3"},
			err:     waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrKeyChain},
			msg:     "Should return error when passed invalid pubkey",
		},
		{
			pubKeys: []string{privKey0, privKey1, privKey2},
			err:     waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrKeyIsPrivate},
			msg:     "Should return error when passed private keys",
		},
	}

	for i, test := range tests {
		err := pool.TstPutSeries(uint32(i), test.pubKeys, test.reqSigs)
		if err == nil {
			str := fmt.Sprintf(test.msg+" pubKeys: %v, reqSigs: %v",
				test.pubKeys, test.reqSigs)
			t.Errorf(str)
		} else {
			retErr := err.(waddrmgr.ManagerError)
			if test.err.ErrorCode != retErr.ErrorCode {
				t.Errorf(
					"Create series #%d - Incorrect error type. Got %s, want %s",
					i, retErr.ErrorCode, test.err.ErrorCode)
			}
		}
	}
}

func TestSerialization(t *testing.T) {
	tearDown, mgr, _ := setUp(t)
	defer tearDown()

	tests := []struct {
		pubKeys  []string
		privKeys []string
		reqSigs  uint32
		err      error
		serial   []byte
		sErr     error
	}{
		{
			pubKeys: []string{pubKey0},
			reqSigs: 1,
		},
		{
			pubKeys:  []string{pubKey0},
			privKeys: []string{privKey0},
			reqSigs:  1,
		},
		{
			pubKeys: []string{pubKey0, pubKey1, pubKey2},
			reqSigs: 2,
		},
		{
			pubKeys:  []string{pubKey0, pubKey1, pubKey2},
			privKeys: []string{privKey0, "", ""},
			reqSigs:  2,
		},
		{
			pubKeys: []string{pubKey0, pubKey1, pubKey2, pubKey3, pubKey4},
			reqSigs: 3,
		},
		{
			pubKeys:  []string{pubKey0, pubKey1, pubKey2, pubKey3, pubKey4, pubKey5, pubKey6},
			privKeys: []string{"", privKey1, "", privKey3, "", "", ""},
			reqSigs:  4,
		},
		// Errors
		{
			pubKeys: []string{"NONSENSE"},
			// not a valid length pub key
			err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
		},
		{
			pubKeys:  []string{"PUBKEY1", "PUBKEY2"},
			privKeys: []string{"PRIVKEY1"},
			// pub and priv keys should be the same length
			err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
		},
		{
			pubKeys:  []string{pubKey0, pubKey1},
			reqSigs:  2,
			privKeys: []string{"NONSENSE"},
			// not a valid length priv key
			err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
		},
		{
			serial: []byte("WRONG"),
			// not enough bytes (under the theoretical minimum)
			sErr: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
		},
		{
			serial: make([]byte, 10000),
			// too many bytes (over the theoretical maximum)
			sErr: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
		},
		{
			serial: []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			// not enough bytes (specifically not enough public keys)
			sErr: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
		},
		{
			serial: []byte{0x01, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00,
			},
			// not enough bytes (specifically no private keys)
			sErr: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
		},
		{
			serial: []byte{0x01, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00,
				0x01, 0x00, 0x00, 0x00,
			},
			// not enough bytes for serialization
			sErr: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
		},
		{
			serial: []byte{0x01, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00,
				0x01, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00,
			},
			// too many bytes for serialization
			sErr: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
		},
	}

	var err error

	t.Logf("Serialization: Running %d tests", len(tests))
	for testNum, test := range tests {
		var serialized []byte
		var encryptedPubs, encryptedPrivs [][]byte
		if test.serial == nil {
			encryptedPubs = make([][]byte, len(test.pubKeys))
			encryptedPrivs = make([][]byte, len(test.privKeys))
			for i, pubKey := range test.pubKeys {
				encryptedPubs[i], err = mgr.EncryptWithCryptoKeyPub([]byte(pubKey))
				if err != nil {
					t.Errorf("Serialization #%d - Failed to encrypt public key %v",
						testNum, pubKey)
				}
			}

			for i, privKey := range test.privKeys {
				if privKey == "" {
					encryptedPrivs[i] = nil
				} else {
					encryptedPrivs[i], err = mgr.EncryptWithCryptoKeyPub([]byte(privKey))
				}
				if err != nil {
					t.Errorf("Serialization #%d - Failed to encrypt private key %v",
						testNum, privKey)
				}
			}

			serialized, err = waddrmgr.SerializeSeries(
				test.reqSigs, encryptedPubs, encryptedPrivs)
			if test.err != nil {
				if err == nil {
					t.Errorf("Serialization #%d - Should have gotten an error and didn't",
						testNum)
					continue
				}
				terr := test.err.(waddrmgr.ManagerError)
				rerr := err.(waddrmgr.ManagerError)
				if terr.ErrorCode != rerr.ErrorCode {
					t.Errorf("Serialization #%d - Received wrong error type. Got %d, want %d",
						testNum, rerr.ErrorCode, terr.ErrorCode)
				}
				continue
			} else if err != nil {
				t.Errorf("Serialization #%d - Error in serialization %v",
					testNum, err)
				continue
			}
		} else {
			// Shortcut this serialization and pretend we got some other string
			// that's defined in the test.
			serialized = test.serial
		}

		row, err := waddrmgr.DeserializeSeries(serialized)

		if test.sErr != nil {
			if err == nil {
				t.Errorf("Serialization #%d - Did not get the expected error",
					testNum)
				continue
			}
			terr := test.sErr.(waddrmgr.ManagerError)
			rerr := err.(waddrmgr.ManagerError)
			if terr.ErrorCode != rerr.ErrorCode {
				t.Errorf("Serialization #%d - Incorrect error type. Got %d, want %d",
					testNum, rerr.ErrorCode, terr.ErrorCode)
			}
			continue
		}

		if err != nil {
			t.Errorf("Serialization #%d - Failed to deserialize %v %v",
				testNum, serialized, err)
			continue
		}

		if row.ReqSigs != test.reqSigs {
			t.Errorf("Serialization #%d - row reqSigs off. Got %d, want %d",
				testNum, row.ReqSigs, test.reqSigs)
			continue
		}

		if len(row.PubKeysEncrypted) != len(test.pubKeys) {
			t.Errorf("Serialization #%d - Wrong no. of pubkeys. Got %d, want %d",
				testNum, len(row.PubKeysEncrypted), len(test.pubKeys))
			continue
		}

		for i, encryptedPub := range encryptedPubs {
			got := string(row.PubKeysEncrypted[i])

			if got != string(encryptedPub) {
				t.Errorf("Serialization #%d - Pubkey deserialization. Got %v, want %v",
					testNum, got, string(encryptedPub))
				continue
			}
		}

		if len(row.PrivKeysEncrypted) != len(row.PubKeysEncrypted) {
			t.Errorf("Serialization #%d - no. privkeys (%d) != no. pubkeys (%d)",
				testNum, len(row.PrivKeysEncrypted), len(row.PubKeysEncrypted))
			continue
		}

		for i, encryptedPriv := range encryptedPrivs {
			got := string(row.PrivKeysEncrypted[i])

			if got != string(encryptedPriv) {
				t.Errorf("Serialization #%d - Privkey deserialization. Got %v, want %v",
					testNum, got, string(encryptedPriv))
				continue
			}
		}
	}
}

func TestCannotReplaceEmpoweredSeries(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	var seriesId uint32 = 1

	err := pool.CreateSeries(seriesId, []string{pubKey0, pubKey1, pubKey2, pubKey3}, 3)
	if err != nil {
		t.Fatalf("Failed to create series", err)
	}

	if err := pool.EmpowerSeries(seriesId, privKey1); err != nil {
		t.Fatalf("Failed to empower series", err)
	}

	err = pool.ReplaceSeries(seriesId, []string{pubKey0, pubKey2, pubKey3}, 2)
	if err == nil {
		t.Errorf("Replaced an empowered series. This should not be possible", err)
	} else {
		gotErr := err.(waddrmgr.ManagerError)
		wantErrCode := waddrmgr.ErrorCode(waddrmgr.ErrSeriesAlreadyEmpowered)
		if wantErrCode != gotErr.ErrorCode {
			t.Errorf("Got %s, want %s", gotErr.ErrorCode, wantErrCode)
		}
	}
}

func TestReplaceNonExistingSeries(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	pubKeys := []string{pubKey0, pubKey1, pubKey2}
	if err := pool.ReplaceSeries(uint32(1), pubKeys, 3); err == nil {
		t.Errorf("Replaced non-existent series. This should not be possible.")
	} else {
		gotErr := err.(waddrmgr.ManagerError)
		wantErrCode := waddrmgr.ErrorCode(waddrmgr.ErrSeriesNotExists)
		if wantErrCode != gotErr.ErrorCode {
			t.Errorf("Got %s, want %s", gotErr.ErrorCode, wantErrCode)
		}
	}
}

type replaceSeriesTestEntry struct {
	testId      int
	orig        seriesRaw
	replaceWith seriesRaw
}

var replaceSeriesTestData = []replaceSeriesTestEntry{
	{
		testId: 0,
		orig: seriesRaw{
			id:      0,
			pubKeys: waddrmgr.CanonicalKeyOrder([]string{pubKey0, pubKey1, pubKey2, pubKey4}),
			reqSigs: 2,
		},
		replaceWith: seriesRaw{
			id:      0,
			pubKeys: waddrmgr.CanonicalKeyOrder([]string{pubKey3, pubKey4, pubKey5}),
			reqSigs: 1,
		},
	},
	{
		testId: 1,
		orig: seriesRaw{
			id:      2,
			pubKeys: waddrmgr.CanonicalKeyOrder([]string{pubKey0, pubKey1, pubKey2}),
			reqSigs: 2,
		},
		replaceWith: seriesRaw{
			id:      2,
			pubKeys: waddrmgr.CanonicalKeyOrder([]string{pubKey3, pubKey4, pubKey5, pubKey6}),
			reqSigs: 2,
		},
	},
	{
		testId: 2,
		orig: seriesRaw{
			id: 4,
			pubKeys: waddrmgr.CanonicalKeyOrder([]string{
				pubKey0, pubKey1, pubKey2, pubKey3, pubKey4, pubKey5, pubKey6, pubKey7, pubKey8}),
			reqSigs: 8,
		},
		replaceWith: seriesRaw{
			id: 4,
			pubKeys: waddrmgr.CanonicalKeyOrder([]string{
				pubKey0, pubKey1, pubKey2, pubKey3, pubKey4, pubKey5, pubKey6, pubKey7}),
			reqSigs: 7,
		},
	},
}

func TestReplaceExistingSeries(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	for _, data := range replaceSeriesTestData {
		seriesID := data.orig.id
		testID := data.testId

		err := pool.CreateSeries(seriesID, data.orig.pubKeys, data.orig.reqSigs)
		if err != nil {
			t.Fatalf("Test #%d: failed to create series in replace series setup",
				testID, err)
		}

		err = pool.ReplaceSeries(seriesID, data.replaceWith.pubKeys, data.replaceWith.reqSigs)
		if err != nil {
			t.Errorf("Test #%d: replaceSeries failed", testID, err)
		}

		validateReplaceSeries(t, pool, testID, data.replaceWith)
	}
}

// validateReplaceSeries validate the created series stored in the system
// corresponds to the series we replaced the original with.
func validateReplaceSeries(t *testing.T, pool *waddrmgr.VotingPool, testID int, replacedWith seriesRaw) {
	seriesID := replacedWith.id
	series := pool.GetSeries(seriesID)
	if series == nil {
		t.Fatalf("Test #%d Series #%d: series not found", testID, seriesID)
	}

	pubKeys := series.TstGetRawPublicKeys()
	// Check that the public keys match what we expect.
	if !reflect.DeepEqual(replacedWith.pubKeys, pubKeys) {
		t.Errorf("Test #%d, series #%d: pubkeys mismatch. Got %v, want %v",
			testID, seriesID, pubKeys, replacedWith.pubKeys)
	}

	// Check number of required sigs.
	if replacedWith.reqSigs != series.TstGetReqSigs() {
		t.Errorf("Test #%d, series #%d: required signatures mismatch. Got %d, want %d",
			testID, seriesID, series.TstGetReqSigs(), replacedWith.reqSigs)
	}

	// Check that the series is not empowered.
	if series.IsEmpowered() {
		t.Errorf("Test #%d, series #%d: series is empowered but should not be",
			testID, seriesID)
	}
}

func TestEmpowerSeries(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	seriesID := uint32(0)
	err := pool.CreateSeries(seriesID, []string{pubKey0, pubKey1, pubKey2}, 2)
	if err != nil {
		t.Fatalf("Failed to create series: %v", err)
	}

	tests := []struct {
		seriesID uint32
		key      string
		err      error
	}{
		{
			seriesID: 0,
			key:      privKey0,
		},
		{
			seriesID: 0,
			key:      privKey1,
		},
		{
			seriesID: 1,
			key:      privKey0,
			// invalid series
			err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesNotExists},
		},
		{
			seriesID: 0,
			key:      "NONSENSE",
			// invalid private key
			err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrKeyChain},
		},
		{
			seriesID: 0,
			key:      pubKey5,
			// wrong type of key
			err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrKeyIsPublic},
		},
		{
			seriesID: 0,
			key:      privKey5,
			// key not corresponding to pub key
			err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrKeysPrivatePublicMismatch},
		},
	}

	for testNum, test := range tests {
		// Add the extended private key to voting pool.
		err := pool.EmpowerSeries(test.seriesID, test.key)
		if test.err != nil {
			if err == nil {
				t.Errorf("EmpowerSeries #%d Expected an error and got none", testNum)
				continue
			}
			if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
				t.Errorf("DepositScript #%d wrong error type. Got: %v <%T>, want: %T",
					testNum, err, err, test.err)
				continue
			}
			rerr := err.(waddrmgr.ManagerError)
			trerr := test.err.(waddrmgr.ManagerError)
			if rerr.ErrorCode != trerr.ErrorCode {
				t.Errorf("DepositScript #%d wrong error code. Got: %v, want: %v",
					testNum, rerr.ErrorCode, trerr.ErrorCode)
				continue
			}
			continue
		}

		if err != nil {
			t.Errorf("EmpowerSeries #%d Unexpected error %v", testNum, err)
			continue
		}
	}

}

func TestGetSeries(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()
	expectedPubKeys := waddrmgr.CanonicalKeyOrder([]string{pubKey0, pubKey1, pubKey2})
	if err := pool.CreateSeries(0, expectedPubKeys, 2); err != nil {
		t.Fatalf("Failed to create series: %v", err)
	}

	series := pool.GetSeries(0)

	if series == nil {
		t.Fatal("GetSeries() returned nil")
	}
	pubKeys := series.TstGetRawPublicKeys()
	if !reflect.DeepEqual(pubKeys, expectedPubKeys) {
		t.Errorf("Series pubKeys mismatch. Got %v, want %v", pubKeys, expectedPubKeys)
	}
}

type seriesRaw struct {
	id       uint32
	pubKeys  []string
	privKeys []string
	reqSigs  uint32
}

type testLoadAllSeriesTest struct {
	id     int
	series []seriesRaw
}

var testLoadAllSeriesTests = []testLoadAllSeriesTest{
	{
		id: 1,
		series: []seriesRaw{
			{
				id:      0,
				pubKeys: []string{pubKey0, pubKey1, pubKey2},
				reqSigs: 2,
			},
			{
				id:       1,
				pubKeys:  []string{pubKey3, pubKey4, pubKey5},
				privKeys: []string{privKey4},
				reqSigs:  2,
			},
			{
				id:       2,
				pubKeys:  []string{pubKey0, pubKey1, pubKey2, pubKey3, pubKey4},
				privKeys: []string{privKey0, privKey2},
				reqSigs:  3,
			},
		},
	},
	{
		id: 2,
		series: []seriesRaw{
			{
				id:      0,
				pubKeys: []string{pubKey0, pubKey1, pubKey2},
				reqSigs: 2,
			},
		},
	},
}

func setUpLoadAllSeries(t *testing.T, mgr *waddrmgr.Manager, test testLoadAllSeriesTest) *waddrmgr.VotingPool {
	pool, err := mgr.CreateVotingPool([]byte{byte(test.id + 1)})
	if err != nil {
		t.Fatalf("Voting Pool creation failed: %v", err)
	}

	for _, series := range test.series {
		err := pool.CreateSeries(series.id, series.pubKeys, series.reqSigs)
		if err != nil {
			t.Fatalf("Test #%d Series #%d: failed to create series: %v",
				test.id, series.id, err)
		}

		for _, privKey := range series.privKeys {
			err := pool.EmpowerSeries(series.id, privKey)
			if err != nil {
				t.Fatalf("Test #%d Series #%d: empower with privKey %v failed: %v",
					test.id, series.id, privKey, err)
			}
		}
	}
	return pool
}

func TestLoadAllSeries(t *testing.T) {
	tearDown, manager, _ := setUp(t)
	defer tearDown()

	for _, test := range testLoadAllSeriesTests {
		pool := setUpLoadAllSeries(t, manager, test)
		pool.TstEmptySeriesLookup()
		err := pool.LoadAllSeries()
		if err != nil {
			t.Fatalf("Test #%d: failed to load voting pool: %v", test.id, err)
		}
		for _, seriesData := range test.series {
			validateLoadAllSeries(t, pool, test.id, seriesData)
		}
	}
}

func validateLoadAllSeries(t *testing.T, pool *waddrmgr.VotingPool, testID int, seriesData seriesRaw) {
	series := pool.GetSeries(seriesData.id)

	// Check that the series exists.
	if series == nil {
		t.Errorf("Test #%d, series #%d: series not found", testID, seriesData.id)
	}

	// Check that reqSigs is what we inserted.
	if seriesData.reqSigs != series.TstGetReqSigs() {
		t.Errorf("Test #%d, series #%d: required sigs are different. Got %d, want %d",
			testID, seriesData.id, series.TstGetReqSigs(), seriesData.reqSigs)
	}

	// Check that pubkeys and privkeys have the same length.
	publicKeys := series.TstGetRawPublicKeys()
	privateKeys := series.TstGetRawPrivateKeys()
	if len(privateKeys) != len(publicKeys) {
		t.Errorf("Test #%d, series #%d: wrong number of private keys. Got %d, want %d",
			testID, seriesData.id, len(privateKeys), len(publicKeys))
	}

	sortedKeys := waddrmgr.CanonicalKeyOrder(seriesData.pubKeys)
	if !reflect.DeepEqual(publicKeys, sortedKeys) {
		t.Errorf("Test #%d, series #%d: public keys mismatch. Got %d, want %d",
			testID, seriesData.id, sortedKeys, publicKeys)
	}

	// Check that privkeys are what we inserted (length and content).
	foundPrivKeys := make([]string, 0, len(seriesData.pubKeys))
	for _, privateKey := range privateKeys {
		if privateKey != "" {
			foundPrivKeys = append(foundPrivKeys, privateKey)
		}
	}
	foundPrivKeys = waddrmgr.CanonicalKeyOrder(foundPrivKeys)
	privKeys := waddrmgr.CanonicalKeyOrder(seriesData.privKeys)
	if !reflect.DeepEqual(privKeys, foundPrivKeys) {
		t.Errorf("Test #%d, series #%d: private keys mismatch. Got %d, want %d",
			testID, seriesData.id, foundPrivKeys, privKeys)
	}
}

func reverse(inKeys []*btcutil.AddressPubKey) []*btcutil.AddressPubKey {
	revKeys := make([]*btcutil.AddressPubKey, len(inKeys))
	max := len(inKeys)
	for i := range inKeys {
		revKeys[i] = inKeys[max-i-1]
	}
	return revKeys
}

func TestBranchOrderZero(t *testing.T) {
	// test change address branch (0) for 0-10 keys
	for i := 0; i < 10; i++ {
		inKeys := createTestPubKeys(t, i, 0)
		wantKeys := reverse(inKeys)
		resKeys := waddrmgr.BranchOrder(inKeys, 0)

		if len(resKeys) != len(wantKeys) {
			t.Errorf("BranchOrder: wrong no. of keys. Got: %d, want %d",
				len(resKeys), len(inKeys))
			return
		}

		for keyIdx := 0; i < len(inKeys); i++ {
			if resKeys[keyIdx] != wantKeys[keyIdx] {
				fmt.Printf("%p, %p\n", resKeys[i], wantKeys[i])
				t.Errorf("BranchOrder(keys, 0): got %v, want %v",
					resKeys[i], wantKeys[i])
			}
		}
	}
}

func TestBranchOrderNilKeys(t *testing.T) {
	// Test branchorder with nil input and various branch numbers.
	for i := 0; i < 10; i++ {
		res := waddrmgr.BranchOrder(nil, uint32(i))
		if res != nil {
			t.Errorf("Got non-nil: %v, want nil.", res)
		}
	}
}

func TestBranchOrderNonZero(t *testing.T) {
	maxBranch := 5
	maxTail := 4
	// Test branch reordering for branch no. > 0. We test all branch values
	// within [1, 5] in a slice of up to 9 (maxBranch-1 + branch-pivot +
	// maxTail) keys. Hopefully that covers all combinations and edge-cases.
	// We test the case where branch no. is 0 elsewhere.
	for branch := 1; branch <= maxBranch; branch++ {
		for j := 0; j <= maxTail; j++ {
			first := createTestPubKeys(t, branch-1, 0)
			pivot := createTestPubKeys(t, 1, branch)
			last := createTestPubKeys(t, j, branch+1)

			inKeys := append(append(first, pivot...), last...)
			wantKeys := append(append(pivot, first...), last...)
			resKeys := waddrmgr.BranchOrder(inKeys, uint32(branch))

			if len(resKeys) != len(inKeys) {
				t.Errorf("BranchOrder: wrong no. of keys. Got: %d, want %d",
					len(resKeys), len(inKeys))
			}

			for idx := 0; idx < len(inKeys); idx++ {
				if resKeys[idx] != wantKeys[idx] {
					o, w, g := branchErrorFormat(inKeys, wantKeys, resKeys)
					t.Errorf("Branch: %d\nOrig: %v\nGot: %v\nWant: %v", branch, o, g, w)
				}
			}
		}
	}
}

func branchErrorFormat(orig, want, got []*btcutil.AddressPubKey) (origOrder, wantOrder, gotOrder []int) {
	origOrder = []int{}
	origMap := make(map[*btcutil.AddressPubKey]int)
	for i, key := range orig {
		origMap[key] = i + 1
		origOrder = append(origOrder, i+1)
	}

	wantOrder = []int{}
	for _, key := range want {
		wantOrder = append(wantOrder, origMap[key])
	}

	gotOrder = []int{}
	for _, key := range got {
		gotOrder = append(gotOrder, origMap[key])
	}

	return origOrder, wantOrder, gotOrder
}

func createTestPubKeys(t *testing.T, number, offset int) []*btcutil.AddressPubKey {

	net := &btcnet.TestNet3Params
	xpubRaw := "xpub661MyMwAqRbcFwdnYF5mvCBY54vaLdJf8c5ugJTp5p7PqF9J1USgBx12qYMnZ9yUiswV7smbQ1DSweMqu8wn7Jociz4PWkuJ6EPvoVEgMw7"
	xpubKey, err := hdkeychain.NewKeyFromString(xpubRaw)
	if err != nil {
		t.Fatalf("Failed to generate new key", err)
	}

	keys := make([]*btcutil.AddressPubKey, number)
	for i := uint32(0); i < uint32(len(keys)); i++ {
		chPubKey, err := xpubKey.Child(i + uint32(offset))
		if err != nil {
			t.Fatalf("Failed to generate child key", err)
		}

		pubKey, err := chPubKey.ECPubKey()
		if err != nil {
			t.Fatalf("Failed to generate ECPubKey", err)
		}

		x, err := btcutil.NewAddressPubKey(pubKey.SerializeCompressed(), net)
		if err != nil {
			t.Fatalf("Failed to create new public key", err)
		}
		keys[i] = x
	}
	return keys
}

func TestReverse(t *testing.T) {
	// Test the utility function that reverses a list of public keys.
	// 11 is arbitrary.
	for numKeys := 0; numKeys < 11; numKeys++ {
		keys := createTestPubKeys(t, numKeys, 0)
		revRevKeys := reverse(reverse(keys))
		if len(keys) != len(revRevKeys) {
			t.Errorf("Reverse(Reverse(x)): the no. pubkeys changed. Got %d, want %d",
				len(revRevKeys), len(keys))
		}

		for i := 0; i < len(keys); i++ {
			if keys[i] != revRevKeys[i] {
				t.Errorf("Reverse(Reverse(x)) != x. Got %v, want %v",
					revRevKeys[i], keys[i])
			}
		}
	}
}

func TestEmpowerSeriesNeuterFailed(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	seriesID := uint32(0)
	err := pool.CreateSeries(seriesID, []string{pubKey0, pubKey1, pubKey2}, 2)
	if err != nil {
		t.Fatalf("Failed to create series: %v", err)
	}

	// A private key with bad version (0xffffffff) will trigger an
	// error in (k *ExtendedKey).Neuter and the associated error path
	// in EmpowerSeries.
	badKey := "wM5uZBNTYmaYGiK8VaGi7zPGbZGLuQgDiR2Zk4nGfbRFLXwHGcMUdVdazRpNHFSR7X7WLmzzbAq8dA1ViN6eWKgKqPye1rJTDQTvBiXvZ7E3nmdx"
	if err := pool.EmpowerSeries(seriesID, badKey); err == nil {
		t.Errorf("Expected an error, got none")
	} else {
		gotErr := err.(waddrmgr.ManagerError)
		wantErrCode := waddrmgr.ErrorCode(waddrmgr.ErrKeyNeuter)
		if wantErrCode != gotErr.ErrorCode {
			t.Errorf("Got %s, want %s", gotErr.ErrorCode, wantErrCode)
		}
	}
}

type testSerializationErrorTest struct {
	version  uint32
	reqSigs  uint32
	pubKeys  [][]byte
	privKeys [][]byte
	err      waddrmgr.ManagerError
}

var testSerializationErrorTests = []testSerializationErrorTest{
	{
		version: 1,
		reqSigs: 1,
		pubKeys: [][]byte{[]byte("NONSENSE")},
		// not a valid length pub key
		err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
	},
	{
		version:  1,
		reqSigs:  1,
		pubKeys:  [][]byte{[]byte("PUBKEY1"), []byte("PUBKEY2")},
		privKeys: [][]byte{[]byte("PRIVKEY1")},
		// pub and priv keys should be the same length
		err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
	},
	{
		version:  1,
		reqSigs:  2,
		pubKeys:  [][]byte{make([]byte, 151), make([]byte, 151)},
		privKeys: [][]byte{[]byte("NONSENSE")},
		// not a valid length priv key
		err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
	},
	{
		version: 2,
		reqSigs: 2,
		pubKeys: [][]byte{make([]byte, 151), make([]byte, 151)},
		// not a valid length priv key
		err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesVersion},
	},
}

func TestSerializationError(t *testing.T) {
	t.Logf("SerializationError: Running %d tests", len(testSerializationErrorTests))
	for testNum, test := range testSerializationErrorTests {
		_, err := waddrmgr.SerializeSeries(test.version, true,
			test.reqSigs, test.pubKeys, test.privKeys)
		if err == nil {
			t.Errorf("SerializationError #%d - expected error %v and got none",
				testNum, test.err)
			continue
		}
		rerr := err.(waddrmgr.ManagerError)
		if rerr.ErrorCode != test.err.ErrorCode {
			t.Errorf("SerializationError #%d - error mismatch: got %v, want %v",
				testNum, rerr, test.err)
			continue
		}
	}
}

type testDeserializationErrorTest struct {
	serial []byte
	err    waddrmgr.ManagerError
}

var testDeserializationErrorTests = []testDeserializationErrorTest{
	{
		serial: make([]byte, 10),
		// not enough bytes (under the theoretical minimum)
		err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
	},
	{
		serial: make([]byte, 10000),
		// too many bytes (over the theoretical maximum)
		err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
	},
	{
		serial: []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00},
		// version too high
		err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesVersion},
	},
	{
		serial: []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00,
		},
		// not enough bytes for serialization
		err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
	},
	{
		serial: []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00,
			0x00,
		},
		// too many bytes for serialization
		err: waddrmgr.ManagerError{ErrorCode: waddrmgr.ErrSeriesStorage},
	},
}

func TestDeserializationError(t *testing.T) {
	t.Logf("SerializationError: Running %d tests", len(testDeserializationErrorTests))
	for testNum, test := range testDeserializationErrorTests {
		_, err := waddrmgr.DeserializeSeries(test.serial)
		if err == nil {
			t.Errorf("SerializationError #%d - expected error %v and got none",
				testNum, test.err)
			continue
		}
		rerr := err.(waddrmgr.ManagerError)
		if rerr.ErrorCode != test.err.ErrorCode {
			t.Errorf("SerializationError #%d - error mismatch: got %v, want %v",
				testNum, rerr, test.err)
			continue
		}
	}
}

type testSerializationTest struct {
	version           uint32
	active            bool
	reqSigs           uint32
	pubKeys           []string
	privKeys          []string
	pubKeysEncrypted  [][]byte
	privKeysEncrypted [][]byte
}

var testSerializationTests = []testSerializationTest{
	{
		version: 1,
		active:  false,
		reqSigs: 1,
		pubKeys: []string{pubKey0},
	},
	{
		version:  1,
		active:   false,
		reqSigs:  1,
		pubKeys:  []string{pubKey0},
		privKeys: []string{privKey0},
	},
	{
		version: 1,
		active:  true,
		reqSigs: 2,
		pubKeys: []string{pubKey0, pubKey1, pubKey2},
	},
	{
		version:  1,
		active:   false,
		reqSigs:  2,
		pubKeys:  []string{pubKey0, pubKey1, pubKey2},
		privKeys: []string{privKey0, "", ""},
	},
	{
		version: 1,
		active:  true,
		reqSigs: 3,
		pubKeys: []string{pubKey0, pubKey1, pubKey2, pubKey3, pubKey4},
	},
	{
		version: 1,
		active:  true,
		reqSigs: 4,
		pubKeys: []string{pubKey0, pubKey1, pubKey2,
			pubKey3, pubKey4, pubKey5, pubKey6},
		privKeys: []string{"", privKey1, "", privKey3, "", "", ""},
	},
}

func setUpSerialization(t *testing.T, mgr *waddrmgr.Manager,
	test *testSerializationTest, testNum int) {
	var err error

	// having no privKeys means that we should have an empty list of privKeys
	if test.privKeys == nil {
		test.privKeys = make([]string, len(test.pubKeys))
	}

	// create the encrypted versions of the pub/priv keys to use later
	test.pubKeysEncrypted = make([][]byte, len(test.pubKeys))
	test.privKeysEncrypted = make([][]byte, len(test.privKeys))
	for i, pubKey := range test.pubKeys {
		test.pubKeysEncrypted[i], err = mgr.EncryptWithCryptoKeyPub([]byte(pubKey))
		if err != nil {
			t.Fatalf("Serialization #%d -  Failed to encrypt public key %v",
				testNum, pubKey)
		}
	}
	for i, privKey := range test.privKeys {
		if privKey == "" {
			test.privKeysEncrypted[i] = nil
		} else {
			test.privKeysEncrypted[i], err =
				mgr.EncryptWithCryptoKeyPub([]byte(privKey))
		}
		if err != nil {
			t.Fatalf("Serialization #%d -  Failed to encrypt private key %v",
				testNum, privKey)
		}
	}
	return
}

func executeSerialization(t *testing.T, test *testSerializationTest,
	testNum int) *waddrmgr.SeriesRow {
	serialized, err := waddrmgr.SerializeSeries(test.version, test.active,
		test.reqSigs, test.pubKeysEncrypted, test.privKeysEncrypted)
	if err != nil {
		t.Fatalf("Serialization #%d - Error in serialization %v",
			testNum, err)
	}
	row, err := waddrmgr.DeserializeSeries(serialized)
	if err != nil {
		t.Fatalf("Serialization #%d - Error in deserialization %v",
			testNum, err)
	}
	return row
}

func TestSerialization(t *testing.T) {
	tearDown, mgr, _ := setUp(t)
	defer tearDown()

	t.Logf("Serialization: Running %d tests", len(testSerializationTests))
	for testNum, test := range testSerializationTests {
		setUpSerialization(t, mgr, &test, testNum)
		row := executeSerialization(t, &test, testNum)
		validateSerialization(t, &test, row, testNum)
	}
}

func validateSerialization(t *testing.T, test *testSerializationTest,
	row *waddrmgr.SeriesRow, testNum int) {

	if row.Version != test.version {
		t.Errorf("Serialization #%d - version mismatch: got %d want %d",
			testNum, row.Version, test.version)
		return
	}

	if row.Active != test.active {
		t.Errorf("Serialization #%d - active mismatch: got %d want %d",
			testNum, row.Active, test.active)
		return
	}

	if row.ReqSigs != test.reqSigs {
		t.Errorf("Serialization #%d - reqSigs mismatch: got %d want %d",
			testNum, row.ReqSigs, test.reqSigs)
		return
	}

	if !reflect.DeepEqual(row.PubKeysEncrypted, test.pubKeysEncrypted) {
		t.Errorf("Serialization #%d - pubkeys mismatch: got %d want %d",
			testNum, row.PubKeysEncrypted, test.pubKeysEncrypted)
		return
	}

	if !reflect.DeepEqual(row.PrivKeysEncrypted, test.privKeysEncrypted) {
		t.Errorf("Serialization #%d - privkeys mismatch: got %d want %d",
			testNum, row.PrivKeysEncrypted, test.privKeysEncrypted)
		return
	}
}
