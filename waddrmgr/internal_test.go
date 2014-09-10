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

// TstResetNewSecretKeyFunc resets the new secret key generation function to the
// original version.
func TstResetNewSecretKeyFunc() {
	newSecretKey = defaultNewSecretKey
}

// TstCheckPublicPassphrase return true if the provided public passphrase is
// correct for the manager.
func (m *Manager) TstCheckPublicPassphrase(pubPassphrase []byte) bool {
	secretKey := snacl.SecretKey{Key: &snacl.CryptoKey{}}
	secretKey.Parameters = m.masterKeyPub.Parameters
	err := secretKey.DeriveKey(&pubPassphrase)
	return err == nil
}

type SeriesRow struct {
	ReqSigs           uint32
	PubKeysEncrypted  [][]byte
	PrivKeysEncrypted [][]byte
}

func (m *Manager) EncryptWithCryptoKeyPub(unencrypted []byte) ([]byte, error) {
	return m.cryptoKeyPub.Encrypt([]byte(unencrypted))
}

func (vp *VotingPool) TstEmptySeriesLookup() {
	vp.seriesLookup = make(map[uint32]*seriesData)
}

func SerializeSeries(reqSigs uint32, pubKeys, privKeys [][]byte) ([]byte, error) {
	row := &dbSeriesRow{
		reqSigs:           reqSigs,
		pubKeysEncrypted:  pubKeys,
		privKeysEncrypted: privKeys,
	}
	return serializeSeriesRow(row)
}

func DeserializeSeries(serializedSeries []byte) (*SeriesRow, error) {
	row, err := deserializeSeriesRow(serializedSeries)

	if err != nil {
		return nil, err
	}

	return &SeriesRow{
		ReqSigs:           row.reqSigs,
		PubKeysEncrypted:  row.pubKeysEncrypted,
		PrivKeysEncrypted: row.privKeysEncrypted,
	}, nil
}

func BranchOrder(pks []*btcutil.AddressPubKey, branch uint32) []*btcutil.AddressPubKey {
	return branchOrder(pks, branch)
}

func (vp *VotingPool) ExistsSeriesTestsOnly(seriesID uint32) bool {
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
		// If there was an error while retrieving the series, we should
		// return an error, but we're too lazy for that.
		return false
	}
	return exists
}

func (s *seriesData) TstGetRawPublicKeys() []string {
	rawKeys := make([]string, len(s.publicKeys))
	for i, key := range s.publicKeys {
		rawKeys[i] = key.String()
	}
	return rawKeys
}

func (s *seriesData) TstGetRawPrivateKeys() []string {
	rawKeys := make([]string, len(s.privateKeys))
	for i, key := range s.privateKeys {
		if key != nil {
			rawKeys[i] = key.String()
		}
	}
	return rawKeys
}

func (s *seriesData) TstGetReqSigs() uint32 {
	return s.reqSigs
}

func (vp *VotingPool) TstPutSeries(seriesID uint32, inRawPubKeys []string, reqSigs uint32) error {
	return vp.putSeries(seriesID, inRawPubKeys, reqSigs)
}
