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

package waddrmgr

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/conformal/bolt"
	"github.com/conformal/fastsha256"
)

var ErrNotImplemented = errors.New("not implemented")

const (
	// lastestDbVersion is the most recent database version.
	lastestDbVersion = 1
)

// syncStatus represents a address synchronization status stored in the
// database.
type syncStatus uint8

// These constants define the various supported sync status types.
const (
	ssNone    syncStatus = 0 // not iota as they need to be stable for db
	ssPartial syncStatus = 1
	ssFull    syncStatus = 2
)

// addressType represents a type of address stored in the database.
type addressType uint8

// These constants define the various supported address types.
const (
	atChain  addressType = 0 // not iota as they need to be stable for db
	atImport addressType = 1
	atScript addressType = 2
)

// dbAccountRow houses information stored about an account in the database.
type dbAccountRow struct {
	pubKeyEncrypted   []byte
	privKeyEncrypted  []byte
	nextExternalIndex uint32
	nextInternalIndex uint32
	name              string
}

// dbAddressRow houses common information stored about an address in the
// database.
type dbAddressRow struct {
	addrType   addressType
	account    uint32
	addTime    uint64
	syncStatus syncStatus
	rawData    []byte // Varies based on address type field.
}

// dbChainAddressRow houses additional information stored about a chained
// address in the database.
type dbChainAddressRow struct {
	dbAddressRow
	branch uint32
	index  uint32
}

// dbImportedAddressRow houses additional information stored about an imported
// public key address in the database.
type dbImportedAddressRow struct {
	dbAddressRow
	encryptedPubKey  []byte
	encryptedPrivKey []byte
}

// dbImportedAddressRow houses additional information stored about a script
// address in the database.
type dbScriptAddressRow struct {
	dbAddressRow
	encryptedHash   []byte
	encryptedScript []byte
}

// Key names for various database fields.
var (
	// Bucket names.
	acctBucketName        = []byte("acct")
	addrBucketName        = []byte("addr")
	mainBucketName        = []byte("main")
	addrAcctIdxBucketName = []byte("addracctidx")

	// Db related key names (main bucket).
	dbVersionName    = []byte("dbver")
	dbCreateDateName = []byte("dbcreated")

	// Crypto related key names (main bucket).
	masterPrivKeyName   = []byte("mpriv")
	masterPubKeyName    = []byte("mpub")
	cryptoPrivKeyName   = []byte("cpriv")
	cryptoPubKeyName    = []byte("cpub")
	cryptoScriptKeyName = []byte("cscript")
	watchingOnlyName    = []byte("watchonly")

	// Account related key names (account && account->accountNum bucket).
	acctNumAcctsName      = []byte("numaccts")
	acctPrivKeyName       = []byte("priv")
	acctPubKeyName        = []byte("pub")
	acctExternalIndexName = []byte("extidx")
	acctInternalIndexName = []byte("intidx")
	acctNameName          = []byte("name")
)

// managerTx represents a database transaction on which all database reads and
// writes occur.
type managerTx bolt.Tx

// FetchMasterKeyParams loads the master key parameters needed to derive them
// (when given the correct user-supplied passphrase) from the database.  Either
// returned value can be nil, but in practice only the private key params will
// be nil for a watching-only database.
func (mtx *managerTx) FetchMasterKeyParams() ([]byte, []byte, error) {
	bucket := (*bolt.Tx)(mtx).Bucket(mainBucketName)

	// Load the master public key parameters.  Required.
	val := bucket.Get(masterPubKeyName)
	if val == nil {
		str := "required master public key parameters not stored in " +
			"database"
		return nil, nil, managerError(ErrDatabase, str, nil)
	}
	pubParams := make([]byte, len(val))
	copy(pubParams, val)

	// Load the master private key parameters if they were stored.
	var privParams []byte
	val = bucket.Get(masterPrivKeyName)
	if val != nil {
		privParams = make([]byte, len(val))
		copy(privParams, val)
	}

	return pubParams, privParams, nil
}

// PutMasterKeyParams stores the master key parameters needed to derive them
// to the database.  Either parameter can be nil in which case no value is
// written for the parameter.
func (mtx *managerTx) PutMasterKeyParams(pubParams, privParams []byte) error {
	bucket := (*bolt.Tx)(mtx).Bucket(mainBucketName)

	if privParams != nil {
		err := bucket.Put(masterPrivKeyName, privParams)
		if err != nil {
			str := "failed to store master private key parameters"
			return managerError(ErrDatabase, str, err)
		}
	}

	if pubParams != nil {
		err := bucket.Put(masterPubKeyName, pubParams)
		if err != nil {
			str := "failed to store master public key parameters"
			return managerError(ErrDatabase, str, err)
		}
	}

	return nil
}

// FetchCryptoKeys loads the encrypted crypto keys which are in turn used to
// protect the extended keys, imported keys, and scripts.  Any of the returned
// values can be nil, but in practice only the crypto private and script keys
// will be nil for a watching-only database.
func (mtx *managerTx) FetchCryptoKeys() ([]byte, []byte, []byte, error) {
	bucket := (*bolt.Tx)(mtx).Bucket(mainBucketName)

	// Load the crypto public key parameters.  Required.
	val := bucket.Get(cryptoPubKeyName)
	if val == nil {
		str := "required encrypted crypto public not stored in database"
		return nil, nil, nil, managerError(ErrDatabase, str, nil)
	}
	pubKey := make([]byte, len(val))
	copy(pubKey, val)

	// Load the crypto private key parameters if they were stored.
	var privKey []byte
	val = bucket.Get(cryptoPrivKeyName)
	if val != nil {
		privKey = make([]byte, len(val))
		copy(privKey, val)
	}

	// Load the crypto script key parameters if they were stored.
	var scriptKey []byte
	val = bucket.Get(cryptoScriptKeyName)
	if val != nil {
		scriptKey = make([]byte, len(val))
		copy(scriptKey, val)
	}

	return pubKey, privKey, scriptKey, nil
}

// PutCryptoKeys stores the encrypted crypto keys which are in turn used to
// protect the extended and imported keys.  Either parameter can be nil in which
// case no value is written for the parameter.
func (mtx *managerTx) PutCryptoKeys(pubKeyEncrypted, privKeyEncrypted, scriptKeyEncrypted []byte) error {
	bucket := (*bolt.Tx)(mtx).Bucket(mainBucketName)

	if pubKeyEncrypted != nil {
		err := bucket.Put(cryptoPubKeyName, pubKeyEncrypted)
		if err != nil {
			str := "failed to store encrypted crypto public key"
			return managerError(ErrDatabase, str, err)
		}
	}

	if privKeyEncrypted != nil {
		err := bucket.Put(cryptoPrivKeyName, privKeyEncrypted)
		if err != nil {
			str := "failed to store encrypted crypto private key"
			return managerError(ErrDatabase, str, err)
		}
	}

	if scriptKeyEncrypted != nil {
		err := bucket.Put(cryptoScriptKeyName, scriptKeyEncrypted)
		if err != nil {
			str := "failed to store encrypted crypto script key"
			return managerError(ErrDatabase, str, err)
		}
	}

	return nil
}

// FetchWatchingOnly loads the watching-only flag from the database.
func (mtx *managerTx) FetchWatchingOnly() (bool, error) {
	bucket := (*bolt.Tx)(mtx).Bucket(mainBucketName)
	buf := bucket.Get(watchingOnlyName)
	if len(buf) != 1 {
		str := "malformed watching-only flag stored in database"
		return false, managerError(ErrDatabase, str, nil)
	}

	return buf[0] != 0, nil
}

// PutWatchingOnly stores the watching-only flag to the database.
func (mtx *managerTx) PutWatchingOnly(watchingOnly bool) error {
	bucket := (*bolt.Tx)(mtx).Bucket(mainBucketName)
	var encoded byte
	if watchingOnly {
		encoded = 1
	}

	if err := bucket.Put(watchingOnlyName, []byte{encoded}); err != nil {
		str := "failed to store wathcing only flag"
		return managerError(ErrDatabase, str, err)
	}
	return nil
}

// accountKey returns the account key to use in the database for a given account
// number.
func accountKey(account uint32) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], account)
	return buf[:]
}

// FetchAccountInfo loads information about the passed account from the
// database.
func (mtx *managerTx) FetchAccountInfo(account uint32) (*dbAccountRow, error) {
	// The returned bytes are only valid during the bolt transaction, so
	// make a copy of the data that is returned.

	bucket := (*bolt.Tx)(mtx).Bucket(acctBucketName)
	bucket = bucket.Bucket(accountKey(account))
	if bucket == nil {
		str := fmt.Sprintf("account %d is invalid", account)
		return nil, managerError(ErrInvalidAccount, str, nil)
	}

	// Load the encrypted public key for the account.
	var info dbAccountRow
	val := bucket.Get(acctPubKeyName)
	if val == nil {
		str := fmt.Sprintf("required account %d encrypted public key "+
			"  not stored in database", account)
		return nil, managerError(ErrDatabase, str, nil)
	}
	info.pubKeyEncrypted = make([]byte, len(val))
	copy(info.pubKeyEncrypted, val)

	// Load the encrypted private key for the account.  It is not required
	// (watching-only mode).
	val = bucket.Get(acctPrivKeyName)
	if val != nil {
		info.privKeyEncrypted = make([]byte, len(val))
		copy(info.privKeyEncrypted, val)
	}

	val = bucket.Get(acctExternalIndexName)
	if val == nil {
		str := fmt.Sprintf("required account %d external index not "+
			"stored in database", account)
		return nil, managerError(ErrDatabase, str, nil)
	}
	info.nextExternalIndex = binary.LittleEndian.Uint32(val)

	val = bucket.Get(acctInternalIndexName)
	if val == nil {
		str := fmt.Sprintf("required account %d internal index not "+
			"stored in database", account)
		return nil, managerError(ErrDatabase, str, nil)
	}
	info.nextInternalIndex = binary.LittleEndian.Uint32(val)

	val = bucket.Get(acctNameName)
	if val == nil {
		str := fmt.Sprintf("required account %d name not stored in "+
			"database", account)
		return nil, managerError(ErrDatabase, str, nil)
	}
	info.name = string(val)

	return &info, nil
}

// PutAccountInfo stores the provided account information to the database.
func (mtx *managerTx) PutAccountInfo(account uint32, row *dbAccountRow) error {
	// Get existing bucket for the account or create one if needed.
	tx := (*bolt.Tx)(mtx)
	bucket := tx.Bucket(acctBucketName).Bucket(accountKey(account))
	if bucket == nil {
		bucket = tx.Bucket(acctBucketName)
		newB, err := bucket.CreateBucket(accountKey(account))
		if err != nil {
			str := fmt.Sprintf("failed to create account bucket %d",
				account)
			return managerError(ErrDatabase, str, err)
		}
		bucket = newB
	}

	// Save the public encrypted key.
	err := bucket.Put(acctPubKeyName, row.pubKeyEncrypted)
	if err != nil {
		str := fmt.Sprintf("failed to store account %d encrypted "+
			"public key", account)
		return managerError(ErrDatabase, str, err)
	}

	// Only write the private key if it's non-nil.
	if row.privKeyEncrypted != nil {
		err := bucket.Put(acctPrivKeyName, row.privKeyEncrypted)
		if err != nil {
			str := fmt.Sprintf("failed to store account %d "+
				"encrypted private key", account)
			return managerError(ErrDatabase, str, err)
		}
	}

	// Save the last external index.
	var val [4]byte
	binary.LittleEndian.PutUint32(val[:], row.nextExternalIndex)
	err = bucket.Put(acctExternalIndexName, val[:])
	if err != nil {
		str := fmt.Sprintf("failed to store account %d external index",
			account)
		return managerError(ErrDatabase, str, err)
	}

	// Save the last internal index.
	binary.LittleEndian.PutUint32(val[:], row.nextInternalIndex)
	err = bucket.Put(acctInternalIndexName, val[:])
	if err != nil {
		str := fmt.Sprintf("failed to store account %d internal index",
			account)
		return managerError(ErrDatabase, str, err)
	}

	// Save the account name.
	err = bucket.Put(acctNameName, []byte(row.name))
	if err != nil {
		str := fmt.Sprintf("failed to store account %d name", account)
		return managerError(ErrDatabase, str, err)
	}

	return nil
}

// FetchNumAccounts loads the number of accounts that have been created from
// the database.
func (mtx *managerTx) FetchNumAccounts() (uint32, error) {
	bucket := (*bolt.Tx)(mtx).Bucket(acctBucketName)

	val := bucket.Get(acctNumAcctsName)
	if val == nil {
		str := "required num accounts not stored in database"
		return 0, managerError(ErrDatabase, str, nil)
	}

	return binary.LittleEndian.Uint32(val), nil
}

// PutNumAccounts stores the number of accounts that have been created to the
// database.
func (mtx *managerTx) PutNumAccounts(numAccounts uint32) error {
	bucket := (*bolt.Tx)(mtx).Bucket(acctBucketName)

	var val [4]byte
	binary.LittleEndian.PutUint32(val[:], numAccounts)
	err := bucket.Put(acctNumAcctsName, val[:])
	if err != nil {
		str := "failed to store num accounts"
		return managerError(ErrDatabase, str, err)
	}

	return nil
}

// fetchAddressRow loads address information for the provided address id from
// the database.  This is used as a common base for the various address types
// to load the common information.

// deserializeAddressRow deserializes the passed serialized address information.
// This is used as a common base for the various address types to deserialize
// the common parts.
func (mtx *managerTx) deserializeAddressRow(addressID, serializedAddress []byte) (*dbAddressRow, error) {
	// The serialized address format is:
	//   <addrType><account><addedTime><syncStatus><rawdata>
	//
	// 1 byte addrType + 4 bytes account + 8 bytes addTime + 1 byte
	// syncStatus + 4 bytes raw data length + raw data

	// Given the above, the length of the entry must be at a minimum
	// the constant value sizes.
	if len(serializedAddress) < 18 {
		str := fmt.Sprintf("malformed serialized address for key %s",
			addressID)
		return nil, managerError(ErrDatabase, str, nil)
	}

	row := dbAddressRow{}
	row.addrType = addressType(serializedAddress[0])
	row.account = binary.LittleEndian.Uint32(serializedAddress[1:5])
	row.addTime = binary.LittleEndian.Uint64(serializedAddress[5:13])
	row.syncStatus = syncStatus(serializedAddress[13])
	rdlen := binary.LittleEndian.Uint32(serializedAddress[14:18])
	row.rawData = make([]byte, rdlen)
	copy(row.rawData, serializedAddress[18:18+rdlen])

	return &row, nil
}

// serializeAddressRow returns the serialization of the passed address row.
func (mtx *managerTx) serializeAddressRow(row *dbAddressRow) []byte {
	// The serialized address format is:
	//   <addrType><account><addedTime><syncStatus><commentlen><comment>
	//   <rawdata>
	//
	// 1 byte addrType + 4 bytes account + 8 bytes addTime + 1 byte
	// syncStatus + 4 bytes raw data length + raw data
	rdlen := len(row.rawData)
	buf := make([]byte, 18+rdlen)
	buf[0] = byte(row.addrType)
	binary.LittleEndian.PutUint32(buf[1:5], row.account)
	binary.LittleEndian.PutUint64(buf[5:13], row.addTime)
	buf[13] = byte(row.syncStatus)
	binary.LittleEndian.PutUint32(buf[14:18], uint32(rdlen))
	copy(buf[18:18+rdlen], row.rawData)
	return buf
}

// deserializeChainedAddress deserializes the raw data from the passed address
// row as a chained address.
func (mtx *managerTx) deserializeChainedAddress(addressID []byte, row *dbAddressRow) (*dbChainAddressRow, error) {
	// The serialized chain address raw data format is:
	//   <branch><index>
	//
	// 4 bytes branch + 4 bytes address index
	if len(row.rawData) != 8 {
		str := fmt.Sprintf("malformed serialized chained address for "+
			"key %s", addressID)
		return nil, managerError(ErrDatabase, str, nil)
	}

	retRow := dbChainAddressRow{
		dbAddressRow: *row,
	}

	retRow.branch = binary.LittleEndian.Uint32(row.rawData[0:4])
	retRow.index = binary.LittleEndian.Uint32(row.rawData[4:8])

	return &retRow, nil
}

// serializeChainedAddress returns the serialization of the raw data field for
// a chained address.
func (mtx *managerTx) serializeChainedAddress(branch, index uint32) []byte {
	// The serialized chain address raw data format is:
	//   <branch><index>
	//
	// 4 bytes branch + 4 bytes address index
	rawData := make([]byte, 8)
	binary.LittleEndian.PutUint32(rawData[0:4], branch)
	binary.LittleEndian.PutUint32(rawData[4:8], index)
	return rawData
}

// deserializeImportedAddress deserializes the raw data from the passed address
// row as an imported address.
func (mtx *managerTx) deserializeImportedAddress(addressID []byte, row *dbAddressRow) (*dbImportedAddressRow, error) {
	// The serialized imported address raw data format is:
	//   <encpubkeylen><encpubkey><encprivkeylen><encprivkey>
	//
	// 4 bytes encrypted pubkey len + encrypted pubkey + 4 bytes encrypted
	// privkey len + encrypted privkey

	// Given the above, the length of the entry must be at a minimum
	// the constant value sizes.
	if len(row.rawData) < 8 {
		str := fmt.Sprintf("malformed serialized imported address for "+
			"key %s", addressID)
		return nil, managerError(ErrDatabase, str, nil)
	}

	retRow := dbImportedAddressRow{
		dbAddressRow: *row,
	}

	pubLen := binary.LittleEndian.Uint32(row.rawData[0:4])
	retRow.encryptedPubKey = make([]byte, pubLen)
	copy(retRow.encryptedPubKey, row.rawData[4:4+pubLen])
	offset := 4 + pubLen
	privLen := binary.LittleEndian.Uint32(row.rawData[offset : offset+4])
	offset += 4
	retRow.encryptedPrivKey = make([]byte, privLen)
	copy(retRow.encryptedPrivKey, row.rawData[offset:offset+privLen])

	return &retRow, nil
}

// serializeImportedAddress returns the serialization of the raw data field for
// an imported address.
func (mtx *managerTx) serializeImportedAddress(encryptedPubKey, encryptedPrivKey []byte) []byte {
	// The serialized imported address raw data format is:
	//   <encpubkeylen><encpubkey><encprivkeylen><encprivkey>
	//
	// 4 bytes encrypted pubkey len + encrypted pubkey + 4 bytes encrypted
	// privkey len + encrypted privkey
	pubLen := uint32(len(encryptedPubKey))
	privLen := uint32(len(encryptedPrivKey))
	rawData := make([]byte, 8+pubLen+privLen)
	binary.LittleEndian.PutUint32(rawData[0:4], pubLen)
	copy(rawData[4:4+pubLen], encryptedPubKey)
	offset := 4 + pubLen
	binary.LittleEndian.PutUint32(rawData[offset:offset+4], privLen)
	offset += 4
	copy(rawData[offset:offset+privLen], encryptedPrivKey)
	return rawData
}

// deserializeScriptAddress deserializes the raw data from the passed address
// row as a script address.
func (mtx *managerTx) deserializeScriptAddress(addressID []byte, row *dbAddressRow) (*dbScriptAddressRow, error) {
	// The serialized script address raw data format is:
	//   <encscripthashlen><encscripthash><encscriptlen><encscript>
	//
	// 4 bytes encrypted script hash len + encrypted script hash + 4 bytes
	// encrypted script len + encrypted script

	// Given the above, the length of the entry must be at a minimum
	// the constant value sizes.
	if len(row.rawData) < 8 {
		str := fmt.Sprintf("malformed serialized script address for "+
			"key %s", addressID)
		return nil, managerError(ErrDatabase, str, nil)
	}

	retRow := dbScriptAddressRow{
		dbAddressRow: *row,
	}

	hashLen := binary.LittleEndian.Uint32(row.rawData[0:4])
	retRow.encryptedHash = make([]byte, hashLen)
	copy(retRow.encryptedHash, row.rawData[4:4+hashLen])
	offset := 4 + hashLen
	scriptLen := binary.LittleEndian.Uint32(row.rawData[offset : offset+4])
	offset += 4
	retRow.encryptedScript = make([]byte, scriptLen)
	copy(retRow.encryptedScript, row.rawData[offset:offset+scriptLen])

	return &retRow, nil
}

// serializeScriptAddress returns the serialization of the raw data field for
// a script address.
func (mtx *managerTx) serializeScriptAddress(encryptedHash, encryptedScript []byte) []byte {
	// The serialized script address raw data format is:
	//   <encscripthashlen><encscripthash><encscriptlen><encscript>
	//
	// 4 bytes encrypted script hash len + encrypted script hash + 4 bytes
	// encrypted script len + encrypted script

	hashLen := uint32(len(encryptedHash))
	scriptLen := uint32(len(encryptedScript))
	rawData := make([]byte, 8+hashLen+scriptLen)
	binary.LittleEndian.PutUint32(rawData[0:4], hashLen)
	copy(rawData[4:4+hashLen], encryptedHash)
	offset := 4 + hashLen
	binary.LittleEndian.PutUint32(rawData[offset:offset+4], scriptLen)
	offset += 4
	copy(rawData[offset:offset+scriptLen], encryptedScript)
	return rawData
}

// FetchAddress loads address information for the provided address id from
// the database.  The returned value is one of the address rows for the specific
// address type.  The caller should use type assertions to ascertain the type.
func (mtx *managerTx) FetchAddress(addressID []byte) (interface{}, error) {
	bucket := (*bolt.Tx)(mtx).Bucket(addrBucketName)

	addrHash := fastsha256.Sum256(addressID)
	serializedRow := bucket.Get(addrHash[:])
	if serializedRow == nil {
		str := "address not found"
		return nil, managerError(ErrAddressNotFound, str, nil)
	}

	row, err := mtx.deserializeAddressRow(addressID, serializedRow)
	if err != nil {
		return nil, err
	}

	switch row.addrType {
	case atChain:
		return mtx.deserializeChainedAddress(addressID, row)
	case atImport:
		return mtx.deserializeImportedAddress(addressID, row)
	case atScript:
		return mtx.deserializeScriptAddress(addressID, row)
	}

	str := fmt.Sprintf("unsupported address type '%d'", row.addrType)
	return nil, managerError(ErrDatabase, str, nil)
}

// putAddress stores the provided address information to the database.  This
// is used a common base for storing the various address types.
func (mtx *managerTx) putAddress(addressID []byte, row *dbAddressRow) error {
	bucket := (*bolt.Tx)(mtx).Bucket(addrBucketName)

	// Write the serialized value keyed by the hash of the address.  The
	// additional hash is used to conceal the actual address while still
	// allowed keyed lookups.
	addrHash := fastsha256.Sum256(addressID)
	err := bucket.Put(addrHash[:], mtx.serializeAddressRow(row))
	if err != nil {
		str := fmt.Sprintf("failed to store address %x", addressID)
		return managerError(ErrDatabase, str, err)
	}
	return nil
}

// PutChainedAddress stores the provided chained address information to the
// database.
func (mtx *managerTx) PutChainedAddress(addressID []byte, account uint32,
	status syncStatus, branch, index uint32) error {

	addrRow := dbAddressRow{
		addrType:   atChain,
		account:    account,
		addTime:    uint64(time.Now().Unix()),
		syncStatus: status,
		rawData:    mtx.serializeChainedAddress(branch, index),
	}
	if err := mtx.putAddress(addressID, &addrRow); err != nil {
		return err
	}

	// Update the next index for the appropriate internal or external
	// branch.
	bucket := (*bolt.Tx)(mtx).Bucket(acctBucketName)
	bucket, err := bucket.CreateBucketIfNotExists(accountKey(account))
	if err != nil {
		str := fmt.Sprintf("failed to create account %d bucket", account)
		return managerError(ErrDatabase, str, err)
	}
	lastIndexName := acctExternalIndexName
	if branch == internalBranch {
		lastIndexName = acctInternalIndexName
	}
	var idx [4]byte
	binary.LittleEndian.PutUint32(idx[:], index+1)
	err = bucket.Put(lastIndexName, idx[:])
	if err != nil {
		str := fmt.Sprintf("failed to store address branch %d new "+
			"index %d", branch, index+1)
		return managerError(ErrDatabase, str, err)
	}
	return nil
}

// PutImportedAddress stores the provided imported address information to the
// database.
func (mtx *managerTx) PutImportedAddress(addressID []byte, account uint32,
	status syncStatus, encryptedPubKey, encryptedPrivKey []byte) error {

	rawData := mtx.serializeImportedAddress(encryptedPubKey, encryptedPrivKey)
	addrRow := dbAddressRow{
		addrType:   atImport,
		account:    account,
		addTime:    uint64(time.Now().Unix()),
		syncStatus: status,
		rawData:    rawData,
	}
	if err := mtx.putAddress(addressID, &addrRow); err != nil {
		return err
	}

	return nil
}

// PutScriptAddress stores the provided script address information to the
// database.
func (mtx *managerTx) PutScriptAddress(addressID []byte, account uint32,
	status syncStatus, encryptedHash, encryptedScript []byte) error {

	rawData := mtx.serializeScriptAddress(encryptedHash, encryptedScript)
	addrRow := dbAddressRow{
		addrType:   atScript,
		account:    account,
		addTime:    uint64(time.Now().Unix()),
		syncStatus: status,
		rawData:    rawData,
	}
	if err := mtx.putAddress(addressID, &addrRow); err != nil {
		return err
	}

	return nil
}

// ExistsAddress returns whether or not the address id exists in the database.
func (mtx *managerTx) ExistsAddress(addressID []byte) bool {
	bucket := (*bolt.Tx)(mtx).Bucket(addrBucketName)

	addrHash := fastsha256.Sum256(addressID)
	return bucket.Get(addrHash[:]) != nil
}

// DeletePrivateKeys removes all private key material from the database.
//
// NOTE: Care should be taken when calling this function.  It is primarily
// intended for use in converting to a watching-only copy.  Removing the private
// keys from the main database without also marking it watching-only will result
// in an unusable database.  It will also make any imported scripts and private
// keys unrecoverable unless there is a backup copy available.
func (mtx *managerTx) DeletePrivateKeys() error {
	bucket := (*bolt.Tx)(mtx).Bucket(mainBucketName)

	// Delete the master private key params and the crypto private and
	// script keys.
	if err := bucket.Delete(masterPrivKeyName); err != nil {
		str := "failed to delete master private key parameters"
		return managerError(ErrDatabase, str, err)
	}
	if err := bucket.Delete(cryptoPrivKeyName); err != nil {
		str := "failed to delete crypto private key"
		return managerError(ErrDatabase, str, err)
	}
	if err := bucket.Delete(cryptoScriptKeyName); err != nil {
		str := "failed to delete crypto script key"
		return managerError(ErrDatabase, str, err)
	}

	// Delete the account extended private key for all accounts.
	bucket = (*bolt.Tx)(mtx).Bucket(acctBucketName)
	cursor := bucket.Cursor()
	for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
		// Skip non-buckets.
		if v != nil {
			continue
		}

		aBucket := bucket.Bucket(k)
		if err := aBucket.Delete(acctPrivKeyName); err != nil {
			str := "failed to delete account private extended key"
			return managerError(ErrDatabase, str, err)
		}
	}

	// Delete the private key for all imported addresses.
	bucket = (*bolt.Tx)(mtx).Bucket(addrBucketName)
	cursor = bucket.Cursor()
	for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
		// Skip buckets
		if v == nil {
			continue
		}

		// Deserialize the address row first to determine the field
		// values.
		row, err := mtx.deserializeAddressRow(k, v)
		if err != nil {
			return err
		}

		switch row.addrType {
		case atImport:
			irow, err := mtx.deserializeImportedAddress(k, row)
			if err != nil {
				return err
			}

			// Reserialize the imported address without the private
			// key and store it.
			row.rawData = mtx.serializeImportedAddress(
				irow.encryptedPubKey, nil)
			err = bucket.Put(k, mtx.serializeAddressRow(row))
			if err != nil {
				str := "failed to delete imported private key"
				return managerError(ErrDatabase, str, err)
			}

		case atScript:
			srow, err := mtx.deserializeScriptAddress(k, row)
			if err != nil {
				return err
			}

			// Reserialize the script address without the script
			// and store it.
			row.rawData = mtx.serializeScriptAddress(
				srow.encryptedHash, nil)
			err = bucket.Put(k, mtx.serializeAddressRow(row))
			if err != nil {
				str := "failed to delete imported script"
				return managerError(ErrDatabase, str, err)
			}
		}
	}

	return nil
}

// managerDB provides transactional facilities to read and write the address
// manager data to a bolt database.
type managerDB struct {
	db      *bolt.DB
	version uint32
	created time.Time
}

// Close releases all database resources.  All transactions must be closed
// before closing the database.
func (db *managerDB) Close() error {
	if err := db.db.Close(); err != nil {
		str := "failed to close database"
		return managerError(ErrDatabase, str, err)
	}

	return nil
}

// View executes the passed function within the context of a managed read-only
// transaction. Any error that is returned from the passed function is returned
// from this function.
func (db *managerDB) View(fn func(tx *managerTx) error) error {
	err := db.db.View(func(tx *bolt.Tx) error {
		return fn((*managerTx)(tx))
	})
	if err != nil {
		// Ensure the returned error is a ManagerError.
		if _, ok := err.(ManagerError); !ok {
			str := "failed during database read transaction"
			return managerError(ErrDatabase, str, err)
		}
		return err
	}

	return nil
}

// Update executes the passed function within the context of a read-write
// managed transaction. The transaction is committed if no error is returned
// from the function. On the other hand, the entire transaction is rolled back
// if an error is returned.  Any error that is returned from the passed function
// or returned from the commit is returned from this function.
func (db *managerDB) Update(fn func(tx *managerTx) error) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		return fn((*managerTx)(tx))
	})
	if err != nil {
		// Ensure the returned error is a ManagerError.
		if _, ok := err.(ManagerError); !ok {
			str := "failed during database write transaction"
			return managerError(ErrDatabase, str, err)
		}
		return err
	}

	return nil
}

// CopyDB copies the entire database to the provided new database path.  A
// reader transaction is maintained during the copy so it is safe to continue
// using the database while a copy is in progress.
func (db *managerDB) CopyDB(newDbPath string) error {
	err := db.db.View(func(tx *bolt.Tx) error {
		if err := tx.CopyFile(newDbPath, 0600); err != nil {
			str := "failed to copy database"
			return managerError(ErrDatabase, str, err)
		}

		return nil
	})
	if err != nil {
		// Ensure the returned error is a ManagerError.
		if _, ok := err.(ManagerError); !ok {
			str := "failed during database copy"
			return managerError(ErrDatabase, str, err)
		}
		return err
	}

	return nil
}

// openOrCreateDB opens the database at the provided path or creates and
// initializes it if it does not already exist.  It also provides facilities to
// upgrade the database to newer versions.
func openOrCreateDB(dbPath string) (*managerDB, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		str := "failed to open database"
		return nil, managerError(ErrDatabase, str, err)
	}

	// Initialize the buckets and main db fields as needed.
	var version uint32
	var createDate uint64
	err = db.Update(func(tx *bolt.Tx) error {
		mainBucket, err := tx.CreateBucketIfNotExists(mainBucketName)
		if err != nil {
			str := "failed to create main bucket"
			return managerError(ErrDatabase, str, err)
		}

		_, err = tx.CreateBucketIfNotExists(addrBucketName)
		if err != nil {
			str := "failed to create address bucket"
			return managerError(ErrDatabase, str, err)
		}

		_, err = tx.CreateBucketIfNotExists(acctBucketName)
		if err != nil {
			str := "failed to create account bucket"
			return managerError(ErrDatabase, str, err)
		}

		_, err = tx.CreateBucketIfNotExists(addrAcctIdxBucketName)
		if err != nil {
			str := "failed to create address index bucket"
			return managerError(ErrDatabase, str, err)
		}

		// Save the most recent database version if it isn't already
		// there, otherwise keep track of it for potential upgrades.
		verBytes := mainBucket.Get(dbVersionName)
		if verBytes == nil {
			version = lastestDbVersion

			var buf [4]byte
			binary.LittleEndian.PutUint32(buf[:], version)
			err := mainBucket.Put(dbVersionName, buf[:])
			if err != nil {
				str := "failed to store latest database version"
				return managerError(ErrDatabase, str, err)
			}
		} else {
			version = binary.LittleEndian.Uint32(verBytes)
		}

		createBytes := mainBucket.Get(dbCreateDateName)
		if createBytes == nil {
			createDate = uint64(time.Now().Unix())
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], createDate)
			err := mainBucket.Put(dbCreateDateName, buf[:])
			if err != nil {
				str := "failed to store database creation time"
				return managerError(ErrDatabase, str, err)
			}
		} else {
			createDate = binary.LittleEndian.Uint64(createBytes)
		}

		return nil
	})
	if err != nil {
		str := "failed to update database"
		return nil, managerError(ErrDatabase, str, err)
	}

	// Upgrade the database as needed.
	if version < lastestDbVersion {
		// No upgrades yet.
	}

	return &managerDB{
		db:      db,
		version: version,
		created: time.Unix(int64(createDate), 0),
	}, nil
}
