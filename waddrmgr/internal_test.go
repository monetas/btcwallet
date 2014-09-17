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

import (
	"testing"

	"github.com/monetas/bolt"
	"github.com/monetas/btcutil"
	"github.com/monetas/btcwallet/snacl"
)

// TstSetScryptParams allows the scrypt parameters to be set to much lower
// values while the tests are running so they are faster.
func TstSetScryptParams(n, r, p int) {
	scryptN = n
	scryptR = r
	scryptP = p
}

// TstReplaceNewSecretKeyFunc replaces the new secret key generation function
// with a version that intentionally fails.
func TstReplaceNewSecretKeyFunc() {
	newSecretKey = func(passphrase *[]byte) (*snacl.SecretKey, error) {
		return nil, snacl.ErrDecryptFailed
	}
}

// TstResetNewSecretKeyFunc resets the new secret key generation function to
// the original version.
func TstResetNewSecretKeyFunc() {
	newSecretKey = defaultNewSecretKey
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

// EncryptWithCryptoKeyPub allows using the manager's public key for
// encryption. Used in serialization tests.
func (m *Manager) EncryptWithCryptoKeyPub(unencrypted []byte) ([]byte, error) {
	return m.cryptoKeyPub.Encrypt([]byte(unencrypted))
}

// TstEmptySeriesLookup empties the voting pool seriesLookup attribute.
func (vp *VotingPool) TstEmptySeriesLookup() {
	vp.seriesLookup = make(map[uint32]*seriesData)
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

// BranchOrder transparently wraps branchOrder.
func BranchOrder(pks []*btcutil.AddressPubKey, branch uint32) []*btcutil.AddressPubKey {
	return branchOrder(pks, branch)
}

// ExistsSeriesTestsOnly checks whether a series is stored in the database.
// Used by the series creation test.
func (vp *VotingPool) ExistsSeriesTestsOnly(seriesID uint32) (bool, error) {
	var exists bool
	err := vp.manager.db.View(func(mtx *managerTx) error {
		vpBucket := (*bolt.Tx)(mtx).Bucket(votingPoolBucketName).Bucket(vp.ID)
		if vpBucket == nil {
			exists = false
			return nil
		}
		exists = vpBucket.Get(uint32ToBytes(seriesID)) != nil
		return nil
	})
	if err != nil {
		return false, err
	}
	return exists, nil
}

// TstGetRawPublicKeys gets a series public keys in string format.
func (s *seriesData) TstGetRawPublicKeys() []string {
	rawKeys := make([]string, len(s.publicKeys))
	for i, key := range s.publicKeys {
		rawKeys[i] = key.String()
	}
	return rawKeys
}

// TstGetRawPrivateKeys gets a series private keys in string format.
func (s *seriesData) TstGetRawPrivateKeys() []string {
	rawKeys := make([]string, len(s.privateKeys))
	for i, key := range s.privateKeys {
		if key != nil {
			rawKeys[i] = key.String()
		}
	}
	return rawKeys
}

// TstGetReqSigs expose the series reqSigs attribute.
func (s *seriesData) TstGetReqSigs() uint32 {
	return s.reqSigs
}

// TstPutSeries transparently wraps the voting pool putSeries method.
func (vp *VotingPool) TstPutSeries(version, seriesID, reqSigs uint32, inRawPubKeys []string) error {
	return vp.putSeries(version, seriesID, reqSigs, inRawPubKeys)
}

func TestDecryptExtendedKeyCannotDecrypt(t *testing.T) {
	cryptoKey, err := newCryptoKey()
	if err != nil {
		t.Fatalf("Failed to generate cryptokey, %v", err)
	}
	if _, err := decryptExtendedKey(cryptoKey, []byte{}); err == nil {
		t.Errorf("Expected function to fail, but it didn't")
	} else {
		gotErr := err.(ManagerError)
		wantErrCode := ErrorCode(ErrCrypto)
		if gotErr.ErrorCode != wantErrCode {
			t.Errorf("Got %s, want %s", gotErr.ErrorCode, wantErrCode)
		}
	}
}

func TestDecryptExtendedKeyCannotCreateResultKey(t *testing.T) {
	cryptoKey, err := newCryptoKey()
	if err != nil {
		t.Fatalf("Failed to generate cryptokey, %v", err)
	}

	// the plaintext not being base58 encoded triggers the error
	cipherText, err := cryptoKey.Encrypt([]byte("not-base58-encoded"))
	if err != nil {
		t.Fatalf("Failed to encrypt plaintext: %v", err)
	}

	if _, err := decryptExtendedKey(cryptoKey, cipherText); err == nil {
		t.Errorf("Expected function to fail, but it didn't")
	} else {
		gotErr := err.(ManagerError)
		wantErrCode := ErrorCode(ErrKeyChain)
		if gotErr.ErrorCode != wantErrCode {
			t.Errorf("Got %s, want %s", gotErr.ErrorCode, wantErrCode)
		}
	}
}

// Replace Manager.cryptoKeyScript with the given one and calls the given function,
// resetting Manager.cryptoKeyScript to its original value after that.
func RunWithReplacedCryptoKeyScript(mgr *Manager, cryptoKey EncryptorDecryptor, callback func()) {
	orig := mgr.cryptoKeyScript
	defer func() { mgr.cryptoKeyScript = orig }()
	mgr.cryptoKeyScript = cryptoKey
	callback()
}
