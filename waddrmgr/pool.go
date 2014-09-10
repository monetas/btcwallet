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
	"sort"

	"github.com/monetas/btcscript"
	"github.com/monetas/btcutil"
	"github.com/monetas/btcutil/hdkeychain"
	"github.com/monetas/btcwallet/snacl"
)

type seriesData struct {
	publicKeys  []*hdkeychain.ExtendedKey
	privateKeys []*hdkeychain.ExtendedKey
	reqSigs     uint32
	// A.k.a. "m" in "m of n signatures needed".
}

type VotingPool struct {
	ID           []byte
	seriesLookup map[uint32]*seriesData
	manager      *Manager
}

func (m *Manager) CreateVotingPool(poolID []byte) (*VotingPool, error) {
	err := m.db.Update(func(tx *managerTx) error {
		return tx.PutVotingPool(poolID)
	})
	if err != nil {
		str := fmt.Sprintf("unable to add voting pool %v to db", poolID)
		return nil, managerError(ErrDatabase, str, err)
	}
	return &VotingPool{
		ID:           poolID,
		seriesLookup: make(map[uint32]*seriesData),
		manager:      m,
	}, nil
}

func (m *Manager) LoadVotingPool(poolID []byte) (*VotingPool, error) {
	err := m.db.View(func(tx *managerTx) error {
		if exists := tx.ExistsVotingPool(poolID); !exists {
			str := fmt.Sprintf("unable to find voting pool %v in db", poolID)
			return managerError(ErrVotingPoolNotExists, str, nil)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	vp := &VotingPool{
		ID:           poolID,
		manager:      m,
		seriesLookup: make(map[uint32]*seriesData),
	}
	if err = vp.LoadAllSeries(); err != nil {
		return nil, err
	}
	return vp, nil
}

func (m *Manager) LoadVotingPoolAndDepositScript(
	poolID string, seriesID, branch, index uint32) ([]byte, error) {
	pid := []byte(poolID)
	vp, err := m.LoadVotingPool(pid)
	if err != nil {
		return nil, err
	}
	script, err := vp.DepositScript(seriesID, branch, index)
	if err != nil {
		return nil, err
	}
	return script, nil
}

func (m *Manager) LoadVotingPoolAndCreateSeries(
	poolID string, seriesID uint32, rawPubKeys []string, reqSigs uint32) error {
	pid := []byte(poolID)
	vp, err := m.LoadVotingPool(pid)
	if err != nil {
		managerErr := err.(ManagerError)
		if managerErr.ErrorCode == ErrVotingPoolNotExists {
			vp, err = m.CreateVotingPool(pid)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return vp.CreateSeries(seriesID, rawPubKeys, reqSigs)
}

func (m *Manager) LoadVotingPoolAndReplaceSeries(
	poolID string, seriesID uint32, rawPubKeys []string, reqSigs uint32) error {
	pid := []byte(poolID)
	vp, err := m.LoadVotingPool(pid)
	if err != nil {
		return err
	}
	return vp.ReplaceSeries(seriesID, rawPubKeys, reqSigs)
}

func (m *Manager) LoadVotingPoolAndEmpowerSeries(
	poolID string, seriesID uint32, rawPrivKey string) error {
	pid := []byte(poolID)
	pool, err := m.LoadVotingPool(pid)
	if err != nil {
		return err
	}
	return pool.EmpowerSeries(seriesID, rawPrivKey)
}

func (vp *VotingPool) GetSeries(seriesID uint32) *seriesData {
	series, exists := vp.seriesLookup[seriesID]
	if !exists {
		return nil
	}
	return series
}

func (vp *VotingPool) saveSeriesToDisk(seriesID uint32, data *seriesData) error {
	var err error
	encryptedPubKeys := make([][]byte, len(data.publicKeys))
	for i, pubKey := range data.publicKeys {
		encryptedPubKeys[i], err = vp.manager.cryptoKeyPub.Encrypt(
			[]byte(pubKey.String()))
		if err != nil {
			str := fmt.Sprintf("key %v failed encryption", pubKey)
			return managerError(ErrCrypto, str, err)
		}
	}
	encryptedPrivKeys := make([][]byte, len(data.privateKeys))
	for i, privKey := range data.privateKeys {
		if privKey == nil {
			encryptedPrivKeys[i] = nil
		} else {
			encryptedPrivKeys[i], err = vp.manager.cryptoKeyPriv.Encrypt(
				[]byte(privKey.String()))
		}
		if err != nil {
			str := fmt.Sprintf("key %v failed encryption", privKey)
			return managerError(ErrCrypto, str, err)
		}
	}

	err = vp.manager.db.Update(func(tx *managerTx) error {
		return tx.PutSeries(vp.ID, seriesID, data.reqSigs,
			encryptedPubKeys, encryptedPrivKeys)
	})
	if err != nil {
		str := fmt.Sprintf("cannot put series #%d into db", seriesID)
		return managerError(ErrSeriesStorage, str, err)
	}
	return nil
}

// CanonicalKeyOrder will return a copy of the input canonically
// ordered which is defined to be lexicographical.
func CanonicalKeyOrder(keys []string) []string {
	orderedKeys := make([]string, len(keys))
	copy(orderedKeys, keys)
	sort.Sort(sort.StringSlice(orderedKeys))
	return orderedKeys
}

// Convert the given slice of strings into a slice of ExtendedKeys,
// checking that all of them are valid public (and not private) keys,
// and that there are no duplicates.
func convertAndValidatePubKeys(rawPubKeys []string) ([]*hdkeychain.ExtendedKey, error) {
	seenKeys := make(map[string]bool)
	keys := make([]*hdkeychain.ExtendedKey, len(rawPubKeys))
	for i, rawPubKey := range rawPubKeys {
		if _, seen := seenKeys[rawPubKey]; seen {
			str := fmt.Sprintf("duplicated public key: %v", rawPubKey)
			return nil, managerError(ErrKeyDuplicate, str, nil)
		} else {
			seenKeys[rawPubKey] = true
		}

		key, err := hdkeychain.NewKeyFromString(rawPubKey)
		if err != nil {
			str := fmt.Sprintf("invalid extended public key %v", rawPubKey)
			return nil, managerError(ErrKeyChain, str, err)
		}

		if key.IsPrivate() {
			str := fmt.Sprintf("private keys not accepted: %v", rawPubKey)
			return nil, managerError(ErrKeyIsPrivate, str, nil)
		}
		keys[i] = key
	}
	return keys, nil
}

func (vp *VotingPool) putSeries(seriesID uint32, inRawPubKeys []string, reqSigs uint32) error {
	if len(inRawPubKeys) < 3 {
		str := fmt.Sprintf("need at least three public keys to create a series")
		return managerError(ErrTooFewPublicKeys, str, nil)
	}

	if reqSigs > uint32(len(inRawPubKeys)) {
		str := fmt.Sprintf(
			"the number of required signatures cannot be more than the number of keys")
		return managerError(ErrTooManyReqSignatures, str, nil)
	}

	rawPubKeys := CanonicalKeyOrder(inRawPubKeys)

	keys, err := convertAndValidatePubKeys(rawPubKeys)
	if err != nil {
		return err
	}

	data := &seriesData{
		publicKeys:  keys,
		privateKeys: make([]*hdkeychain.ExtendedKey, len(keys)),
		reqSigs:     reqSigs,
	}

	err = vp.saveSeriesToDisk(seriesID, data)
	if err != nil {
		return err
	}
	vp.seriesLookup[seriesID] = data
	return nil
}

// CreateSeries will create a new non-existing series.
//
// - rawPubKeys has to contain three or more public keys;
// - reqSigs has to be less or equal than the number of public keys in rawPubKeys.
func (vp *VotingPool) CreateSeries(seriesID uint32, rawPubKeys []string, reqSigs uint32) error {
	if series := vp.GetSeries(seriesID); series != nil {
		str := fmt.Sprintf("series #%d already exists", seriesID)
		return managerError(ErrSeriesAlreadyExists, str, nil)
	}

	return vp.putSeries(seriesID, rawPubKeys, reqSigs)
}

// ReplaceSeries will replace an already existing series.
//
// - rawPubKeys has to contain three or more public keys
// - reqSigs has to be less or equal than the number of public keys in rawPubKeys.
func (vp *VotingPool) ReplaceSeries(seriesID uint32, rawPubKeys []string, reqSigs uint32) error {
	series := vp.GetSeries(seriesID)
	if series == nil {
		str := fmt.Sprintf("series #%d does not exist, cannot replace it", seriesID)
		return managerError(ErrSeriesNotExists, str, nil)
	}

	if series.IsEmpowered() {
		str := fmt.Sprintf("series #%d has private keys and cannot be replaced", seriesID)
		return managerError(ErrSeriesAlreadyEmpowered, str, nil)
	}

	return vp.putSeries(seriesID, rawPubKeys, reqSigs)
}

func decryptExtendedKey(cryptoKey *snacl.CryptoKey, encrypted []byte) (*hdkeychain.ExtendedKey, error) {
	decrypted, err := cryptoKey.Decrypt(encrypted)
	if err != nil {
		str := fmt.Sprintf("cannot decrypt key %v", encrypted)
		return nil, managerError(ErrCrypto, str, err)
	}
	result, err := hdkeychain.NewKeyFromString(string(decrypted))
	zero(decrypted)
	if err != nil {
		str := fmt.Sprintf("cannot get key from string %v", decrypted)
		return nil, managerError(ErrKeyChain, str, err)
	}
	return result, nil
}

func (vp *VotingPool) LoadAllSeries() error {
	var allSeries map[uint32]*dbSeriesRow
	err := vp.manager.db.View(func(tx *managerTx) error {
		var err error
		allSeries, err = tx.LoadAllSeries(vp.ID)
		return err
	})
	if err != nil {
		return err
	}
	for id, series := range allSeries {
		pubKeys := make([]*hdkeychain.ExtendedKey, len(series.pubKeysEncrypted))
		privKeys := make([]*hdkeychain.ExtendedKey, len(series.privKeysEncrypted))
		if len(pubKeys) != len(privKeys) {
			return managerError(ErrKeysPrivatePublicMismatch,
				"the pub key and priv key arrays should have the same number of elements",
				nil)
		}

		for i, encryptedPub := range series.pubKeysEncrypted {
			pubKey, err := decryptExtendedKey(vp.manager.cryptoKeyPub, encryptedPub)
			if err != nil {
				return err
			}
			pubKeys[i] = pubKey

			encryptedPriv := series.privKeysEncrypted[i]
			var privKey *hdkeychain.ExtendedKey
			if encryptedPriv == nil {
				privKey = nil
			} else {
				privKey, err = decryptExtendedKey(vp.manager.cryptoKeyPriv, encryptedPriv)
				if err != nil {
					return err
				}
			}
			privKeys[i] = privKey

			if privKey != nil {
				checkPubKey, err := privKey.Neuter()
				if err != nil {
					str := fmt.Sprintf("cannot neuter key %v", privKey)
					return managerError(ErrKeyNeuter, str, err)
				}
				if pubKey.String() != checkPubKey.String() {
					str := fmt.Sprintf("public key %v different than expected %v",
						pubKey, checkPubKey)
					return managerError(ErrKeyMismatch, str, nil)
				}
			}
		}
		vp.seriesLookup[id] = &seriesData{
			publicKeys:  pubKeys,
			privateKeys: privKeys,
			reqSigs:     series.reqSigs,
		}
	}
	return nil
}

// Change the order of the pubkeys based on branch number.
// Given the three pubkeys ABC, this would mean:
// - branch 0: CBA (reversed)
// - branch 1: ABC (first key priority)
// - branch 2: BAC (second key priority)
// - branch 3: CAB (third key priority)
func branchOrder(pks []*btcutil.AddressPubKey, branch uint32) []*btcutil.AddressPubKey {
	if pks == nil {
		return nil
	}

	// Change the order of pubkeys based on branch number.
	if branch == 0 {
		numKeys := len(pks)
		res := make([]*btcutil.AddressPubKey, numKeys)
		copy(res, pks)
		// reverse pk
		for i, j := 0, numKeys-1; i < j; i, j = i+1, j-1 {
			res[i], res[j] = res[j], res[i]
		}
		return res
	} else {
		tmp := make([]*btcutil.AddressPubKey, len(pks))
		tmp[0] = pks[branch-1]
		j := 1
		for i := 0; i < len(pks); i++ {
			if i != int(branch-1) {
				tmp[j] = pks[i]
				j++
			}
		}
		return tmp
	}
}

func (vp *VotingPool) DepositScriptAddress(seriesID, branch, index uint32) (ManagedScriptAddress, error) {
	script, err := vp.DepositScript(seriesID, branch, index)
	if err != nil {
		return nil, err
	}

	encryptedScript, err := vp.manager.cryptoKeyScript.Encrypt(script)
	if err != nil {
		str := fmt.Sprintf("error while encrypting multisig script hash")
		return nil, managerError(ErrCrypto, str, err)
	}

	scriptHash := btcutil.Hash160(script)

	return newScriptAddress(vp.manager, ImportedAddrAccount, scriptHash, encryptedScript)
}

func (vp *VotingPool) DepositScript(seriesID, branch, index uint32) ([]byte, error) {
	series := vp.GetSeries(seriesID)
	if series == nil {
		str := fmt.Sprintf("series #%d does not exist", seriesID)
		return nil, managerError(ErrSeriesNotExists, str, nil)
	}

	pks := make([]*btcutil.AddressPubKey, len(series.publicKeys))

	for i, key := range series.publicKeys {
		child, err := key.Child(index)
		// TODO: implement getting the next index until we find a valid one,
		// in case there is a hdkeychain.ErrInvalidChild.
		if err != nil {
			str := fmt.Sprintf("child #%d for this pubkey %d does not exist", index, i)
			return nil, managerError(ErrKeyChain, str, err)
		}
		pubkey, err := child.ECPubKey()
		if err != nil {
			str := fmt.Sprintf("child #%d for this pubkey %d does not exist", index, i)
			return nil, managerError(ErrKeyChain, str, err)
		}
		pks[i], err = btcutil.NewAddressPubKey(pubkey.SerializeCompressed(), vp.manager.net)
		if err != nil {
			str := fmt.Sprintf(
				"child #%d for this pubkey %d could not be converted to an address",
				index, i)
			return nil, managerError(ErrKeyChain, str, err)
		}
	}

	pks = branchOrder(pks, branch)

	script, err := btcscript.MultiSigScript(pks, int(series.reqSigs))
	if err != nil {
		str := fmt.Sprintf("error while making multisig script hash, %d", len(pks))
		return nil, managerError(ErrScriptCreation, str, err)
	}

	return script, nil
}

func (vp *VotingPool) EmpowerSeries(seriesID uint32, rawPrivKey string) error {
	// make sure this series exists
	series := vp.GetSeries(seriesID)
	if series == nil {
		str := fmt.Sprintf("series %d does not exist for this voting pool",
			seriesID)
		return managerError(ErrSeriesNotExists, str, nil)
	}

	// Check that the private key is valid.
	privKey, err := hdkeychain.NewKeyFromString(rawPrivKey)
	if err != nil {
		str := fmt.Sprintf("invalid extended private key %v", rawPrivKey)
		return managerError(ErrKeyChain, str, err)
	}
	if !privKey.IsPrivate() {
		str := fmt.Sprintf(
			"to empower a series you need the extended private key, not an extended public key %v",
			privKey)
		return managerError(ErrKeyIsPublic, str, err)
	}

	pubKey, err := privKey.Neuter()
	if err != nil {
		str := fmt.Sprintf("invalid extended private key %v, can't convert to public key",
			rawPrivKey)
		return managerError(ErrKeyNeuter, str, err)
	}

	lookingFor := pubKey.String()
	found := false

	// Make sure the private key has the corresponding public key in the series,
	// to be able to empower it.
	for i, publicKey := range series.publicKeys {
		if publicKey.String() == lookingFor {
			found = true
			series.privateKeys[i] = privKey
		}
	}

	if !found {
		str := fmt.Sprintf(
			"private Key does not have a corresponding public key in this series")
		return managerError(ErrKeysPrivatePublicMismatch, str, nil)
	}

	err = vp.saveSeriesToDisk(seriesID, series)

	if err != nil {
		return err
	}

	return nil
}

func (s *seriesData) IsEmpowered() bool {
	for _, key := range s.privateKeys {
		if key != nil {
			return true
		}
	}
	return false
}
