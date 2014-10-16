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

/*
This test file is part of the waddrmgr package rather than than the
waddrmgr_test package so it can bridge access to the internals to properly test
cases which are either not possible or can't reliably be tested via the public
interface. The functions are only exported while the tests are being run.
*/

package waddrmgr

import "github.com/conformal/btcwallet/snacl"

// TstMaxRecentHashes makes the unexported maxRecentHashes constant available
// when tests are run.
var TstMaxRecentHashes = maxRecentHashes

// Replace the Manager.newSecretKey function with the given one and calls
// the callback function. Afterwards the original newSecretKey
// function will be restored.
func TstRunWithReplacedNewSecretKey(callback func()) {
	orig := newSecretKey
	defer func() {
		newSecretKey = orig
	}()
	newSecretKey = func(passphrase *[]byte, config *Options) (*snacl.SecretKey, error) {
		return nil, snacl.ErrDecryptFailed
	}
	callback()
}

// TstCheckPublicPassphrase returns true if the provided public passphrase is
// correct for the manager.
func (m *Manager) TstCheckPublicPassphrase(pubPassphrase []byte) bool {
	secretKey := snacl.SecretKey{Key: &snacl.CryptoKey{}}
	secretKey.Parameters = m.masterKeyPub.Parameters
	err := secretKey.DeriveKey(&pubPassphrase)
	return err == nil
}

// SeriesRow mimics dbSeriesRow defined in db.go .
type SeriesRow struct {
	Version           uint32
	Active            bool
	ReqSigs           uint32
	PubKeysEncrypted  [][]byte
	PrivKeysEncrypted [][]byte
}

// SerializeSeries wraps serializeSeriesRow by passing it a freshly-built
// dbSeriesRow.
func SerializeSeries(version uint32, active bool, reqSigs uint32, pubKeys, privKeys [][]byte) ([]byte, error) {
	row := &dbSeriesRow{
		version:           version,
		active:            active,
		reqSigs:           reqSigs,
		pubKeysEncrypted:  pubKeys,
		privKeysEncrypted: privKeys,
	}
	return serializeSeriesRow(row)
}

// DeserializeSeries wraps deserializeSeriesRow and returns a freshly-built
// SeriesRow.
func DeserializeSeries(serializedSeries []byte) (*SeriesRow, error) {
	row, err := deserializeSeriesRow(serializedSeries)

	if err != nil {
		return nil, err
	}

	return &SeriesRow{
		Version:           row.version,
		Active:            row.active,
		ReqSigs:           row.reqSigs,
		PubKeysEncrypted:  row.pubKeysEncrypted,
		PrivKeysEncrypted: row.privKeysEncrypted,
	}, nil
}

// EncryptWithCryptoKeyPub allows using the manager's crypto key used for
// encryption of public keys.
func (m *Manager) EncryptWithCryptoKeyPub(unencrypted []byte) ([]byte, error) {
	return m.cryptoKeyPub.Encrypt([]byte(unencrypted))
}

// EncryptWithCryptoKeyPriv allows using the manager's crypto key used for
// encryption of private keys.
func (m *Manager) EncryptWithCryptoKeyPriv(unencrypted []byte) ([]byte, error) {
	return m.cryptoKeyPriv.Encrypt([]byte(unencrypted))
}
