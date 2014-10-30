package waddrmgr

import (
	"fmt"

	"github.com/conformal/bolt"
)

// CryptoKeyType is used to differentiate between different kinds of
// crypto keys.
type CryptoKeyType byte

// Crypto key types.
const (
	CKTPrivate CryptoKeyType = iota
	CKTScript
	CKTPublic
)

// Encrypt in using the crypto key type specified by keyType.
func (m *Manager) Encrypt(keyType CryptoKeyType, in []byte) ([]byte, error) {
	// Encryption must be performed under the manager mutex since the
	// keys are cleared when the manager is locked.
	m.mtx.Lock()
	defer m.mtx.Unlock()

	cryptoKey, err := m.selectCryptoKey(keyType)
	if err != nil {
		return nil, err
	}

	encrypted, err := cryptoKey.Encrypt(in)
	if err != nil {
		return nil, managerError(ErrCrypto, "failed to encrypt", err)
	}
	return encrypted, nil
}

// Decrypt in using the crypto key type specified by keyType.
func (m *Manager) Decrypt(keyType CryptoKeyType, in []byte) ([]byte, error) {
	// Encryption must be performed under the manager mutex since the
	// keys are cleared when the manager is locked.
	m.mtx.Lock()
	defer m.mtx.Unlock()

	cryptoKey, err := m.selectCryptoKey(keyType)
	if err != nil {
		return nil, err
	}

	encrypted, err := cryptoKey.Decrypt(in)
	if err != nil {
		return nil, managerError(ErrCrypto, "failed to decrypt", err)
	}
	return encrypted, nil
}

// selectCryptoKey selects the appropriate crypto key based on the
// keyType. If the keyType is invalid or the key requested requires
// the manager to be unlocked and it isn't, an error is returned.
func (m *Manager) selectCryptoKey(keyType CryptoKeyType) (EncryptorDecryptor, error) {
	if keyType == CKTPrivate || keyType == CKTScript {
		// The manager must be unlocked to encrypt with the private keys.
		if m.locked || m.watchingOnly {
			return nil, managerError(ErrLocked, errLocked, nil)
		}
	}

	var cryptoKey EncryptorDecryptor
	switch keyType {
	case CKTPrivate:
		cryptoKey = m.cryptoKeyPriv

	case CKTScript:
		cryptoKey = m.cryptoKeyScript

	case CKTPublic:
		cryptoKey = m.cryptoKeyPub

	default:
		return nil, managerError(ErrInvalidKeyType, "invalid key type",
			nil)
	}

	return cryptoKey, nil
}

// Added here since we do not have access to the managerTx type
func PutVotingPool(m *Manager, poolID []byte) error {
	return m.db.Update(func(tx *managerTx) error {
		return tx.PutVotingPool(poolID)
	})
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
