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
			in:      []string{pubKey0, pubKey1, pubKey2},
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

func TestCreateSeriesErrors(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	tests := []struct {
		pubKeys []string
		reqSigs uint32
		err     error
		msg     string
	}{
		{
			pubKeys: []string{pubKey0, pubKey1, pubKey2, pubKey0},
			err:     waddrmgr.ManagerError{}, // FIXME: Use specific error type here
			msg:     "Should return error when passed duplicate pubkeys.",
		},
		{
			pubKeys: []string{"invalidxpub"},
			err:     waddrmgr.ManagerError{}, // FIXME: Use specific error type here
			msg:     "Should return error when passed invalid pubkey.",
		},
		{
			pubKeys: []string{privKey0},
			err:     waddrmgr.ManagerError{}, // FIXME: Use specific error type here
			msg:     "Should return error when passed private keys.",
		},
	}

	for i, test := range tests {
		err := pool.CreateSeries(uint32(i), test.pubKeys, test.reqSigs)
		if err == nil {
			str := fmt.Sprintf(test.msg+" pubKeys: %v, reqSigs: %v", test.pubKeys, test.reqSigs)
			t.Errorf(str)
		}
		// TODO: Check that the error matches the expected type.
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
		// TODO: correct the error codes to their proper values
		//  once actual error codes are in error.go
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
					t.Errorf("Serialization #%d -  Failed to encrypt public key %v",
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
					t.Errorf("Serialization #%d -  Failed to encrypt private key %v",
						testNum, privKey)
				}
			}

			serialized, err = waddrmgr.SerializeSeries(test.reqSigs, encryptedPubs, encryptedPrivs)
			if test.err != nil {
				if err == nil {
					t.Errorf("Serialization #%d -  Should have gotten an error and didn't",
						testNum)
					continue
				}
				terr := test.err.(waddrmgr.ManagerError)
				rerr := err.(waddrmgr.ManagerError)
				if terr.ErrorCode != rerr.ErrorCode {
					t.Errorf("Serialization #%d -  Incorrect type of error passed back: "+
						"want %d got %d", testNum, terr.ErrorCode, rerr.ErrorCode)
				}
				continue
			} else if err != nil {
				t.Errorf("Serialization #%d - Error in serialization %v",
					testNum, err)
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
				continue

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
			t.Errorf("Serialization #%d -  Failed to deserialize %v %v", testNum, serialized, err)
			continue

		}

		if row.ReqSigs != test.reqSigs {
			t.Errorf("Serialization #%d -  row reqSigs off: want %d got %d",
				testNum, test.reqSigs, row.ReqSigs)
			continue

		}

		if len(row.PubKeysEncrypted) != len(test.pubKeys) {
			t.Errorf("Serialization #%d -  Number of pubkeys off: want %d got %d",
				testNum, len(test.pubKeys), len(row.PubKeysEncrypted))
			continue

		}

		for i, encryptedPub := range encryptedPubs {
			got := string(row.PubKeysEncrypted[i])

			if got != string(encryptedPub) {
				t.Errorf("Serialization #%d -  Pubkey deserialization not the same: "+
					"want %v got %v", testNum, string(encryptedPub), got)
				continue

			}

		}

		if len(row.PrivKeysEncrypted) != len(row.PubKeysEncrypted) {
			t.Errorf("Serialization #%d -  Number of privkeys not the same as number of pubkeys: pub %d priv %d",
				testNum, len(row.PubKeysEncrypted), len(row.PrivKeysEncrypted))
			continue

		}

		for i, encryptedPriv := range encryptedPrivs {
			got := string(row.PrivKeysEncrypted[i])

			if got != string(encryptedPriv) {
				t.Errorf("Serialization #%d -  Privkey deserialization not the same: "+
					"want %v got %v", testNum, string(encryptedPriv), got)
				continue

			}
		}
	}
}

func TestCannotReplaceEmpoweredSeries(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	var seriesId uint32 = 1

	if err := pool.CreateSeries(seriesId, []string{pubKey0, pubKey1}, 3); err != nil {
		t.Fatalf("Failed to create series", err)
	}

	if err := pool.EmpowerSeries(seriesId, privKey1); err != nil {
		t.Fatalf("Failed to empower series", err)
	}

	if err := pool.ReplaceSeries(seriesId, []string{pubKey3}, 4); err == nil {
		t.Errorf("Replaced an empowered series. That should not be possible", err)
	}

}

func TestReplaceNonExistingSeries(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	pubKeys := []string{pubKey0, pubKey1, pubKey2}
	if err := pool.ReplaceSeries(uint32(1), pubKeys, 3); err == nil {
		t.Errorf("Replaced non-existant series. This should not be possible.")
	}
}

func TestReplaceExistingSeries(t *testing.T) {
	tearDown, _, pool := setUp(t)
	defer tearDown()

	var seriesID uint32 = 1
	origKeys := []string{pubKey0}
	if err := pool.CreateSeries(seriesID, origKeys, 1); err != nil {
		t.Fatalf("Failed to create series", err)
	}

	replacementKeys := waddrmgr.CanonicalKeyOrder([]string{pubKey1, pubKey3})
	var replacementReqSigs uint32 = 2
	if err := pool.ReplaceSeries(seriesID, replacementKeys, replacementReqSigs); err != nil {
		t.Errorf("ReplaceSeries failed: ", err)
	}

	series := pool.GetSeries(seriesID)
	if series == nil {
		t.Fatalf("Got unexpected nil!")
	}

	if len(replacementKeys) != len(series.TstGetPublicKeys()) {
		t.Fatalf("Expected and actual series have different lengths! Exp: %d, Got %d", len(replacementKeys), len(series.TstGetPublicKeys()))
	}

	for i, key := range series.TstGetPublicKeys() {
		if replacementKeys[i] != key.String() {
			t.Fatalf("Replace series failed! Exp: %v, got %v", replacementKeys[i], key.String())
		}
	}
	if replacementReqSigs != series.TstGetReqSigs() {
		t.Fatalf("Replace series failed, required signatures mismatch. Exp: %d, got %d", replacementReqSigs, series.TstGetReqSigs())
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
			err: waddrmgr.ManagerError{ErrorCode: 0},
		},
		{
			seriesID: 0,
			key:      "NONSENSE",
			// invalid private key
			err: waddrmgr.ManagerError{ErrorCode: 0},
		},
		{
			seriesID: 0,
			key:      pubKey5,
			// wrong type of key
			err: waddrmgr.ManagerError{ErrorCode: 0},
		},
		{
			seriesID: 0,
			key:      privKey5,
			// key not corresponding to pub key
			err: waddrmgr.ManagerError{ErrorCode: 0},
		},
	}

	for testNum, test := range tests {
		// add the extended private key to voting pool
		err := pool.EmpowerSeries(test.seriesID, test.key)
		if test.err != nil {
			if err == nil {
				t.Errorf("EmpowerSeries #%d Expected an error and got none", testNum)
				continue
			}
			if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
				t.Errorf("DepositScript #%d wrong error type "+
					"got: %v <%T>, want: %T", testNum, err, err, test.err)
				continue
			}
			rerr := err.(waddrmgr.ManagerError)
			trerr := test.err.(waddrmgr.ManagerError)
			if rerr.ErrorCode != trerr.ErrorCode {
				t.Errorf("DepositScript #%d wrong error code got: %v, want: %v",
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
	rawPubKeys := waddrmgr.CanonicalKeyOrder([]string{pubKey0, pubKey1, pubKey2})
	if err := pool.CreateSeries(0, rawPubKeys, 2); err != nil {
		t.Fatalf("Failed to create series: %v", err)
	}

	series := pool.GetSeries(0)

	if series == nil {
		t.Fatal("GetSeries() returned nil")
	}
	pubKeys := series.TstGetPublicKeys()
	if len(pubKeys) != len(rawPubKeys) {
		t.Errorf("Expected %d public keys, found %d", len(rawPubKeys), len(pubKeys))
	}
	for i, key := range pubKeys {
		if key.String() != rawPubKeys[i] {
			t.Fatalf("Series pubKeys mismatch. Expected %v, got %v", rawPubKeys, pubKeys)
		}
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
			t.Fatalf("Test #%d Series #%d: Failed to create series: %v",
				test.id, series.id, err)
		}

		for _, privKey := range series.privKeys {
			err := pool.EmpowerSeries(series.id, privKey)
			if err != nil {
				t.Fatalf("Test #%d Series #%d: Failed to empower series with privKey"+
					"%v: %v", test.id, series.id, privKey, err)
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
			t.Fatalf("Test #%d: Failed to load voting pool: %v", test.id, err)
		}
		for _, seriesData := range test.series {
			validateLoadAllSeries(t, pool, test.id, seriesData)
		}
	}
}

/*
validateLoadAllSeries checks the following:
1. series exists
2. reqSigs is what we inserted
3. pubkeys and privkeys have the same length
4. pubkeys are what we inserted (length and content)
5. privkeys are what we inserted (length and content)
*/
func validateLoadAllSeries(t *testing.T, pool *waddrmgr.VotingPool, testID int, seriesData seriesRaw) {

	series := pool.GetSeries(seriesData.id)
	if series == nil {
		t.Errorf("Test #%d Series #%d: series not found",
			testID, seriesData.id)
	}

	if seriesData.reqSigs != series.TstGetReqSigs() {
		t.Errorf("Test #%d Series #%d: required sigs are different want %d got %d",
			testID, seriesData.id, seriesData.reqSigs, series.TstGetReqSigs())
	}

	publicKeys := series.TstGetPublicKeys()
	privateKeys := series.TstGetPrivateKeys()
	if len(privateKeys) != len(publicKeys) {
		t.Errorf("Test #%d Series #%d: wrong number of private keys: want %d got %d",
			testID, seriesData.id, len(publicKeys), len(privateKeys))
	}

	if len(publicKeys) != len(seriesData.pubKeys) {
		t.Errorf("Test #%d Series #%d: wrong number of public keys: want %d got %d",
			testID, seriesData.id, len(seriesData.pubKeys), len(publicKeys))
	}
	sortedKeys := waddrmgr.CanonicalKeyOrder(seriesData.pubKeys)
	for i, sortedKey := range sortedKeys {
		if publicKeys[i].String() != sortedKey {
			t.Errorf("Test #%d Series #%d: incorrectly ordered public keys"+
				" at index %d", testID, seriesData.id, i)
		}
	}

	foundPrivKeys := make([]string, 0, len(seriesData.pubKeys))
	for _, privateKey := range privateKeys {
		if privateKey != nil {
			foundPrivKeys = append(foundPrivKeys, privateKey.String())
		}
	}
	if len(foundPrivKeys) != len(seriesData.privKeys) {
		t.Errorf("Test #%d Series #%d: wrong number of non-nil private keys: "+
			"want %v got %v", testID, seriesData.id, seriesData.privKeys,
			foundPrivKeys)
	}
	foundPrivKeys = waddrmgr.CanonicalKeyOrder(foundPrivKeys)
	privKeys := waddrmgr.CanonicalKeyOrder(seriesData.privKeys)
	for i, privKey := range privKeys {
		if privKey != foundPrivKeys[i] {
			t.Errorf("Test #%d Series #%d: didn't find the same private keys "+
				"want %v got %v", testID, seriesData.id, privKeys,
				foundPrivKeys)
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
		inKeys := createTestPubKeys(t, i, 0)
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
			first := createTestPubKeys(t, branch-1, 0)
			pivot := createTestPubKeys(t, 1, branch)
			last := createTestPubKeys(t, j, branch+1)

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
	// this basically just tests that the utility function that
	// reverses a bunch of public keys. 11 is a random number
	for numKeys := 0; numKeys < 11; numKeys++ {
		keys := createTestPubKeys(t, numKeys, 0)
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
