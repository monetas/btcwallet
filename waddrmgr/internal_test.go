// Bridge file specifically for tests to expose some functionality from db.go

package waddrmgr

import (
	"github.com/monetas/bolt"
	"github.com/monetas/btcutil"
)

type SeriesRow struct {
	ReqSigs           uint32
	PubKeysEncrypted  [][]byte
	PrivKeysEncrypted [][]byte
}

func (m *Manager) Encrypt(unencrypted []byte) ([]byte, error) {
	return m.cryptoKeyPub.Encrypt([]byte(unencrypted))
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
	err := vp.manager.db.View(func(tx *managerTx) error {
		exists = tx.ExistsSeries(vp.ID, seriesID)
		return nil
	})
	if err != nil {
		// If there was an error while retrieving the series, we should
		// return an error, but we're too lazy for that.
		return false
	}
	return exists
}

func (mtx *managerTx) ExistsSeries(votingPoolID []byte, ID uint32) bool {
	vpBucket := (*bolt.Tx)(mtx).Bucket(votingPoolBucketName).Bucket(votingPoolID)
	if vpBucket == nil {
		return false
	}
	return vpBucket.Get(uint32ToBytes(ID)) != nil
}
