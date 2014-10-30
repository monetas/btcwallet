package waddrmgr

import (
	"fmt"

	"github.com/conformal/bolt"
)

func (m *Manager) EncryptPub(in []byte) ([]byte, error) {
	return m.cryptoKeyPub.Encrypt(in)
}

func (m *Manager) DecryptPub(in []byte) ([]byte, error) {
	return m.cryptoKeyPub.Decrypt(in)
}

func (m *Manager) EncryptPriv(in []byte) ([]byte, error) {
	return m.cryptoKeyPriv.Encrypt(in)
}

func (m *Manager) DecryptPriv(in []byte) ([]byte, error) {
	return m.cryptoKeyPriv.Decrypt(in)
}

func (m *Manager) EncryptScript(in []byte) ([]byte, error) {
	return m.cryptoKeyScript.Encrypt(in)
}

func (m *Manager) DecryptScript(in []byte) ([]byte, error) {
	return m.cryptoKeyScript.Decrypt(in)
}

// Added here since we do not have access to the managerTx type
func PutVotingPool(m *Manager, poolID []byte) error {
	return m.db.Update(func(tx *managerTx) error {
		return tx.PutVotingPool(poolID)
	})
}

// Move the function to the manager?
func NewScriptAddress(m *Manager, account uint32, scriptHash, scriptEncrypted []byte) (ManagedScriptAddress, error) {
	return newScriptAddress(m, account, scriptHash, scriptEncrypted)

}

// Added here since we do not have access to the managerTx type
func PutSeries(m *Manager, votingPoolID []byte, version, ID uint32, active bool, reqSigs uint32, pubKeysEncrypted, privKeysEncrypted [][]byte) error {

	return m.db.Update(func(tx *managerTx) error {
		return tx.PutSeries(votingPoolID, version, ID, active,
			reqSigs, pubKeysEncrypted, privKeysEncrypted)
	})
}

// Added here since we do not have access to the managerTx type
func ExistsVotingPool(m *Manager, poolID []byte) error {
	return m.db.View(func(tx *managerTx) error {
		if exists := tx.ExistsVotingPool(poolID); !exists {
			str := fmt.Sprintf("unable to find voting pool %v in db", poolID)
			return managerError(ErrVotingPoolNotExists, str, nil)
		}
		return nil
	})
}

// DBSeries mimics dbSeriesRow defined in waddrmgr/db.go.
type DBSeries struct {
	Version           uint32
	Active            bool
	ReqSigs           uint32
	PubKeysEncrypted  [][]byte
	PrivKeysEncrypted [][]byte
}

// Added here since we do not have access to the managerTx type
func LoadAllSeries(m *Manager, ID []byte) ([]DBSeries, error) {
	var allSeries map[uint32]*dbSeriesRow
	err := m.db.View(func(tx *managerTx) error {
		var err error
		allSeries, err = tx.LoadAllSeries(ID)
		return err
	})
	if err != nil {
		return nil, err
	}

	series := make([]DBSeries, len(allSeries))
	for idx, seriesRow := range allSeries {
		s := DBSeries{
			Version:           seriesRow.version,
			Active:            seriesRow.active,
			ReqSigs:           seriesRow.reqSigs,
			PubKeysEncrypted:  seriesRow.pubKeysEncrypted,
			PrivKeysEncrypted: seriesRow.privKeysEncrypted,
		}
		series[idx] = s
	}
	return series, nil
}

// Stuff added solely to satisfy tests

// TstExistsSeries checks whether a series is stored in the database.
// Used by the series creation test.
func ExistsSeries(mgr *Manager, poolID []byte, seriesID uint32) (bool, error) {
	var exists bool
	err := mgr.db.View(func(mtx *managerTx) error {
		vpBucket := (*bolt.Tx)(mtx).Bucket(votingPoolBucketName).Bucket(poolID)
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
