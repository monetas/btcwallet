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
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/monetas/btcnet"
	"github.com/monetas/btcutil"
	"github.com/monetas/btcutil/hdkeychain"
	"github.com/monetas/btcwallet/waddrmgr"
)

var (
	privKeys = []string{
		"xprv9s21ZrQH143K2j9PK4CXkCu8sgxkpUxCF7p1KVwiV5tdnkeYzJXReUkxz5iB2FUzTXC1L15abCDG4RMxSYT5zhm67uvsnLYxuDhZfoFcB6a",
		"xprv9s21ZrQH143K4PtW77ATQAKAGk7KAFFCzxFuAcWduoMEeQhCgWpuYWQvMGZknqdispUbgLZV1YPqFCbpzMJij8tSZ5xPSaZqPbchojeNuq7",
		"xprv9s21ZrQH143K27XboWxXZGU5j7VZ9SqVBnmMQPKTbddiWAhuNzeLynKHaZTAti6N454tVUUcvy6u15DfuW68NCBUxry6ZsHHzqoA8UtzdMn",
		"xprv9s21ZrQH143K2vb4DGQymRejLcZSksBHTYLxB7Stg1c7Lk9JxgEUGZTozwUKxoEWJPoGSdGnJY1TW7LNFQCWrpZjDdEXJeqJuDde6BmdD4P",
		"xprv9s21ZrQH143K4JNmRvWeLc1PggzusKcDYV1y8fAMNDdb9Rm5X1AvGHizxEdhTVR3sc62XvifC6dLAXMuQesX1y6999xnDwQ3aVno8KviU9d",
		"xprv9s21ZrQH143K3dxrqESqeHZ7pSwM6Uq77ssQADSBs7qdFs6dyRWmRcPyLUTQRpgB3EduNhJuWkCGG2LHjuUisw8KKfXJpPqYJ1MSPrZpe1z",
		"xprv9s21ZrQH143K2nE8ENAMNksTTVxPrMxFNWUuwThMy2bcH9LHTtQDXSNq2pTNcbuq36n5A3J9pbXVqnq5LDXvqniFRLN299kW7Svnxsx9tQv",
		"xprv9s21ZrQH143K3p93xF1oFeB6ey5ruUesWjuPxA9Z2R5wf6BLYfGXz7fg7NavWkQ2cx3Vm8w2HV9uKpSprNNHnenGeW9XhYDPSjwS9hyCs33",
		"xprv9s21ZrQH143K3WxnnvPZ8SDGXndASvLTFwMLBVzNCVgs9rzP6rXgW92DLvozdyBm8T9bSQvrFm1jMpTJrRE6w1KY5tshFeDk9Nn3K6V5FYX",
	}

	pubKeys = []string{
		"xpub661MyMwAqRbcFDDrR5jY7LqsRioFDwg3cLjc7tML3RRcfYyhXqqgCH5SqMSQdpQ1Xh8EtVwcfm8psD8zXKPcRaCVSY4GCqbb3aMEs27GitE",
		"xpub661MyMwAqRbcGsxyD8hTmJFtpmwoZhy4NBBVxzvFU8tDXD2ME49A6JjQCYgbpSUpHGP1q4S2S1Pxv2EqTjwfERS5pc9Q2yeLkPFzSgRpjs9",
		"xpub661MyMwAqRbcEbc4uYVXvQQpH9L3YuZLZ1gxCmj59yAhNy33vXxbXadmRpx5YZEupNSqWRrR7PqU6duS2FiVCGEiugBEa5zuEAjsyLJjKCh",
		"xpub661MyMwAqRbcFQfXKHwz8ZbTtePwAKu8pmGYyVrWEM96DYUTWDYipMnHrFcemZHn13jcRMfsNU3UWQUudiaE7mhkWCHGFRMavF167DQM4Va",
		"xpub661MyMwAqRbcGnTEXx3ehjx8EiqQGnL4uhwZw3ZxvZAa2E6E4YVAp63UoVtvm2vMDDF8BdPpcarcf7PWcEKvzHhxzAYw1zG23C2egeh82AR",
		"xpub661MyMwAqRbcG83KwFyr1RVrNUmqVwYxV6nzxbqoRTNc8fRnWxq1yQiTBifTHhevcEM9ucZ1TqFS7Kv17Gd81cesv6RDrrvYS9SLPjPXhV5",
		"xpub661MyMwAqRbcFGJbLPhMjtpC1XntFpg6jjQWjr6yXN8b9wfS1RiU5EhJt5L7qoFuidYawc3XJoLjT2PcjVpXryS3hn1WmSPCyvQDNuKsfgM",
		"xpub661MyMwAqRbcGJDX4GYocn7qCzvMJwNisxpzkYZAakcvXtWV6CanXuz9xdfe5kTptFMJ4hDt2iTiT11zyN14u8R5zLvoZ1gnEVqNLxp1r3v",
		"xpub661MyMwAqRbcG13FtwvZVaA15pTerP4JdAGvytPykqDr2fKXePqw3wLhCALPAixsE176jFkc2ac9K3tnF4KwaTRKUqFF5apWD6XL9LHCu7E",
	}
)

func setUp(t *testing.T) (tearDownFunc func(), mgr *waddrmgr.Manager, pool *waddrmgr.VotingPool) {
	// Create a new manager.
	// we create the file and immediately delete it as the waddrmgr
	//  needs to be doing the creating.
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

func TestDepositScriptAddress(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	tests := []struct {
		in      []string
		series  uint32
		reqSigs uint32
		// map of branch:address (we only check the branch index at 0)
		addresses map[uint32]string
		err       error
	}{
		{
			in:      pubKeys[:3],
			series:  0,
			reqSigs: 2,
			addresses: map[uint32]string{
				0: "3Hb4xcebcKg4DiETJfwjh8sF4uDw9rqtVC",
				1: "34eVkREKgvvGASZW7hkgE2uNc1yycntMK6",
				2: "3Qt1EaKRD9g9FeL2DGkLLswhK1AKmmXFSe",
				3: "3PbExiaztsSYgh6zeMswC49hLUwhTQ86XG",
			},
			err: nil,
		},
		// Errors..
		{
			in: []string{"xpub"},
			// TODO: correct this error code to the right thing in error.go
			err: waddrmgr.ManagerError{ErrorCode: 0},
		},
	}

	t.Logf("DepositScript: Running %d tests", len(tests))
	for i, test := range tests {
		err := pool.CreateSeries(test.series, test.in, test.reqSigs)
		if err != nil {
			if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
				t.Errorf("DepositScript #%d wrong error type "+
					"got: %v <%T>, want: %T", i, err, err, test.err)
				continue
			}
			rerr := err.(waddrmgr.ManagerError)
			trerr := test.err.(waddrmgr.ManagerError)
			if rerr.ErrorCode != trerr.ErrorCode {
				t.Errorf("DepositScript #%d wrong "+
					"error code got: %v, want: %v", i,
					rerr.ErrorCode, trerr.ErrorCode)
				continue
			}
		} else {
			for branch, address := range test.addresses {
				addr, err := pool.DepositScriptAddress(test.series, branch, 0)
				if err != nil {
					t.Errorf("DepositScript #%d wrong "+
						"error %v", i, err)
					continue
				}
				got := addr.Address().EncodeAddress()
				if address != got {
					t.Errorf("DepositScript #%d returned "+
						"the wrong deposit script got: %v, want: %v",
						i, got, address)
				}
			}
		}

	}

	return
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
		err     error
	}{
		{
			in:      pubKeys[:3],
			series:  0,
			reqSigs: 2,
			err:     nil,
		},
		{
			in:      pubKeys[:5],
			series:  1,
			reqSigs: 3,
			err:     nil,
		},
		{
			in:      pubKeys[:7],
			series:  2,
			reqSigs: 4,
			err:     nil,
		},
		{
			in:      pubKeys[:9],
			series:  3,
			reqSigs: 5,
			err:     nil,
		},
		// Errors..
		{
			in:     []string{"xpub"},
			series: 99,
			// TODO: get the correct error code
			err: waddrmgr.ManagerError{ErrorCode: 0},
		},
	}

	t.Logf("CreateSeries: Running %d tests", len(tests))
	for testNum, test := range tests {
		err := pool.CreateSeries(uint32(test.series), test.in, test.reqSigs)
		if test.err != nil {
			if err == nil {
				t.Errorf("%d: Expected a test failure and didn't get one", testNum)
			} else {
				rerr := err.(waddrmgr.ManagerError)
				terr := test.err.(waddrmgr.ManagerError)
				if terr.ErrorCode != rerr.ErrorCode {
					t.Errorf("%d: Incorrect type of error passed back: "+
						"want %d got %d", testNum, terr.ErrorCode, rerr.ErrorCode)
				}
				continue
			}
		}
		if err != nil {
			t.Errorf("%d: Cannot create series %d", testNum, test.series)
		}
		if !pool.ExistsSeriesTestsOnly(test.series) {
			t.Errorf("%d: Series %d not in database", testNum, test.series)
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
			pubKeys: pubKeys[:1],
			reqSigs: 1,
		},
		{
			pubKeys:  pubKeys[:1],
			privKeys: privKeys[:1],
			reqSigs:  1,
		},
		{
			pubKeys: pubKeys[:3],
			reqSigs: 2,
		},
		{
			pubKeys:  pubKeys[:3],
			privKeys: privKeys[:1],
			reqSigs:  2,
		},
		{
			pubKeys: pubKeys[:5],
			reqSigs: 3,
		},
		{
			pubKeys:  pubKeys[:7],
			privKeys: privKeys[:2],
			reqSigs:  4,
		},
		// Errors
		// TODO: correct the error codes to their proper values
		//  once actual error codes are in error.go
		{
			pubKeys: []string{"NONSENSE"},
			// not a valid length pub key
			err: waddrmgr.ManagerError{ErrorCode: 0},
		},
		{
			pubKeys:  pubKeys[0:1],
			reqSigs:  2,
			privKeys: []string{"NONSENSE"},
			// not a valid length priv key
			err: waddrmgr.ManagerError{ErrorCode: 0},
		},
		{
			serial: []byte("WRONG"),
			// not enough bytes (under the theoretical minimum)
			sErr: waddrmgr.ManagerError{ErrorCode: 0},
		},
		{
			serial: make([]byte, 10000),
			// too many bytes (over the theoretical maximum)
			sErr: waddrmgr.ManagerError{ErrorCode: 0},
		},
		{
			serial: []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			// not enough bytes (specifically not enough public keys)
			sErr: waddrmgr.ManagerError{ErrorCode: 0},
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
			sErr: waddrmgr.ManagerError{ErrorCode: 0},
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
			sErr: waddrmgr.ManagerError{ErrorCode: 0},
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
			sErr: waddrmgr.ManagerError{ErrorCode: 0},
		},
	}

	var err error

	t.Logf("Serialization: Running %d tests", len(tests))
	for testNum, test := range tests {
		var serialized []byte
		var encryptedPubs, encryptedPrivs [][]byte
		if test.serial == nil {
			encryptedPubs = make([][]byte, len(test.pubKeys))
			for i, pubKey := range test.pubKeys {
				encryptedPubs[i], err = mgr.Encrypt([]byte(pubKey))
				if err != nil {
					t.Errorf("Serialization #%d -  Failed to encrypt public key %v",
						testNum, pubKey)
				}
			}

			encryptedPrivs = make([][]byte, len(test.privKeys))
			for i, privKey := range test.privKeys {
				encryptedPrivs[i], err = mgr.Encrypt([]byte(privKey))
				if err != nil {
					t.Errorf("Serialization #%d -  Failed to encrypt private key %v",
						testNum, privKey)
				}
			}

			serialized, err = waddrmgr.SerializeSeries(test.reqSigs, encryptedPubs, encryptedPrivs)
			if test.err != nil {
				if err == nil {
					t.Errorf("Serialization #%d -  Should have gotten an error and didn't",
						testNum)
				}
				terr := test.err.(waddrmgr.ManagerError)
				rerr := err.(waddrmgr.ManagerError)
				if terr.ErrorCode != rerr.ErrorCode {
					t.Errorf("Serialization #%d -  Incorrect type of error passed back: "+
						"want %d got %d", testNum, terr.ErrorCode, rerr.ErrorCode)
				}
				continue
			}
		} else {
			// shortcut this serialization and pretend we got some other string
			//  that's defined in the test
			serialized = test.serial
		}

		row, err := waddrmgr.DeserializeSeries(serialized)

		if test.sErr != nil {
			if err == nil {
				t.Errorf("Serialization #%d -  Should have gotten an error and didn't",
					testNum)
			}
			terr := test.sErr.(waddrmgr.ManagerError)
			rerr := err.(waddrmgr.ManagerError)
			if terr.ErrorCode != rerr.ErrorCode {
				t.Errorf("Serialization #%d -  Incorrect type of error passed back: "+
					"want %d got %d", testNum, terr.ErrorCode, rerr.ErrorCode)
			}

			continue
		}

		if err != nil {
			t.Errorf("Serialization #%d -  Failed to deserialize %v", testNum, serialized)
		}

		if row.ReqSigs != test.reqSigs {
			t.Errorf("Serialization #%d -  row reqSigs off: want %d got %d",
				testNum, test.reqSigs, row.ReqSigs)
		}

		if len(row.PubKeysEncrypted) != len(test.pubKeys) {
			t.Errorf("Serialization #%d -  Number of pubkeys off: want %d got %d",
				testNum, len(test.pubKeys), len(row.PubKeysEncrypted))
		}

		for i, encryptedPub := range encryptedPubs {
			got := string(row.PubKeysEncrypted[i])

			if got != string(encryptedPub) {
				t.Errorf("Serialization #%d -  Pubkey deserialization not the same: "+
					"want %v got %v", testNum, string(encryptedPub), got)
			}

		}

		if len(row.PrivKeysEncrypted) != len(test.privKeys) {
			t.Errorf("Serialization #%d -  Number of privkeys off: want %d got %d",
				testNum, len(test.privKeys), len(row.PrivKeysEncrypted))
		}

		for i, encryptedPriv := range encryptedPrivs {
			got := string(row.PrivKeysEncrypted[i])

			if got != string(encryptedPriv) {
				t.Errorf("Serialization #%d -  Privkey deserialization not the same: "+
					"want %v got %v", testNum, string(encryptedPriv), got)
			}
		}
	}
}

func TestReplaceSeries(t *testing.T) {
	// TODO
}

func TestEmpowerBranch(t *testing.T) {
	// TODO
}

func TestGetSeries(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()
	err := pool.CreateSeries(0, pubKeys[:3], 2)
	if err != nil {
		t.Fatalf("Failed to create series: %v", err)
	}

	series := pool.GetSeries(0)
	if series == nil {
		t.Fatal("GetSeries() returned nil")
	}
	// TODO: Must check that the series is really the one we expect
}

func TestLoadAllSeries(t *testing.T) {
	tearDown, manager, pool := setUp(t)
	defer tearDown()

	err := pool.CreateSeries(0, pubKeys[:3], 2)
	if err != nil {
		t.Fatalf("Failed to create series: %v", err)
	}
	expectedSeries := pool.GetSeries(0)

	// Ideally we should reset pool.seriesLookup and call LoadAllSeries() manually, but that
	// is a private attribute so we just call LoadVotingPool, which calls LoadAllSeries.
	pool2, err := manager.LoadVotingPool(pool.ID)
	if err != nil {
		t.Fatalf("Failed to load voting pool: %v", err)
	}

	series := pool2.GetSeries(0)
	expectedKeys := expectedSeries.GetPublicKeys()
	keys := series.GetPublicKeys()
	if len(keys) != len(expectedKeys) {
		t.Errorf("Series pubKeys mismatch. Expected %v, got %v", expectedKeys, keys)
	}
	for i, key := range keys {
		if key.String() != expectedKeys[i].String() {
			t.Errorf("Series pubKeys mismatch. Expected %v, got %v", expectedSeries, series)
		}
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
		inKeys := createTestPubKeys(i, 0)
		wantKeys := reverse(inKeys)
		resKeys := waddrmgr.BranchOrder(inKeys, 0)

		if len(resKeys) != len(wantKeys) {
			t.Errorf("BranchOrder failed: returned slice has different length than the argument. Got: %d Exp: %d", len(resKeys), len(inKeys))
			return
		}

		for keyIdx := 0; i < len(inKeys); i++ {
			if resKeys[keyIdx] != wantKeys[keyIdx] {
				fmt.Printf("%p, %p\n", resKeys[i], wantKeys[i])
				t.Errorf("BranchOrder(keys, 0) failed: Exp: %v, Got: %v", wantKeys[i], resKeys[i])
			}
		}
	}
}

func TestBranchOrderNilKeys(t *testing.T) {
	// Test branchorder with nil input and various branch numbers.
	for i := 0; i < 10; i++ {
		res := waddrmgr.BranchOrder(nil, uint32(i))
		if res != nil {
			t.Errorf("Tried to reorder a nil slice of public keys, but got something non-nil back")
		}
	}
}

func TestBranchOrderNonZero(t *testing.T) {
	maxBranch := 5
	maxTail := 4
	// Test branch reordering branch > 0. We test all all branch
	// values in [1,5] in a slice of up to 9 (maxBranch-1 + branch-pivot
	// + maxTail) keys. Hopefully that covers all combinations and
	// edge-cases.

	// we test the case branch := 0 elsewhere
	for branch := 1; branch <= maxBranch; branch++ {
		for j := 0; j <= maxTail; j++ {
			first := createTestPubKeys(branch-1, 0)
			pivot := createTestPubKeys(1, branch)
			last := createTestPubKeys(j, branch+1)

			inKeys := append(append(first, pivot...), last...)

			wantKeys := append(append(pivot, first...), last...)

			resKeys := waddrmgr.BranchOrder(inKeys, uint32(branch))

			if len(resKeys) != len(inKeys) {
				t.Errorf("BranchOrder failed: returned slice has different length than the argument. Got: %d Exp: %d", len(resKeys), len(inKeys))
			}

			for idx := 0; idx < len(inKeys); idx++ {
				if resKeys[idx] != wantKeys[idx] {
					o, w, g := branchErrorFormat(inKeys, wantKeys, resKeys)
					t.Errorf("Branch: %d\nOrig: %v\nWant: %v\nGot:  %v", branch, o, w, g)
				}
			}
		}
	}
}

// branchErrorFormat returns orig, want, got in that order
func branchErrorFormat(orig, want, got []*btcutil.AddressPubKey) ([]int, []int, []int) {
	origOrder := []int{}
	origMap := make(map[*btcutil.AddressPubKey]int)
	for i, key := range orig {
		origMap[key] = i + 1
		origOrder = append(origOrder, i+1)
	}

	wantOrder := []int{}
	for _, key := range want {
		wantOrder = append(wantOrder, origMap[key])
	}

	gotOrder := []int{}
	for _, key := range got {
		gotOrder = append(gotOrder, origMap[key])
	}

	return origOrder, wantOrder, gotOrder
}

func createTestPubKeys(number, offset int) []*btcutil.AddressPubKey {

	net := &btcnet.TestNet3Params
	xpubRaw := "xpub661MyMwAqRbcFwdnYF5mvCBY54vaLdJf8c5ugJTp5p7PqF9J1USgBx12qYMnZ9yUiswV7smbQ1DSweMqu8wn7Jociz4PWkuJ6EPvoVEgMw7"
	xpubKey, _ := hdkeychain.NewKeyFromString(xpubRaw)

	keys := make([]*btcutil.AddressPubKey, number)
	for i := uint32(0); i < uint32(len(keys)); i++ {
		chPubKey, _ := xpubKey.Child(i + uint32(offset))
		pubKey, _ := chPubKey.ECPubKey()
		x, _ := btcutil.NewAddressPubKey(pubKey.SerializeCompressed(), net)
		keys[i] = x
	}
	return keys
}

func TestReverse(t *testing.T) {
	// this basically just tests that the utility function that
	// reverses a bunch of public keys. 11 is a random number
	for numKeys := 0; numKeys < 11; numKeys++ {
		keys := createTestPubKeys(numKeys, 0)
		revRevKeys := reverse(reverse(keys))
		if len(keys) != len(revRevKeys) {
			t.Errorf("Reverse twice the list of pubkeys changed the length. Exp: %d, Got: %d", len(keys), len(revRevKeys))
		}

		for i := 0; i < len(keys); i++ {
			if keys[i] != revRevKeys[i] {
				t.Errorf("Reverse failed: Reverse(Reverse(x)) != x. Exp: %v, Got: %v", keys[i], revRevKeys[i])
			}
		}
	}
}
