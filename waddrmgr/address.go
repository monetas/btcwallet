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
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"

	"github.com/conformal/btcec"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcutil/hdkeychain"
	"github.com/conformal/btcwallet/snacl"
)

// zero sets all bytes in the passed slice to zero.  This is used to
// explicitly clear private key material from memory.
func zero(b []byte) {
	for i := range b {
		b[i] ^= b[i]
	}
}

// zeroBigInt sets all bytes in the passed big int to zero and then sets the
// value to 0.  This differs from simply setting the value in that it
// specifically clears the underlying bytes whereas simply setting the value
// does not.  This is mostly useful to forcefully clear private keys.
func zeroBigInt(x *big.Int) {
	// NOTE: This could make use of .Xor, however this is safer since the
	// specific implementation of Xor could technically change in such a way
	// as the original bits aren't cleared.  This function would silenty
	// fail in that case and it's best to avoid that possibility.
	bits := x.Bits()
	numBits := len(bits)
	for i := 0; i < numBits; i++ {
		bits[i] ^= bits[i]
	}
	x.SetInt64(0)
}

// ManagedAddress is an interface that provides acces to information regarding
// an address managed by an address manager. Concrete implementations of this
// type may provide further fields to provide information specific to that type
// of address.
type ManagedAddress interface {
	// Account returns the account the address is associated with.
	Account() uint32

	// Address returns a btcutil.Address for the backing address.
	Address() btcutil.Address

	// AddrHash returns the key or script hash related to the address
	AddrHash() []byte

	// Imported returns true if the backing address was imported instead
	// of being part of an address chain.
	Imported() bool

	// Internal returns true if the backing address was created for internal
	// use such as a change output of a transaction.
	Internal() bool

	// Compressed returns true if the backing address is compressed.
	Compressed() bool

	// SyncStatus returns the current synced state of an address.
	SyncStatus() SyncStatus
}

// ManagedPubKeyAddress extends ManagedAddress and additionally provides the
// public and private keys for pubkey-based addresses.
type ManagedPubKeyAddress interface {
	ManagedAddress

	// PubKey returns the public key associated with the address.
	PubKey() *btcec.PublicKey

	// ExportPubKey returns the public key associated with the address
	// serialized as a hex encoded string.
	ExportPubKey() string

	// PrivKey returns the private key for the address.  It can fail if the
	// address manager is watching-only or locked, or the address does not
	// have any keys.
	PrivKey() (*btcec.PrivateKey, error)

	// ExportPrivKey returns the private key associated with the address
	// serialized as Wallet Import Format (WIF).
	ExportPrivKey() (*btcutil.WIF, error)
}

// ManagedScriptAddress extends ManagedAddress and represents a pay-to-script-hash
// style of bitcoin addresses.  It additionally provides information about the
// script.
type ManagedScriptAddress interface {
	ManagedAddress

	// Script returns the script associated with the address.
	Script() ([]byte, error)
}

// managedAddress represents a public key address.  It also may or may not have
// the private key associated with the public key.
type managedAddress struct {
	manager           *Manager
	account           uint32
	address           *btcutil.AddressPubKeyHash
	imported          bool
	internal          bool
	compressed        bool
	pubKey            *btcec.PublicKey
	syncStatus        SyncStatus
	partialSyncHeight int32 // only used when syncStatus is not SSFull
	privKeyEncrypted  []byte
	privKeyCT         []byte // non-nil if unlocked
	privKeyMutex      sync.Mutex
}

// Enforce mangedAddress satisfies the ManagedPubKeyAddress interface.
var _ ManagedPubKeyAddress = &managedAddress{}

// unlock decrypts and stores a pointer to the associated private key.  It will
// fail if the key is invalid or the encrypted private key is not available.
// The returned clear text private key will always be a copy that may be safely
// used by the caller without worrying about it being zeroed during an address
// lock.
func (a *managedAddress) unlock(key *snacl.CryptoKey) ([]byte, error) {
	// Protect concurrent access to clear text private key.
	a.privKeyMutex.Lock()
	defer a.privKeyMutex.Unlock()

	if len(a.privKeyCT) == 0 {
		privKey, err := key.Decrypt(a.privKeyEncrypted)
		if err != nil {
			str := fmt.Sprintf("failed to decrypt private key for "+
				"%s", a.address)
			return nil, managerError(ErrCrypto, str, nil)
		}

		a.privKeyCT = privKey
	}

	privKeyCopy := make([]byte, len(a.privKeyCT))
	copy(privKeyCopy, a.privKeyCT)
	return privKeyCopy, nil
}

// lock zeroes the associated clear text private key.
func (a *managedAddress) lock() {
	// Protect concurrent access to clear text private key.
	a.privKeyMutex.Lock()
	defer a.privKeyMutex.Unlock()

	// Zero and nil the clear text private key associated with this
	// address.
	zero(a.privKeyCT)
	a.privKeyCT = nil
}

// Account returns the account number the address is associated with.
//
// This is part of the ManagedAddress interface implementation.
func (a *managedAddress) Account() uint32 {
	return a.account
}

// Address returns the btcutil.Address which represents the managed address.
// This will be a pay-to-pubkey-hash address.
//
// This is part of the ManagedAddress interface implementation.
func (a *managedAddress) Address() btcutil.Address {
	return a.address
}

// AddrHash returns the public key hash for the address.
//
// This is part of the ManagedAddress interface implementation.
func (a *managedAddress) AddrHash() []byte {
	return a.address.Hash160()[:]
}

// Imported returns true if the address was imported instead of being part of an
// address chain.
//
// This is part of the ManagedAddress interface implementation.
func (a *managedAddress) Imported() bool {
	return a.imported
}

// Internal returns true if the address was created for internal use such as a
// change output of a transaction.
//
// This is part of the ManagedAddress interface implementation.
func (a *managedAddress) Internal() bool {
	return a.internal
}

// Compressed returns true if the address is compressed.
//
// This is part of the ManagedAddress interface implementation.
func (a *managedAddress) Compressed() bool {
	return a.compressed
}

// TODO(davec): Comment
//
// This is part of the ManagedAddress interface implementation.
func (a *managedAddress) SyncStatus() SyncStatus {
	return a.syncStatus
}

// PubKey returns the public key associated with the address.
//
// This is part of the ManagedPubKeyAddress interface implementation.
func (a *managedAddress) PubKey() *btcec.PublicKey {
	return a.pubKey
}

// pubKeyBytes returns the serialized public key bytes for the managed address
// based on whether or not the managed address is marked as compressed.
func (a *managedAddress) pubKeyBytes() []byte {
	if a.compressed {
		return a.pubKey.SerializeCompressed()
	}
	return a.pubKey.SerializeUncompressed()
}

// ExportPubKey returns the public key associated with the address
// serialized as a hex encoded string.
//
// This is part of the ManagedPubKeyAddress interface implementation.
func (a *managedAddress) ExportPubKey() string {
	return hex.EncodeToString(a.pubKeyBytes())
}

// PrivKey returns the private key for the address.  It can fail if the address
// manager is watching-only or locked, or the address does not have any keys.
//
// This is part of the ManagedPubKeyAddress interface implementation.
func (a *managedAddress) PrivKey() (*btcec.PrivateKey, error) {
	// No private keys are available for a watching-only address manager.
	if a.manager.watchingOnly {
		return nil, managerError(ErrWatchingOnly, errWatchingOnly, nil)
	}

	a.manager.mtx.Lock()
	defer a.manager.mtx.Unlock()

	// Account manager must be unlocked to decrypt the private key.
	if a.manager.locked {
		return nil, managerError(ErrLocked, errLocked, nil)
	}

	// Decrypt the key as needed.  Also, make sure it's a copy since the
	// private key stored in memory can be cleared at any time.  Otherwise
	// the returned private key could be invalidated from under the caller.
	privKeyCopy, err := a.unlock(a.manager.cryptoKeyPriv)
	if err != nil {
		return nil, err
	}

	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privKeyCopy)
	return privKey, nil
}

// ExportPrivKey returns the private key associated with the address serialized
// as Wallet Import Format (WIF).
//
// This is part of the ManagedPubKeyAddress interface implementation.
func (a *managedAddress) ExportPrivKey() (*btcutil.WIF, error) {
	pk, err := a.PrivKey()
	if err != nil {
		return nil, err
	}

	return btcutil.NewWIF(pk, a.manager.net, a.compressed)
}

// newManagedAddressWithoutPrivKey returns a new managed address based on the
// passed account, public key, and whether or not the public key should be
// compressed.
func newManagedAddressWithoutPrivKey(m *Manager, account uint32, pubKey *btcec.PublicKey, compressed bool) (*managedAddress, error) {
	// Create a pay-to-pubkey-hash address from the public key.
	var pubKeyHash []byte
	if compressed {
		pubKeyHash = btcutil.Hash160(pubKey.SerializeCompressed())
	} else {
		pubKeyHash = btcutil.Hash160(pubKey.SerializeUncompressed())
	}
	address, err := btcutil.NewAddressPubKeyHash(pubKeyHash, m.net)
	if err != nil {
		return nil, err
	}

	return &managedAddress{
		manager:           m,
		address:           address,
		account:           account,
		imported:          false,
		internal:          false,
		compressed:        compressed,
		pubKey:            pubKey,
		partialSyncHeight: 0,
		privKeyEncrypted:  nil,
		privKeyCT:         nil,
	}, nil
}

// newManagedAddress returns a new managed address based on the passed account,
// private key, and whether or not the public key is compressed.  The managed
// address will have access to the private and public keys.
func newManagedAddress(m *Manager, account uint32, privKey *btcec.PrivateKey, compressed bool) (*managedAddress, error) {
	// Encrypt the private key.
	//
	// NOTE: The privKeyBytes here are set into the managed address which
	// are cleared when locked, so they aren't cleared here.
	privKeyBytes := privKey.Serialize()
	privKeyEncrypted, err := m.cryptoKeyPriv.Encrypt(privKeyBytes)
	if err != nil {
		str := "failed to encrypt private key"
		return nil, managerError(ErrCrypto, str, err)
	}

	// Leverage the code to create a managed address without a private key
	// and then add the private key to it.
	ecPubKey := (*btcec.PublicKey)(&privKey.PublicKey)
	managedAddr, err := newManagedAddressWithoutPrivKey(m, account,
		ecPubKey, compressed)
	if err != nil {
		return nil, err
	}
	managedAddr.privKeyEncrypted = privKeyEncrypted
	managedAddr.privKeyCT = privKeyBytes

	return managedAddr, nil
}

// newManagedAddressFromExtKey returns a new managed address based on the passed
// account and extended key.  The managed address will have access to the
// private and public keys if the provided extended key is private, otherwise it
// will only have access to the public key.
func newManagedAddressFromExtKey(m *Manager, account uint32, key *hdkeychain.ExtendedKey) (*managedAddress, error) {
	// Create a new managed address based on the public or private key
	// depending on whether the generated key is private.
	var managedAddr *managedAddress
	if key.IsPrivate() {
		privKey, err := key.ECPrivKey()
		if err != nil {
			return nil, err
		}

		// Ensure the temp private key big integer is cleared after use.
		managedAddr, err = newManagedAddress(m, account, privKey, true)
		zeroBigInt(privKey.D)
		if err != nil {
			return nil, err
		}
	} else {
		pubKey, err := key.ECPubKey()
		if err != nil {
			return nil, err
		}

		managedAddr, err = newManagedAddressWithoutPrivKey(m, account,
			pubKey, true)
		if err != nil {
			return nil, err
		}
	}

	return managedAddr, nil
}

// scriptAddress represents a pay-to-script-hash address.
type scriptAddress struct {
	manager         *Manager
	account         uint32
	address         *btcutil.AddressScriptHash
	scriptEncrypted []byte
	scriptCT        []byte
	scriptMutex     sync.Mutex
}

// Enforce scriptAddress satisfies the ScriptAddress interface.
var _ ManagedScriptAddress = &scriptAddress{}

// unlock decrypts and stores the associated script.  It will fail if the key is
// invalid or the encrypted script is not available.  The returned clear text
// script will always be a copy that may be safely used by the caller without
// worrying about it being zeroed during an address lock.
func (a *scriptAddress) unlock(key *snacl.CryptoKey) ([]byte, error) {
	// Protect concurrent access to clear text script.
	a.scriptMutex.Lock()
	defer a.scriptMutex.Unlock()

	if len(a.scriptCT) == 0 {
		script, err := key.Decrypt(a.scriptEncrypted)
		if err != nil {
			str := fmt.Sprintf("failed to decrypt script for %s",
				a.address)
			return nil, managerError(ErrCrypto, str, nil)
		}

		a.scriptCT = script
	}

	scriptCopy := make([]byte, len(a.scriptCT))
	copy(scriptCopy, a.scriptCT)
	return scriptCopy, nil
}

// lock zeroes the associated clear text private key.  This function fails if
// the address is not encrypted.
func (a *scriptAddress) lock() {
	// Protect concurrent access to clear text script.
	a.scriptMutex.Lock()
	defer a.scriptMutex.Unlock()

	// Zero and nil the clear text script associated with this  address.
	zero(a.scriptCT)
	a.scriptCT = nil
}

// Account returns the account the address is associated with.  This will always
// be the ImportedAddrAccount constant for script addresses.
//
// This is part of the ManagedAddress interface implementation.
func (a *scriptAddress) Account() uint32 {
	return a.account
}

// Address returns the btcutil.Address which represents the managed address.
// This will be a pay-to-script-hash address.
//
// This is part of the ManagedAddress interface implementation.
func (a *scriptAddress) Address() btcutil.Address {
	return a.address
}

// AddrHash returns the script hash for the address.
//
// This is part of the ManagedAddress interface implementation.
//
// This is part of the ManagedAddress interface implementation.
func (a *scriptAddress) AddrHash() []byte {
	return a.address.Hash160()[:]
}

// Imported always returns true since script addresses are always imported
// addresses and not part of any chain.
//
// This is part of the ManagedAddress interface implementation.
func (a *scriptAddress) Imported() bool {
	return true
}

// Internal always returns false since script addresses are always imported
// addresses and not part of any chain in order to be for internal use.
//
// This is part of the ManagedAddress interface implementation.
func (a *scriptAddress) Internal() bool {
	return false

}

// Compressed returns false since script addresses are never compressed.
//
// This is part of the ManagedAddress interface implementation.
func (a *scriptAddress) Compressed() bool {
	return false
}

// TODO(davec): Comment
//
// This is part of the ManagedAddress interface implementation.
func (a *scriptAddress) SyncStatus() SyncStatus {
	// TODO(davec): Finish
	return SSFull
}

// Script returns the script associated with the address.
//
// This implements the ScriptAddress interface.
func (a *scriptAddress) Script() ([]byte, error) {
	// No script is available for a watching-only address manager.
	if a.manager.watchingOnly {
		return nil, managerError(ErrWatchingOnly, errWatchingOnly, nil)
	}

	a.manager.mtx.Lock()
	defer a.manager.mtx.Unlock()

	// Account manager must be unlocked to decrypt the script.
	if a.manager.locked {
		return nil, managerError(ErrLocked, errLocked, nil)
	}

	// Decrypt the script as needed.  Also, make sure it's a copy since the
	// script stored in memory can be cleared at any time.  Otherwise,
	// the returned script could be invalidated from under the caller.
	scriptCopy, err := a.unlock(a.manager.cryptoKeyScript)
	if err != nil {
		return nil, err
	}

	return scriptCopy, nil
}

// newScriptAddress initializes and returns a new pay-to-script-hash address.
func newScriptAddress(m *Manager, account uint32, scriptHash, scriptEncrypted []byte) (*scriptAddress, error) {
	address, err := btcutil.NewAddressScriptHashFromHash(scriptHash, m.net)
	if err != nil {
		return nil, err
	}

	return &scriptAddress{
		manager:         m,
		account:         account,
		address:         address,
		scriptEncrypted: scriptEncrypted,
	}, nil
}
