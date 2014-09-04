// Bridge file specifically for tests to expose some functionality from db.go

package waddrmgr

import (
	"github.com/monetas/bolt"
	"github.com/monetas/btcutil"
	"github.com/monetas/btcutil/hdkeychain"
)

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

func (s *seriesData) TstGetPublicKeys() []*hdkeychain.ExtendedKey {
	return s.publicKeys
}

func (s *seriesData) TstGetPrivateKeys() []*hdkeychain.ExtendedKey {
	return s.privateKeys
}

func (s *seriesData) TstGetReqSigs() uint32 {
	return s.reqSigs
}
