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
	"fmt"
	"os"
	"sync"

	"github.com/conformal/btcec"
	"github.com/conformal/btcnet"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcutil/hdkeychain"
	"github.com/conformal/btcwallet/snacl"
	"github.com/conformal/btcwire"
)

const (
	// MaxAccountNum is the maximum allowed account number.  This value was
	// chosen because accounts are hardened children and therefore must
	// not exceed the hardened child range of extended keys and it provides
	// a reserved account at the top of the range for supporting imported
	// addresses.
	MaxAccountNum = hdkeychain.HardenedKeyStart - 2 // 2^31 - 2

	// MaxAddressesPerAccount is the maximum allowed number of addresses
	// per account number.  This value is based on the limitation of
	// the underlying hierarchical deterministic key derivation.
	MaxAddressesPerAccount = hdkeychain.HardenedKeyStart - 1

	// importedAddrAccount is the account number to use for all imported
	// addresses.  This is useful since normal accounts are derived from the
	// root hierarchical deterministic key and imported addresses do not
	// fit into that model.
	ImportedAddrAccount = MaxAccountNum + 1 // 2^31 - 1

	// defaultAccountNum is the number of the default account.
	defaultAccountNum = 0

	// scryptN, scryptR, and scryptP are the parameters used for scrypt
	// password-based key derivation.
	scryptN = snacl.DefaultN
	scryptR = snacl.DefaultR
	scryptP = snacl.DefaultP

	// The hierarchy described by BIP0043 is:
	//  m/<purpose>'/*
	// This is further extended by BIP0044 to:
	//  m/44'/<coin type>'/<account>'/<branch>/<address index>
	//
	// The branch is 0 for external addresses and 1 for internal addresses.

	// externalBranch is the child number to use when performing BIP0044
	// style hierarchical deterministic key derivation for the external
	// branch.
	externalBranch = 0

	// internalBranch is the child number to use when performing BIP0044
	// style hierarchical deterministic key derivation for the internal
	// branch.
	internalBranch = 1
)

// BlockStamp defines a block (by height and a unique hash) and is
// used to mark a point in the blockchain that an address manager element is
// synced to.
type BlockStamp struct {
	Height int32
	Hash   btcwire.ShaHash
}

// SyncStatus represents the synchronization status of an address.
type SyncStatus uint8

// The various supported sync status types.  Note that this package does not
// maintain balances or otherwise use the synchronization state, but provides
// this status mechanism to make it easier to associate synchronization state
// with addresses.
const (
	// SSUnsynced indicates an address is not synced to the block chain
	// and hence any transactions which involve the address have not yet
	// been accounted for.
	SSUnsynced SyncStatus = iota

	// SSPartial indicates an address is only partially synced to the block
	// chain and hence any transactions which involve the address in later
	// blocks have not yet been accounted for.
	SSPartial

	// SSFull indicates an address is fully synced to the block returned
	// by the SyncedTo function.
	SSFull
)

// addrKey is used to uniquely identify an address even when those addresses
// would end up being the same bitcoin address (as is the case for pay-to-pubkey
// and pay-to-pubkey-hash style of addresses).
type addrKey string

// accountInfo houses the current state of the internal and external branches
// of an account along with the extended keys needed to derive new keys.  It
// also handles locking by keeping an encrypted version of the serialized
// private extended key so the unencrypted versions can be cleared from memory
// when the account manager is locked.
type accountInfo struct {
	// The account key is used to derive the branches which in turn derive
	// the internal and external addresses.
	// The accountKeyPriv will be nil when the address manager is locked.
	acctKeyEncrypted []byte
	acctKeyPriv      *hdkeychain.ExtendedKey
	acctKeyPub       *hdkeychain.ExtendedKey

	// The external branch is used for all addresses which are intended
	// for external use.
	nextExternalIndex uint32
	lastExternalAddr  ManagedAddress

	// The internal branch is used for all adddresses which are only
	// intended for internal wallet use such as change addresses.
	nextInternalIndex uint32
	lastInternalAddr  ManagedAddress
}

// unlockDeriveInfo houses the information needed to derive a private key for a
// managed address when the account manager is unlocked.  See the deriveOnUnlock
// field in the Manager struct for more details on how this is used.
type unlockDeriveInfo struct {
	managedAddr *managedAddress
	branch      uint32
	index       uint32
}

// Manager represents a concurrency safe crypto currency address manager and
// key store.
type Manager struct {
	mtx sync.RWMutex

	db           *managerDB
	net          *btcnet.Params
	addrs        map[addrKey]ManagedAddress
	syncedTo     *BlockStamp
	watchingOnly bool
	locked       bool
	closed       bool

	// acctInfo houses information about accounts including what is needed
	// to generate deterministic chained keys for each created account.
	acctInfo map[uint32]*accountInfo

	// masterKeyPub is the secret key used to secure the cryptoKeyPub key.
	// This approach is used because it makes changing the password much
	// simpler as it then becomes just changing this one key.  It also
	// provides future flexibility.
	//
	// NOTE: This is not the same thing as BIP0032 master node extended
	// key.
	//
	// The underlying key will be zeroed when the address manager is locked.
	masterKeyPub *snacl.SecretKey

	// masterKeyPriv is the secret key used to secure the cryptoKeyPriv key.
	// This approach is used because it makes changing the password much
	// simpler as it then becomes just changing this one key.  It also
	// provides future flexibility.
	//
	// NOTE: This is not the same thing as BIP0032 master node extended
	// key.
	//
	// The underlying key will be zeroed when the address manager is locked.
	masterKeyPriv *snacl.SecretKey

	// cryptoKeyPub is the key used to encrypt public extended keys and
	// addresses.
	//
	// NOTE: This is currently intentionally a hard coded value which
	// provides no additional security benefit (the code is open source thus
	// the hard coded value is readily available), but it has been included
	// here to make it easier in the future to support user-specified (or
	// potentially derived) password protection of public key data.  Since
	// this only protects public key data, it is not locked on a timeout
	// like the secret key that protects private key.
	cryptoKeyPub *snacl.CryptoKey

	// cryptoKeyPriv is the key used to encrypt private data such as the
	// master hierarchical deterministic extended key.
	//
	// This key will be zeroed when the address manager is locked.
	cryptoKeyPrivEncrypted []byte
	cryptoKeyPriv          *snacl.CryptoKey

	// cryptoKeyScript is the key used to encrypt script data.
	//
	// This key will be zeroed when the address manager is locked.
	cryptoKeyScriptEncrypted []byte
	cryptoKeyScript          *snacl.CryptoKey

	// deriveOnUnlock is a list of private keys which needs to be derived
	// on the next unlock.  This occurs when a public address is derived
	// while the account manager is locked since it does not have access to
	// the private extended key (hence nor the underlying private key) in
	// order to encrypt it.
	deriveOnUnlock []*unlockDeriveInfo
}

// lock performs a best try effort to remove and zero all secret keys associated
// with the address manager.
//
// This function MUST be called with the manager lock held for writes.
func (m *Manager) lock() {
	// Clear all of the account private keys.
	for _, acctInfo := range m.acctInfo {
		if acctInfo.acctKeyPriv != nil {
			acctInfo.acctKeyPriv.Zero()
		}
		acctInfo.acctKeyPriv = nil
	}

	// Remove clear text private keys and scripts from all address entries.
	for _, ma := range m.addrs {
		switch addr := ma.(type) {
		case *managedAddress:
			addr.lock()
		case *scriptAddress:
			addr.lock()
		}
	}

	// Remove clear text private master and crypto keys from memory.
	m.cryptoKeyScript.Zero()
	m.cryptoKeyPriv.Zero()
	m.masterKeyPriv.Zero()

	// NOTE: m.cryptoKeyPub is intentionally not cleared here as the address
	// manager needs to be able to continue to read and decrypt public data
	// which uses a separate derived key from the database even when it is
	// locked.

	m.locked = true
}

// zeroSensitivePublicData performs a best try effort to remove and zero all
// sensitive public data associated with the address manager such as
// hierarchical deterministic extended public keys and the crypto public keys.
func (m *Manager) zeroSensitivePublicData() {
	// Clear all of the account private keys.
	for _, acctInfo := range m.acctInfo {
		acctInfo.acctKeyPub.Zero()
		acctInfo.acctKeyPub = nil
	}

	// Remove clear text public master and crypto keys from memory.
	m.cryptoKeyPub.Zero()
	m.masterKeyPub.Zero()
}

// Close cleanly shuts down the underlying database and syncs all data.  It also
// makes a best try effort to remove and zero all private key and sensitive
// public key material associated with the address manager.
func (m *Manager) Close() error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Attempt to clear private key material from memory.
	if !m.watchingOnly && !m.locked {
		m.lock()
	}

	// Attempt to clear sensitive public key material from memory too.
	m.zeroSensitivePublicData()

	if err := m.db.Close(); err != nil {
		return err
	}

	m.closed = true
	return nil
}

// keyToManaged returns a new managed address for the provided derived key and
// its derivation path which consists of the account, branch, and index.
//
// The passed derivedKey is zeroed after the new address is created.
//
// This function MUST be called with the manager lock held for writes.
func (m *Manager) keyToManaged(derivedKey *hdkeychain.ExtendedKey, account, branch, index uint32) (ManagedAddress, error) {
	// Create a new managed address based on the public or private key
	// depending on whether the passed key is private.  Also, zero the
	// key after creating the managed address from it.
	ma, err := newManagedAddressFromExtKey(m, account, derivedKey)
	defer derivedKey.Zero()
	if err != nil {
		return nil, err
	}
	if !derivedKey.IsPrivate() {
		// Add the managed address to the list of addresses that need
		// their private keys derived when the account manager is next
		// unlocked.
		info := unlockDeriveInfo{
			managedAddr: ma,
			branch:      branch,
			index:       index,
		}
		m.deriveOnUnlock = append(m.deriveOnUnlock, &info)
	}
	if branch == internalBranch {
		ma.internal = true
	}

	return ma, nil
}

// deriveKey returns either a public or private derived extended key based on
// the private flag for the given an account info, branch, and index.
func (m *Manager) deriveKey(acctInfo *accountInfo, branch, index uint32, private bool) (*hdkeychain.ExtendedKey, error) {
	// Choose the public or private extended key based on whether or not
	// the private flag was specified.  This, in turn, allows for public or
	// private child derivation.
	acctKey := acctInfo.acctKeyPub
	if private {
		acctKey = acctInfo.acctKeyPriv
	}

	// Derive and return the key.
	branchKey, err := acctKey.Child(branch)
	if err != nil {
		str := fmt.Sprintf("failed to derive extended key branch %d",
			branch)
		return nil, managerError(ErrKeyChain, str, err)
	}
	addressKey, err := branchKey.Child(index)
	branchKey.Zero() // Zero branch key after it's used.
	if err != nil {
		str := fmt.Sprintf("failed to derive child extended key -- "+
			"branch %d, child %d",
			branch, index)
		return nil, managerError(ErrKeyChain, str, err)
	}
	return addressKey, nil
}

// loadAccountInfo attempts to load and cache information about the given
// account from the database.   This includes what is necessary to derive new
// keys for it and track the state of the internal and external branches.
//
// This function MUST be called with the manager lock held for writes.
func (m *Manager) loadAccountInfo(account uint32) (*accountInfo, error) {
	// Return the account info from cache if it's available.
	if acctInfo, ok := m.acctInfo[account]; ok {
		return acctInfo, nil
	}

	// The account is either invalid or just wasn't cached, so attempt to
	// load the information from the database.
	var row *dbAccountRow
	err := m.db.View(func(tx *managerTx) error {
		var err error
		row, err = tx.FetchAccountInfo(account)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Use the crypto public key to decrypt the account public extended key.
	serializedKeyPub, err := m.cryptoKeyPub.Decrypt(row.pubKeyEncrypted)
	if err != nil {
		str := fmt.Sprintf("failed to decrypt public key for account %d",
			account)
		return nil, managerError(ErrCrypto, str, err)
	}
	acctKeyPub, err := hdkeychain.NewKeyFromString(string(serializedKeyPub))
	if err != nil {
		str := fmt.Sprintf("failed to create extended public key for "+
			"account %d", account)
		return nil, managerError(ErrKeyChain, str, err)
	}

	// Create the new account info with the known information.  The rest
	// of the fields are filled out below.
	acctInfo := &accountInfo{
		acctKeyEncrypted:  row.privKeyEncrypted,
		acctKeyPub:        acctKeyPub,
		nextExternalIndex: row.nextExternalIndex,
		nextInternalIndex: row.nextInternalIndex,
	}

	if !m.locked {
		// Use the crypto private key to decrypt the account private
		// extended keys.
		decrypted, err := m.cryptoKeyPriv.Decrypt(acctInfo.acctKeyEncrypted)
		if err != nil {
			str := fmt.Sprintf("failed to decrypt private key for "+
				"account %d", account)
			return nil, managerError(ErrCrypto, str, err)
		}

		acctKeyPriv, err := hdkeychain.NewKeyFromString(string(decrypted))
		if err != nil {
			str := fmt.Sprintf("failed to create extendd private "+
				"key for account %d", account)
			return nil, managerError(ErrKeyChain, str, err)
		}
		acctInfo.acctKeyPriv = acctKeyPriv
	}

	// Derive and cache the managed address for the last external address.
	branch, index := uint32(externalBranch), row.nextExternalIndex
	if index > 0 {
		index--
	}
	lastExtKey, err := m.deriveKey(acctInfo, branch, index, !m.locked)
	if err != nil {
		return nil, err
	}
	lastExtAddr, err := m.keyToManaged(lastExtKey, account, branch, index)
	if err != nil {
		return nil, err
	}
	acctInfo.lastExternalAddr = lastExtAddr

	// Derive and cache the managed address for the last internal address.
	branch, index = uint32(internalBranch), row.nextExternalIndex
	if index > 0 {
		index--
	}
	lastIntKey, err := m.deriveKey(acctInfo, branch, index, !m.locked)
	if err != nil {
		return nil, err
	}
	lastIntAddr, err := m.keyToManaged(lastIntKey, account, branch, index)
	if err != nil {
		return nil, err
	}
	acctInfo.lastInternalAddr = lastIntAddr

	// Add it to the cache and return it when everything is successful.
	m.acctInfo[account] = acctInfo
	return acctInfo, nil
}

// deriveKeyFromPath returns either a public or private derived extended key
// based on the private flag for the given an account, branch, and index.
//
// This function MUST be called with the manager lock held for writes.
func (m *Manager) deriveKeyFromPath(account, branch, index uint32, private bool) (*hdkeychain.ExtendedKey, error) {
	// Look up the account key information.
	acctInfo, err := m.loadAccountInfo(account)
	if err != nil {
		return nil, err
	}

	return m.deriveKey(acctInfo, branch, index, private)
}

// chainAddressRowToManaged returns a new managed address based on chained
// address data loaded from the database.
//
// This function MUST be called with the manager lock held for writes.
func (m *Manager) chainAddressRowToManaged(row *dbChainAddressRow) (ManagedAddress, error) {
	addressKey, err := m.deriveKeyFromPath(row.account, row.branch,
		row.index, !m.locked)
	if err != nil {
		return nil, err
	}

	return m.keyToManaged(addressKey, row.account, row.branch, row.index)
}

// importedAddressRowToManaged returns a new managed address based on imported
// address data loaded from the database.
func (m *Manager) importedAddressRowToManaged(row *dbImportedAddressRow) (ManagedAddress, error) {
	// Use the crypto public key to decrypt the imported public key.
	pubBytes, err := m.cryptoKeyPub.Decrypt(row.encryptedPubKey)
	if err != nil {
		str := "failed to decrypt public key for imported address"
		return nil, managerError(ErrCrypto, str, err)
	}

	pubKey, err := btcec.ParsePubKey(pubBytes, btcec.S256())
	if err != nil {
		str := "invalid public key for imported address"
		return nil, managerError(ErrCrypto, str, err)
	}

	ma, err := newManagedAddressWithoutPrivKey(m, row.account, pubKey, true)
	if err != nil {
		return nil, err
	}
	ma.privKeyEncrypted = row.encryptedPrivKey
	ma.imported = true

	return ma, nil
}

// scriptAddressRowToManaged returns a new managed address based on script
// address data loaded from the database.
func (m *Manager) scriptAddressRowToManaged(row *dbScriptAddressRow) (ManagedAddress, error) {
	// Use the crypto public key to decrypt the imported script hash.
	scriptHash, err := m.cryptoKeyPub.Decrypt(row.encryptedHash)
	if err != nil {
		str := "failed to decrypt imported script hash"
		return nil, managerError(ErrCrypto, str, err)
	}

	sa, err := newScriptAddress(m, row.account, scriptHash,
		row.encryptedScript)
	if err != nil {
		return nil, err
	}

	return sa, nil
}

// loadAndCacheAddress attempts to load the passed address from the database and
// caches the associated managed address.
//
// This function MUST be called with the manager lock held for writes.
func (m *Manager) loadAndCacheAddress(address btcutil.Address) (ManagedAddress, error) {
	// Attempt to load the raw address information from the database.
	var rowInterface interface{}
	err := m.db.View(func(tx *managerTx) error {
		var err error
		rowInterface, err = tx.FetchAddress(address.ScriptAddress())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Create a new managed address for the specific type of address.
	// based on type and clean this nonsense up.
	var managedAddr ManagedAddress
	switch row := rowInterface.(type) {
	case *dbChainAddressRow:
		managedAddr, err = m.chainAddressRowToManaged(row)
		if err != nil {
			return nil, err
		}

	case *dbImportedAddressRow:
		managedAddr, err = m.importedAddressRowToManaged(row)
		if err != nil {
			return nil, err
		}

	case *dbScriptAddressRow:
		managedAddr, err = m.scriptAddressRowToManaged(row)
		if err != nil {
			return nil, err
		}

	default:
		str := fmt.Sprintf("unsupported address type %T", row)
		return nil, managerError(ErrDatabase, str, nil)
	}

	// Cache and return the new managed address.
	m.addrs[addrKey(managedAddr.Address().ScriptAddress())] = managedAddr
	return managedAddr, nil
}

// Address returns a managed address given the passed address if it is known
// to the address manager.  A managed address differs from the passed address
// in that it also potentially contains extra information needed to sign
// transactions such as the associated private key for pay-to-pubkey and
// pay-to-pubkey-hash addresses and the script associated with
// pay-to-script-hash addresses.
func (m *Manager) Address(address btcutil.Address) (ManagedAddress, error) {
	// Return the address from cache if it's available.
	//
	// NOTE: Not using a defer on the lock here since a write lock is
	// needed if the lookup fails.
	m.mtx.RLock()
	if ma, ok := m.addrs[addrKey(address.ScriptAddress())]; ok {
		m.mtx.RUnlock()
		return ma, nil
	}
	m.mtx.RUnlock()

	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Attempt to load the address from the database.
	return m.loadAndCacheAddress(address)
}

// ChangePassphrase changes either the public or private passphrase to the
// provided value depending on the private flag.  In order to change the private
// password, the account manager must not be watching-only and also must be
// unlocked.
func (m *Manager) ChangePassphrase(newPassphrase []byte, private bool) error {
	// No private passphrase to change for a watching-only address manager.
	if private && m.watchingOnly {
		return managerError(ErrWatchingOnly, errWatchingOnly, nil)
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Can't change the private passphrase when locked.
	if private && m.locked {
		return managerError(ErrLocked, errLocked, nil)
	}

	// Generate a new master key from the passphrase which is used to secure
	// the actual secret keys.
	newMasterKey, err := snacl.NewSecretKey(&newPassphrase, scryptN,
		scryptR, scryptP)
	if err != nil {
		str := "failed to create new master private key"
		return managerError(ErrCrypto, str, err)
	}
	newKeyParams := newMasterKey.Marshal()

	if private {
		// Re-encrypt the crypto private key using the new master
		// private key.
		encPriv, err := newMasterKey.Encrypt(m.cryptoKeyPriv[:])
		if err != nil {
			str := "failed to encrypt crypto private key"
			return managerError(ErrCrypto, str, err)
		}

		// Re-encrypt the crypto script key using the new master private
		// key.
		encScript, err := newMasterKey.Encrypt(m.cryptoKeyScript[:])
		if err != nil {
			str := "failed to encrypt crypto script key"
			return managerError(ErrCrypto, str, err)
		}

		// Save the new keys and params to the the db in a single
		// transaction.
		err = m.db.Update(func(tx *managerTx) error {
			err := tx.PutCryptoKeys(nil, encPriv, encScript)
			if err != nil {
				return err
			}

			err = tx.PutMasterKeyParams(nil, newKeyParams)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}

		// Now that the db has been successfully updated, update memory.
		copy(m.cryptoKeyPrivEncrypted[:], encPriv)
		copy(m.cryptoKeyScriptEncrypted[:], encScript)
		m.masterKeyPriv = newMasterKey
	} else {
		// Re-encrypt the crypto public key using the new master public
		// key.
		encryptedPub, err := newMasterKey.Encrypt(m.cryptoKeyPub[:])
		if err != nil {
			str := "failed to encrypt crypto public key"
			return managerError(ErrCrypto, str, err)
		}

		// Save the new keys and params to the the db in a single
		// transaction.
		err = m.db.Update(func(tx *managerTx) error {
			err := tx.PutCryptoKeys(encryptedPub, nil, nil)
			if err != nil {
				return err
			}

			err = tx.PutMasterKeyParams(newKeyParams, nil)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}

		// Now that the db has been successfully updated, update memory.
		m.masterKeyPub = newMasterKey
	}

	return nil
}

// ExportWatchingOnly creates a new watching-only account manager backed by a
// database at the provided path.  A watching-only account manager has all
// private keys removed which means it is not possible to create transactions
// which spend funds.
func (m *Manager) ExportWatchingOnly(newDbPath string, pubPassphrase []byte) (*Manager, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	// Return an error if the specified database already exists.
	if fileExists(newDbPath) {
		return nil, managerError(ErrAlreadyExists, errAlreadyExists, nil)
	}

	// Copy the existing manager database to the provided path.
	if err := m.db.CopyDB(newDbPath); err != nil {
		return nil, err
	}

	// Open the copied database.
	watchingDb, err := openOrCreateDB(newDbPath)
	if err != nil {
		return nil, err
	}

	// Remove all private key material and mark the new database as watching
	// only.
	err = watchingDb.Update(func(tx *managerTx) error {
		if err := tx.DeletePrivateKeys(); err != nil {
			return err
		}

		if err := tx.PutWatchingOnly(true); err != nil {
			return err
		}

		return nil

	})
	if err != nil {
		return nil, err
	}

	return loadManager(watchingDb, pubPassphrase, m.net)
}

// existsAddress returns whether or not the passed address is known to the
// address manager.
//
// This function MUST be called with the manager lock held for reads.
func (m *Manager) existsAddress(addressID []byte) (bool, error) {
	// Check the in-memory map first since it's faster than a db access.
	if _, ok := m.addrs[addrKey(addressID)]; ok {
		return true, nil
	}

	// Check the database if not already found above.
	var exists bool
	err := m.db.View(func(tx *managerTx) error {
		exists = tx.ExistsAddress(addressID)
		return nil
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}

// ImportPrivateKey imports a WIF private key into the account manager.  The
// imported address is created using either a compressed or uncompressed
// serialized public key, depending on the CompressPubKey bool of the WIF.
//
// All imported addresses will be part of the account defined by the
// ImportedAddrAccount constant.
//
// This function will return a an error if the address manager is watching-only,
// locked, or not for the same network as the key trying to be imported.  It
// will also return an error if the address already exists.  Any other errors
// returned are generally unexpected.
func (m *Manager) ImportPrivateKey(wif *btcutil.WIF, bs *BlockStamp) (ManagedPubKeyAddress, error) {
	// A watching-only address manager must not contain private keys.
	if m.watchingOnly {
		return nil, managerError(ErrWatchingOnly, errWatchingOnly, nil)
	}

	// Ensure the address is intended for network the address manager is
	// associated with.
	if !wif.IsForNet(m.net) {
		str := fmt.Sprintf("private key is not for the same network the "+
			"account manager is configured for (%s)", m.net.Name)
		return nil, managerError(ErrWrongNet, str, nil)
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	// The manager must be unlocked to encrypt the imported private key.
	if m.locked {
		return nil, managerError(ErrLocked, errLocked, nil)
	}

	// Prevent duplicates.
	serializedPubKey := wif.SerializePubKey()
	scriptAddress := btcutil.Hash160(serializedPubKey)
	alreadyExists, err := m.existsAddress(scriptAddress)
	if err != nil {
		return nil, err
	}
	if alreadyExists {
		str := fmt.Sprintf("address for public key %x already exists",
			serializedPubKey)
		return nil, managerError(ErrDuplicate, str, nil)
	}

	// Encrypt private and public keys.
	encryptedPrivKey, err := m.cryptoKeyPriv.Encrypt(wif.PrivKey.Serialize())
	if err != nil {
		str := fmt.Sprintf("failed to encrypt private key for %x",
			serializedPubKey)
		return nil, managerError(ErrCrypto, str, nil)
	}
	encryptedPubKey, err := m.cryptoKeyPub.Encrypt(serializedPubKey)
	if err != nil {
		str := fmt.Sprintf("failed to encrypt public key for %x",
			serializedPubKey)
		return nil, managerError(ErrCrypto, str, nil)
	}

	// Save the new imported address to the db in a single transaction.
	err = m.db.Update(func(tx *managerTx) error {
		return tx.PutImportedAddress(scriptAddress, ImportedAddrAccount,
			ssNone, encryptedPubKey, encryptedPrivKey)
	})
	if err != nil {
		return nil, err
	}

	// Create a new managed address based on the imported address.
	managedAddr, err := newManagedAddress(m, ImportedAddrAccount,
		wif.PrivKey, wif.CompressPubKey)
	if err != nil {
		return nil, err
	}
	managedAddr.imported = true

	// Add the new managed address to the cache of recent addresses and
	// return it.
	m.addrs[addrKey(managedAddr.Address().ScriptAddress())] = managedAddr
	return managedAddr, nil
}

// ImportScript imports a a user-provided script into the account manager.  The
// imported script will act as a pay-to-script-hash address.
//
// This function will return an error if the address manager is watching-only,
// locked, or the address already exists.  It will also return any underlying
// script parse errors and any other errors returned are generally unexpected.
func (m *Manager) ImportScript(script []byte, bs *BlockStamp) (ManagedScriptAddress, error) {
	// A watching-only address manager must not contain private data.
	if m.watchingOnly {
		return nil, managerError(ErrWatchingOnly, errWatchingOnly, nil)
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	// The manager must be unlocked to encrypt the imported script.
	if m.locked {
		return nil, managerError(ErrLocked, errLocked, nil)
	}

	// Prevent duplicates.
	scriptHash := btcutil.Hash160(script)
	alreadyExists, err := m.existsAddress(scriptHash)
	if err != nil {
		return nil, err
	}
	if alreadyExists {
		str := fmt.Sprintf("address for script hash %x already exists",
			scriptHash)
		return nil, managerError(ErrDuplicate, str, nil)
	}

	// Encrypt the script hash using the crypto public key so it is
	// accessible when the address manager is locked.
	encryptedHash, err := m.cryptoKeyPub.Encrypt(scriptHash)
	if err != nil {
		str := fmt.Sprintf("failed to encrypt script hash %x",
			scriptHash)
		return nil, managerError(ErrCrypto, str, err)
	}

	// Encrypt the script for storage in database using the crypto script
	// key.
	encryptedScript, err := m.cryptoKeyScript.Encrypt(script)
	if err != nil {
		str := fmt.Sprintf("failed to encrypt script for %x",
			scriptHash)
		return nil, managerError(ErrCrypto, str, err)
	}

	// Save the new imported address to the db in a single transaction.
	err = m.db.Update(func(tx *managerTx) error {
		return tx.PutScriptAddress(scriptHash, ImportedAddrAccount,
			ssNone, encryptedHash, encryptedScript)
	})
	if err != nil {
		return nil, err
	}

	// Create a new managed address based on the imported script.  Also,
	// make a copy of the script since it will be cleared on lock and the
	// script the caller passed should not be cleared out from under the
	// caller.
	scriptAddr, err := newScriptAddress(m, ImportedAddrAccount, scriptHash,
		encryptedScript)
	if err != nil {
		return nil, err
	}
	scriptAddr.scriptCT = make([]byte, len(script))
	copy(scriptAddr.scriptCT, script)

	// Add the new managed address to the cache of recent addresses and
	// return it.
	m.addrs[addrKey(scriptHash)] = scriptAddr
	return scriptAddr, nil
}

// IsLocked returns whether or not the account managed is locked.  When it is
// unlocked, the decryption key needed to decrypt private keys used for signing
// is in memory.
func (m *Manager) IsLocked() bool {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	return m.locked
}

// Lock performs a best try effort to remove and zero all secret keys associated
// with the address manager.
//
// This function will return an error if invoked on a watching-only address
// manager.
func (m *Manager) Lock() error {
	// A watching-only address manager can't be locked.
	if m.watchingOnly {
		return managerError(ErrWatchingOnly, errWatchingOnly, nil)
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Error on attempt to lock an already locked manager.
	if m.locked {
		return managerError(ErrLocked, errLocked, nil)
	}

	m.lock()
	return nil
}

// Unlock derives the master private key from the specified passphrase.  An
// invalid passphrase will return an error.  Otherwise, the derived secret key
// is stored in memory until the account manager is locked.  Any failures that
// occur during this function will result in the account manager being locked,
// even if it was already unlocked prior to calling this function.
//
// This function will return an error if invoked on a watching-only address
// manager.
func (m *Manager) Unlock(passphrase []byte) error {
	// A watching-only address manager can't be unlocked.
	if m.watchingOnly {
		return managerError(ErrWatchingOnly, errWatchingOnly, nil)
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Derive the master private key using the provided passphrase.
	if err := m.masterKeyPriv.DeriveKey(&passphrase); err != nil {
		m.lock()
		if err == snacl.ErrInvalidPassword {
			str := "invalid passphrase for master private key"
			return managerError(ErrWrongPassphrase, str, nil)
		}

		str := "failed to derive master private key"
		return managerError(ErrCrypto, str, nil)
	}

	// Use the master private key to decrypt the crypto private key.
	decryptedKey, err := m.masterKeyPriv.Decrypt(m.cryptoKeyPrivEncrypted)
	if err != nil {
		m.lock()
		str := "failed to decrypt crypto private key"
		return managerError(ErrCrypto, str, nil)
	}
	copy(m.cryptoKeyPriv[:], decryptedKey)
	zero(decryptedKey)

	// Use the crypto private key to decrypt all of the account private
	// extended keys.
	for account, acctInfo := range m.acctInfo {
		decrypted, err := m.cryptoKeyPriv.Decrypt(acctInfo.acctKeyEncrypted)
		if err != nil {
			m.lock()
			str := fmt.Sprintf("failed to decrypt account %d "+
				"private key", account)
			return managerError(ErrCrypto, str, nil)
		}

		acctKeyPriv, err := hdkeychain.NewKeyFromString(string(decrypted))
		zero(decrypted)
		if err != nil {
			m.lock()
			str := fmt.Sprintf("failed to regenerate account %d "+
				"extended key", account)
			return managerError(ErrKeyChain, str, nil)
		}
		acctInfo.acctKeyPriv = acctKeyPriv
	}

	// Derive any private keys that are pending due to them being created
	// while the account manager was locked.
	for _, info := range m.deriveOnUnlock {
		addressKey, err := m.deriveKeyFromPath(info.managedAddr.account,
			info.branch, info.index, true)
		if err != nil {
			return err
		}

		// It's ok to ignore the error here since it can only fail if
		// the extended key is not private, however it was just derived
		// as a private key.
		privKey, _ := addressKey.ECPrivKey()
		addressKey.Zero()

		privKeyBytes := privKey.Serialize()
		privKeyEncrypted, err := m.cryptoKeyPriv.Encrypt(privKeyBytes)
		zeroBigInt(privKey.D)
		if err != nil {
			str := fmt.Sprintf("failed to encrypt private key for "+
				"address %s", info.managedAddr.Address())
			return managerError(ErrCrypto, str, nil)
		}
		info.managedAddr.privKeyEncrypted = privKeyEncrypted
		info.managedAddr.privKeyCT = privKeyBytes
	}

	m.locked = false
	return nil
}

// Net returns the network parameters for this account manager.
func (m *Manager) Net() *btcnet.Params {
	// NOTE: No need for mutex here since the net field does not change
	// after the manager instance is created.

	return m.net
}

// nextAddresses returns the specified number of next chained address from the
// branch indicated by the internal flag.
//
// This function MUST be called with the manager lock held for writes.
func (m *Manager) nextAddresses(account uint32, numAddresses uint32, internal bool) ([]ManagedAddress, error) {
	// The next address can only be generated for accounts that have already
	// been created.
	acctInfo, err := m.loadAccountInfo(account)
	if err != nil {
		return nil, err
	}

	// Choose the account key to used based on whether the account manager
	// is locked.
	acctKey := acctInfo.acctKeyPub
	if !m.locked {
		acctKey = acctInfo.acctKeyPriv
	}

	// Choose the branch key and index depending on whether or not this
	// is an internal address.
	branchNum := uint32(externalBranch)
	nextIndex := acctInfo.nextExternalIndex
	if internal {
		branchNum = internalBranch
		nextIndex = acctInfo.nextInternalIndex
	}

	// Ensure the requested number of addresses doesn't exceed the maximum
	// allowed for this account.
	if numAddresses > MaxAddressesPerAccount || nextIndex+numAddresses >
		MaxAddressesPerAccount {
		str := fmt.Sprintf("%d new addresses would exceed the maximum "+
			"allowed number of addresses per account of %d",
			numAddresses, MaxAddressesPerAccount)
		return nil, managerError(ErrTooManyAddresses, str, nil)
	}

	// Derive the appropriate branch key and ensure it is zeroed when done.
	branchKey, err := acctKey.Child(branchNum)
	if err != nil {
		str := fmt.Sprintf("failed to derive extended key branch %d",
			branchNum)
		return nil, managerError(ErrKeyChain, str, err)
	}
	defer branchKey.Zero() // Ensure branch key is zeroed when done.

	// Create the requested number of addresses and keep track of the index
	// with each one.
	addressInfo := make([]*unlockDeriveInfo, 0, numAddresses)
	for i := uint32(0); i < numAddresses; i++ {
		// There is an extremely small chance that a particular child is
		// invalid, so use a loop to derive the next valid child.
		var nextKey *hdkeychain.ExtendedKey
		for {
			// Derive the next child in the external chain branch.
			key, err := branchKey.Child(nextIndex)
			if err != nil {
				// When this particular child is invalid, skip to the
				// next index.
				if err == hdkeychain.ErrInvalidChild {
					nextIndex++
					continue
				}

				str := fmt.Sprintf("failed to generate child %d",
					nextIndex)
				return nil, managerError(ErrKeyChain, str, err)
			}
			key.SetNet(m.net)

			nextIndex++
			nextKey = key
			break
		}

		// Create a new managed address based on the public or private
		// key depending on whether the generated key is private.  Also,
		// zero the next key after creating the managed address from it.
		managedAddr, err := newManagedAddressFromExtKey(m, account, nextKey)
		nextKey.Zero()
		if err != nil {
			return nil, err
		}
		info := unlockDeriveInfo{
			managedAddr: managedAddr,
			branch:      branchNum,
			index:       nextIndex - 1,
		}
		addressInfo = append(addressInfo, &info)
	}

	// Now that all addresses have been successfully generated, update the
	// database in a single transaction.
	err = m.db.Update(func(tx *managerTx) error {
		for _, info := range addressInfo {
			ma := info.managedAddr
			addressID := ma.Address().ScriptAddress()
			err := tx.PutChainedAddress(addressID, account,
				ssFull, info.branch, info.index)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Finally update the next address tracking and add the addresses to the
	// cache after the newly generated address have been successfully added
	// to the db.
	managedAddresses := make([]ManagedAddress, 0, len(addressInfo))
	for _, info := range addressInfo {
		ma := info.managedAddr
		m.addrs[addrKey(ma.Address().ScriptAddress())] = ma

		// Add the new managed address to the list of addresses that need their
		// private keys derived when the account manager is next unlocked.
		if m.locked && !m.watchingOnly {
			m.deriveOnUnlock = append(m.deriveOnUnlock, info)
		}

		managedAddresses = append(managedAddresses, ma)
	}

	// Set the last address and next address for tracking.
	ma := addressInfo[len(addressInfo)-1].managedAddr
	if internal {
		acctInfo.nextInternalIndex = nextIndex
		acctInfo.lastInternalAddr = ma
	} else {
		acctInfo.nextExternalIndex = nextIndex
		acctInfo.lastExternalAddr = ma
	}

	return managedAddresses, nil
}

// NextExternalAddresses returns the specified number of next chained addresses
// that are intended for external use from the address manager.
func (m *Manager) NextExternalAddresses(account uint32, numAddresses uint32) ([]ManagedAddress, error) {
	// Enforce maximum account number.
	if account > MaxAccountNum {
		err := managerError(ErrAccountNumTooHigh, errAcctTooHigh, nil)
		return nil, err
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.nextAddresses(account, numAddresses, false)
}

// NextInternalAddresses returns the specified number of next chained addresses
// that are intended for internal use such as change from the address manager.
func (m *Manager) NextInternalAddresses(account uint32, numAddresses uint32) ([]ManagedAddress, error) {
	// Enforce maximum account number.
	if account > MaxAccountNum {
		err := managerError(ErrAccountNumTooHigh, errAcctTooHigh, nil)
		return nil, err
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.nextAddresses(account, numAddresses, true)
}

// LastExternalAddress returns the most recently requested chained external
// address from calling NextExternalAddress for the given account.  The first
// external address for the account will be returned if none have been
// previously requested.
//
// This function will return an error if the provided account number is greater
// than the MaxAccountNum constant or there is no account information for the
// passed account.  Any other errors returned are generally unexpected.
func (m *Manager) LastExternalAddress(account uint32) (ManagedAddress, error) {
	// Enforce maximum account number.
	if account > MaxAccountNum {
		err := managerError(ErrAccountNumTooHigh, errAcctTooHigh, nil)
		return nil, err
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Load account information for the passed account.  It is typically
	// cached, but if not it will be loaded from the database.
	acctInfo, err := m.loadAccountInfo(account)
	if err != nil {
		return nil, err
	}

	return acctInfo.lastExternalAddr, nil
}

// LastInternalAddress returns the most recently requested chained internal
// address from calling NextInternalAddress for the given account.  The first
// internal address for the account will be returned if none have been
// previously requested.
//
// This function will return an error if the provided account number is greater
// than the MaxAccountNum constant or there is no account information for the
// passed account.  Any other errors returned are generally unexpected.
func (m *Manager) LastInternalAddress(account uint32) (ManagedAddress, error) {
	// Enforce maximum account number.
	if account > MaxAccountNum {
		err := managerError(ErrAccountNumTooHigh, errAcctTooHigh, nil)
		return nil, err
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Load account information for the passed account.  It is typically
	// cached, but if not it will be loaded from the database.
	acctInfo, err := m.loadAccountInfo(account)
	if err != nil {
		return nil, err
	}

	return acctInfo.lastInternalAddr, nil
}

// SetSyncStatus sets the sync status for a single address.  This may error if
// the address is not found in the address manager.
//
// When marking an address as unsynced, only the type Unsynced matters.
// The value is ignored.
func (m *Manager) SetSyncStatus(a btcutil.Address, ss SyncStatus) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	ma, ok := m.addrs[addrKey(a.ScriptAddress())]
	if !ok {
		str := fmt.Sprintf("address %s not found", a.EncodeAddress())
		return managerError(ErrAddressNotFound, str, nil)
	}
	_ = ma

	// TODO(davec: Implement
	//ma.setSyncStatus(ss)
	return ErrNotImplemented
}

// SetSyncedTo marks already synced addresses in the address manager to be in
// sync with the recently-seen block described by the blockstamp.  Unsynced
// addresses are unaffected by this method.
func (m *Manager) SetSyncedTo(bs *BlockStamp) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// TODO(davec): Fix it up..
	return ErrNotImplemented
}

// SyncedTo returns details about the block height and hash that the address
// manager is synced through at the very least.  The intention is that callers
// can use this information for intelligently initiating rescans to sync back to
// the best chain from the last known good block.
//
// Each ManagedAddress that is marked with SyncStatus of SSFull will be assumed
// synced through this block.
func (m *Manager) SyncedTo() BlockStamp {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Return a copy so updates do not affect the caller.
	return *m.syncedTo
}

// newManager returns a new locked account manager with the given parameters.
func newManager(db *managerDB, net *btcnet.Params, masterKeyPub *snacl.SecretKey,
	masterKeyPriv *snacl.SecretKey, cryptoKeyPub *snacl.CryptoKey,
	cryptoKeyPrivEncrypted, cryptoKeyScriptEncrypted []byte) *Manager {

	return &Manager{
		db:                       db,
		net:                      net,
		addrs:                    make(map[addrKey]ManagedAddress),
		syncedTo:                 &BlockStamp{},
		locked:                   true,
		acctInfo:                 make(map[uint32]*accountInfo),
		masterKeyPub:             masterKeyPub,
		masterKeyPriv:            masterKeyPriv,
		cryptoKeyPub:             cryptoKeyPub,
		cryptoKeyPrivEncrypted:   cryptoKeyPrivEncrypted,
		cryptoKeyPriv:            &snacl.CryptoKey{},
		cryptoKeyScriptEncrypted: cryptoKeyScriptEncrypted,
		cryptoKeyScript:          &snacl.CryptoKey{},
	}
}

// filesExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// deriveAccountKey derives the extended key for an account according to the
// hierarchy described by BIP0044 given the master node.
//
// In particular this is the hierarchical deterministic extended key path:
//   m/44'/<coin type>'/<account>'
func deriveAccountKey(masterNode *hdkeychain.ExtendedKey, coinType uint32,
	account uint32) (*hdkeychain.ExtendedKey, error) {

	// Enforce maximum account number.
	if account > MaxAccountNum {
		err := managerError(ErrAccountNumTooHigh, errAcctTooHigh, nil)
		return nil, err
	}

	// The hierarchy described by BIP0043 is:
	//  m/<purpose>'/*
	// This is further extended by BIP0044 to:
	//  m/44'/<coin type>'/<account>'/<branch>/<address index>
	//
	// The branch is 0 for external addresses and 1 for internal addresses.

	// Derive the purpose key as a child of the master node.
	purpose, err := masterNode.Child(44 + hdkeychain.HardenedKeyStart)
	if err != nil {
		return nil, err
	}

	// Derive the coin type key as a child of the purpose key.
	coinTypeKey, err := purpose.Child(coinType + hdkeychain.HardenedKeyStart)
	if err != nil {
		return nil, err
	}

	// Derive the account key as a child of the coin type key.
	acctKey, err := coinTypeKey.Child(account + hdkeychain.HardenedKeyStart)
	if err != nil {
		return nil, err
	}

	return acctKey, nil
}

// checkBranchKeys ensures deriving the extended keys for the internal and
// external branches given an account key does not result in an invalid child
// error which means the chosen seed is not usable.  This conforms to the
// hierarchy described by BIP0044 so long as the account key is already derived
// accordingly.
//
// In particular this is the hierarchical deterministic extended key path:
//   m/44'/<coin type>'/<account>'/<branch>
//
// The branch is 0 for external addresses and 1 for internal addresses.
func checkBranchKeys(acctKey *hdkeychain.ExtendedKey) error {
	// Derive the external branch as the first child of the account key.
	if _, err := acctKey.Child(externalBranch); err != nil {
		return err
	}

	// Derive the external branch as the second child of the account key.
	if _, err := acctKey.Child(internalBranch); err != nil {
		return err
	}

	return nil
}

// loadManager returns a new account manager that results from loading it from
// the passed opened database.  The public passphrase is required to decrypt the
// public keys.
func loadManager(db *managerDB, pubPassphrase []byte, net *btcnet.Params) (*Manager, error) {
	// Perform all database lookups in a read-only view.
	var watchingOnly bool
	var masterKeyPubParams, masterKeyPrivParams []byte
	var cryptoKeyPubEnc, cryptoKeyPrivEnc, cryptoKeyScriptEnc []byte
	err := db.View(func(tx *managerTx) error {
		// Load whether or not the manager is watching-only from the db.
		var err error
		watchingOnly, err = tx.FetchWatchingOnly()
		if err != nil {
			return err
		}

		// Load the master key params from the db.
		masterKeyPubParams, masterKeyPrivParams, err =
			tx.FetchMasterKeyParams()
		if err != nil {
			return err
		}

		// Load the crypto keys from the db.
		cryptoKeyPubEnc, cryptoKeyPrivEnc, cryptoKeyScriptEnc, err =
			tx.FetchCryptoKeys()
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// When not a watching-only manager, set the master private key params,
	// but don't derive it now since the manager starts off locked.
	var masterKeyPriv snacl.SecretKey
	if !watchingOnly {
		err := masterKeyPriv.Unmarshal(masterKeyPrivParams)
		if err != nil {
			str := "failed to unmarshal master private key"
			return nil, managerError(ErrCrypto, str, err)
		}
	}

	// Derive the master public key using the serialized params and provided
	// passphrase.
	var masterKeyPub snacl.SecretKey
	if err := masterKeyPub.Unmarshal(masterKeyPubParams); err != nil {
		str := "failed to unmarshal master public key"
		return nil, managerError(ErrCrypto, str, err)
	}
	if err := masterKeyPub.DeriveKey(&pubPassphrase); err != nil {
		str := "invalid passphrase for master public key"
		return nil, managerError(ErrWrongPassphrase, str, nil)
	}

	// Use the master public key to decrypt the crypto public key.
	var cryptoKeyPub snacl.CryptoKey
	cryptoKeyPubCT, err := masterKeyPub.Decrypt(cryptoKeyPubEnc)
	if err != nil {
		str := "failed to decrypt crypto public key"
		return nil, managerError(ErrCrypto, str, err)
	}
	copy(cryptoKeyPub[:], cryptoKeyPubCT)
	zero(cryptoKeyPubCT)

	// TODO(davec): Load up syncedTo from db.
	syncedTo := &BlockStamp{}

	// Create new account manager with the given parameters.  Also, override
	// the defaults for the additional fields which are not specified in the
	// call to new with the values loaded from the database.
	mgr := newManager(db, net, &masterKeyPub, &masterKeyPriv, &cryptoKeyPub,
		cryptoKeyPrivEnc, cryptoKeyScriptEnc)
	mgr.watchingOnly = watchingOnly
	mgr.syncedTo = syncedTo
	return mgr, nil
}

// Open loads an existing account manager from the given database path.  The
// public passphrase is required to decrypt the public keys used to protect the
// public information such as addresses.  This is important since access to
// BIP0032 extended keys means it is possible to generate all future addresses.
//
// a ManagerError with an error code of ErrNoExist will be returned if the
// passed database does not exist.
func Open(dbPath string, pubPassphrase []byte, net *btcnet.Params) (*Manager, error) {
	// Return an error if the specified database does not exist.
	if !fileExists(dbPath) {
		str := "the specified account manager does not exist"
		return nil, managerError(ErrNoExist, str, nil)
	}

	db, err := openOrCreateDB(dbPath)
	if err != nil {
		return nil, err
	}

	return loadManager(db, pubPassphrase, net)
}

// Create returns a new locked account manager at the given database path.  The
// seed must conform to the standards described in hdkeychain.NewMaster and will
// be used to create the master root node from which all hierarchical
// deterministic addresses are derived.  This allows all chained addresses in
// the account manager to be recovered by using the same seed.
//
// All private and public keys and information are protected by secret keys
// derived from the provided private and public passphrases.  The public
// passphrase is required on subsequent opens of the account manager, and the
// private passphrase is required to unlock the account manager in order to gain
// access to any private keys and information.
//
// A ManagerError with an error code of ErrAlreadyExists will be returned if the
// passed database already exists.
func Create(dbPath string, seed, pubPassphrase, privPassphrase []byte, net *btcnet.Params) (*Manager, error) {
	// Return an error if the specified database already exists.
	if fileExists(dbPath) {
		return nil, managerError(ErrAlreadyExists, errAlreadyExists, nil)
	}

	db, err := openOrCreateDB(dbPath)
	if err != nil {
		return nil, err
	}

	// Generate the BIP0044 HD key structure to ensure the provided seed
	// can generate the required structure with no issues.

	// Derive the master extended key from the seed.
	root, err := hdkeychain.NewMaster(seed)
	if err != nil {
		str := "failed to derive master extended key"
		return nil, managerError(ErrKeyChain, str, err)
	}

	// Derive the account key for the first account according to BIP0044.
	acctKeyPriv, err := deriveAccountKey(root, net.HDCoinType, 0)
	if err != nil {
		// The seed is unusable if the any of the children in the
		// required hierarchy can't be derived due to invalid child.
		if err == hdkeychain.ErrInvalidChild {
			str := "the provided seed is unusable"
			return nil, managerError(ErrKeyChain, str,
				hdkeychain.ErrUnusableSeed)
		}

		return nil, err
	}

	// Ensure the branch keys can be derived for the provided seed according
	// to BIP0044.
	if err := checkBranchKeys(acctKeyPriv); err != nil {
		// The seed is unusable if the any of the children in the
		// required hierarchy can't be derived due to invalid child.
		if err == hdkeychain.ErrInvalidChild {
			str := "the provided seed is unusable"
			return nil, managerError(ErrKeyChain, str,
				hdkeychain.ErrUnusableSeed)
		}

		return nil, err
	}

	// The address manager needs the public extended key for the account.
	acctKeyPub, err := acctKeyPriv.Neuter()
	if err != nil {
		str := "failed to convert private key for account 0"
		return nil, managerError(ErrKeyChain, str, err)
	}

	// Generate new master keys.  These master keys are used to protect the
	// crypto keys that will be generated next.
	masterKeyPub, err := snacl.NewSecretKey(&pubPassphrase, scryptN,
		scryptR, scryptP)
	if err != nil {
		str := "failed to master public key"
		return nil, managerError(ErrCrypto, str, err)
	}
	masterKeyPriv, err := snacl.NewSecretKey(&privPassphrase, scryptN,
		scryptR, scryptP)
	if err != nil {
		str := "failed to master private key"
		return nil, managerError(ErrCrypto, str, err)
	}

	// Generate new crypto public, private, and script keys.  These keys are
	// used to protect the actual public and private data such as addresses,
	// extended keys, and scripts.
	cryptoKeyPub, err := snacl.GenerateCryptoKey()
	if err != nil {
		str := "failed to generate crypto public key"
		return nil, managerError(ErrCrypto, str, err)
	}
	cryptoKeyPriv, err := snacl.GenerateCryptoKey()
	if err != nil {
		str := "failed to generate crypto private key"
		return nil, managerError(ErrCrypto, str, err)
	}
	cryptoKeyScript, err := snacl.GenerateCryptoKey()
	if err != nil {
		str := "failed to generate crypto script key"
		return nil, managerError(ErrCrypto, str, err)
	}

	// Encrypt the crypto keys with the associated master keys.
	cryptoKeyPubEnc, err := masterKeyPub.Encrypt(cryptoKeyPub[:])
	if err != nil {
		str := "failed to encrypt crypto public key"
		return nil, managerError(ErrCrypto, str, err)
	}
	cryptoKeyPrivEnc, err := masterKeyPriv.Encrypt(cryptoKeyPriv[:])
	if err != nil {
		str := "failed to encrypt crypto private key"
		return nil, managerError(ErrCrypto, str, err)
	}
	cryptoKeyScriptEnc, err := masterKeyPriv.Encrypt(cryptoKeyScript[:])
	if err != nil {
		str := "failed to encrypt crypto script key"
		return nil, managerError(ErrCrypto, str, err)
	}

	// Encrypt the default account keys with the associated crypto keys.
	acctPubEnc, err := cryptoKeyPub.Encrypt([]byte(acctKeyPub.String()))
	if err != nil {
		str := "failed to  encrypt crypto public key"
		return nil, managerError(ErrCrypto, str, err)
	}
	acctPrivEnc, err := cryptoKeyPriv.Encrypt([]byte(acctKeyPriv.String()))
	if err != nil {
		str := "failed to encrypt public key for account 0"
		return nil, managerError(ErrCrypto, str, err)
	}

	// Perform all database updates in a single transaction.
	err = db.Update(func(tx *managerTx) error {
		// Save the master key params to the database.
		pubParams := masterKeyPub.Marshal()
		privParams := masterKeyPriv.Marshal()
		err = tx.PutMasterKeyParams(pubParams, privParams)
		if err != nil {
			return err
		}

		// Save the encrypted crypto keys to the database.
		err = tx.PutCryptoKeys(cryptoKeyPubEnc, cryptoKeyPrivEnc,
			cryptoKeyScriptEnc)
		if err != nil {
			return err
		}

		// Save the fact this is not a watching-only account manager to
		// the database.
		err = tx.PutWatchingOnly(false)
		if err != nil {
			return err
		}

		// Save the information for the default account to the database.
		acctInfo := dbAccountRow{
			pubKeyEncrypted:   acctPubEnc,
			privKeyEncrypted:  acctPrivEnc,
			nextExternalIndex: 0,
			nextInternalIndex: 0,
			name:              "",
		}
		err = tx.PutAccountInfo(defaultAccountNum, &acctInfo)
		if err != nil {
			return err
		}

		err = tx.PutNumAccounts(1)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// The new account manager is locked by default, so clear the master,
	// crypto private, and crypto script keys from memory.
	masterKeyPriv.Zero()
	cryptoKeyPriv.Zero()
	cryptoKeyScript.Zero()
	return newManager(db, net, masterKeyPub, masterKeyPriv, cryptoKeyPub,
		cryptoKeyPrivEnc, cryptoKeyScriptEnc), nil
}
