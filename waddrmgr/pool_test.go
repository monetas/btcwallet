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
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/conformal/btcnet"
	"github.com/conformal/btcwallet/votingpool"
	"github.com/conformal/btcwallet/waddrmgr"
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

var (
	encryptPub = func(m *waddrmgr.Manager) func([]byte) ([]byte, error) {
		return func(in []byte) ([]byte, error) {
			return m.Encrypt(waddrmgr.CKTPublic, in)
		}
	}

	encryptPriv = func(m *waddrmgr.Manager) func([]byte) ([]byte, error) {
		return func(in []byte) ([]byte, error) {
			return m.Encrypt(waddrmgr.CKTPrivate, in)
		}
	}
)

func setUp(t *testing.T) (tearDownFunc func(), mgr *waddrmgr.Manager, pool *votingpool.Pool) {
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
		&btcnet.MainNetParams, fastScrypt)
	if err != nil {
		t.Fatalf("Failed to create Manager: %v", err)
	}
	pool, err = votingpool.Create(mgr, []byte{0x00})
	if err != nil {
		t.Fatalf("Voting Pool creation failed: %v", err)
	}
	tearDownFunc = func() {
		os.Remove(file)
		mgr.Close()
	}
	return tearDownFunc, mgr, pool
}

func TestSerializationErrors(t *testing.T) {
	tearDown, mgr, _ := setUp(t)
	defer tearDown()

	tests := []struct {
		version  uint32
		pubKeys  []string
		privKeys []string
		reqSigs  uint32
		err      waddrmgr.ErrorCode
	}{
		{
			version: 2,
			pubKeys: []string{pubKey0, pubKey1, pubKey2},
			err:     waddrmgr.ErrSeriesVersion,
		},
		{
			pubKeys: []string{"NONSENSE"},
			// Not a valid length public key.
			err: waddrmgr.ErrSeriesStorage,
		},
		{
			pubKeys:  []string{pubKey0, pubKey1, pubKey2},
			privKeys: []string{privKey0},
			// The number of public and private keys should be the same.
			err: waddrmgr.ErrSeriesStorage,
		},
		{
			pubKeys:  []string{pubKey0},
			privKeys: []string{"NONSENSE"},
			// Not a valid length private key.
			err: waddrmgr.ErrSeriesStorage,
		},
	}

	// We need to unlock the manager in order to encrypt with the
	// private key.
	mgr.Unlock(privPassphrase)

	active := true
	for testNum, test := range tests {
		encryptedPubs, err := encryptKeys(test.pubKeys, encryptPub(mgr))
		if err != nil {
			t.Fatalf("Test #%d - Error encrypting pubkeys: %v", testNum, err)
		}
		encryptedPrivs, err := encryptKeys(test.privKeys, encryptPriv(mgr))
		if err != nil {
			t.Fatalf("Test #%d - Error encrypting privkeys: %v", testNum, err)
		}

		_, err = waddrmgr.SerializeSeries(
			test.version, active, test.reqSigs, encryptedPubs, encryptedPrivs)

		checkManagerError(t, fmt.Sprintf("Test #%d", testNum), err, test.err)
	}
}

func TestSerialization(t *testing.T) {
	tearDown, mgr, _ := setUp(t)
	defer tearDown()

	tests := []struct {
		version  uint32
		active   bool
		pubKeys  []string
		privKeys []string
		reqSigs  uint32
	}{
		{
			version: 1,
			active:  true,
			pubKeys: []string{pubKey0},
			reqSigs: 1,
		},
		{
			version:  0,
			active:   false,
			pubKeys:  []string{pubKey0},
			privKeys: []string{privKey0},
			reqSigs:  1,
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
	}

	// We need to unlock the manager in order to encrypt with the
	// private key.
	mgr.Unlock(privPassphrase)

	for testNum, test := range tests {
		encryptedPubs, err := encryptKeys(test.pubKeys, encryptPub(mgr))
		if err != nil {
			t.Fatalf("Test #%d - Error encrypting pubkeys: %v", testNum, err)
		}
		encryptedPrivs, err := encryptKeys(test.privKeys, encryptPriv(mgr))
		if err != nil {
			t.Fatalf("Test #%d - Error encrypting privkeys: %v", testNum, err)
		}

		serialized, err := waddrmgr.SerializeSeries(
			test.version, test.active, test.reqSigs, encryptedPubs, encryptedPrivs)
		if err != nil {
			t.Fatalf("Test #%d - Error in serialization %v", testNum, err)
		}

		row, err := waddrmgr.DeserializeSeries(serialized)
		if err != nil {
			t.Fatalf("Test #%d - Failed to deserialize %v %v", testNum, serialized, err)
		}

		// TODO: Move all of these checks into one or more separate functions.
		if row.Version != test.version {
			t.Errorf("Serialization #%d - version mismatch: got %d want %d",
				testNum, row.Version, test.version)
		}

		if row.Active != test.active {
			t.Errorf("Serialization #%d - active mismatch: got %d want %d",
				testNum, row.Active, test.active)
		}

		if row.ReqSigs != test.reqSigs {
			t.Errorf("Serialization #%d - row reqSigs off. Got %d, want %d",
				testNum, row.ReqSigs, test.reqSigs)
		}

		if len(row.PubKeysEncrypted) != len(test.pubKeys) {
			t.Errorf("Serialization #%d - Wrong no. of pubkeys. Got %d, want %d",
				testNum, len(row.PubKeysEncrypted), len(test.pubKeys))
		}

		for i, encryptedPub := range encryptedPubs {
			got := string(row.PubKeysEncrypted[i])

			if got != string(encryptedPub) {
				t.Errorf("Serialization #%d - Pubkey deserialization. Got %v, want %v",
					testNum, got, string(encryptedPub))
			}
		}

		if len(row.PrivKeysEncrypted) != len(row.PubKeysEncrypted) {
			t.Errorf("Serialization #%d - no. privkeys (%d) != no. pubkeys (%d)",
				testNum, len(row.PrivKeysEncrypted), len(row.PubKeysEncrypted))
		}

		for i, encryptedPriv := range encryptedPrivs {
			got := string(row.PrivKeysEncrypted[i])

			if got != string(encryptedPriv) {
				t.Errorf("Serialization #%d - Privkey deserialization. Got %v, want %v",
					testNum, got, string(encryptedPriv))
			}
		}
	}
}

func TestDeserializationErrors(t *testing.T) {
	tearDown, _, _ := setUp(t)
	defer tearDown()

	tests := []struct {
		serialized []byte
		err        waddrmgr.ErrorCode
	}{
		{
			serialized: make([]byte, 1000000),
			// Too many bytes (over waddrmgr.seriesMaxSerial).
			err: waddrmgr.ErrSeriesStorage,
		},
		{
			serialized: make([]byte, 10),
			// Not enough bytes (under waddrmgr.seriesMinSerial).
			err: waddrmgr.ErrSeriesStorage,
		},
		{
			serialized: []byte{
				1, 0, 0, 0, // 4 bytes (version)
				0,          // 1 byte (active)
				2, 0, 0, 0, // 4 bytes (reqSigs)
				3, 0, 0, 0, // 4 bytes (nKeys)
			},
			// Here we have the constant data but are missing any public/private keys.
			err: waddrmgr.ErrSeriesStorage,
		},
		{
			serialized: []byte{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			// Unsupported version.
			err: waddrmgr.ErrSeriesVersion,
		},
	}

	for testNum, test := range tests {
		_, err := waddrmgr.DeserializeSeries(test.serialized)

		checkManagerError(t, fmt.Sprintf("Test #%d", testNum), err, test.err)
	}
}

func encryptKeys(keys []string, cryptoFunc func([]byte) ([]byte, error)) ([][]byte, error) {
	encryptedKeys := make([][]byte, len(keys))
	var err error
	for i, key := range keys {
		if key == "" {
			encryptedKeys[i] = nil
		} else {
			encryptedKeys[i], err = cryptoFunc([]byte(key))
		}
		if err != nil {
			return nil, err
		}
	}
	return encryptedKeys, nil
}
