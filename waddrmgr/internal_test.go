// Bridge file specifically for tests to expose some functionality from db.go

package waddrmgr

import (
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
