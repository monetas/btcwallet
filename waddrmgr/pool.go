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
)

type seriesData struct {
	publicKeys []*hdkeychain.ExtendedKey
	reqSigs    uint32
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
		// TODO: This should be a managerError()
		return nil, err
	}
	return &VotingPool{
		ID:           poolID,
		seriesLookup: make(map[uint32]*seriesData),
		manager:      m,
	}, nil
}

func (m *Manager) LoadVotingPool(poolID []byte) (*VotingPool, error) {
	err := m.db.View(func(tx *managerTx) error {
		exists := tx.ExistsVotingPool(poolID)
		if !exists {
			return managerError(0, "FIXME", nil)
		}
		return nil
	})
	if err != nil {
		return nil, managerError(0, "FIXME", err)
	}
	vp := &VotingPool{
		ID:           poolID,
		manager:      m,
		seriesLookup: make(map[uint32]*seriesData),
	}
	if err = vp.LoadAllSeries(); err != nil {
		return nil, managerError(0, "FIXME", err)
	}
	return vp, nil
}

func (vp *VotingPool) GetSeries(seriesID uint32) *seriesData {
	series, exists := vp.seriesLookup[seriesID]
	if !exists {
		return nil
	}
	return series
}

func (vp *VotingPool) CreateSeries(seriesID uint32, rawPubKeys []string, reqSigs uint32) error {

	if _, ok := vp.seriesLookup[seriesID]; ok {
		// TODO: define error codes
		str := fmt.Sprintf("Series #%d already exists", seriesID)
		return managerError(0, str, nil)
	}

	keys := make([]*hdkeychain.ExtendedKey, len(rawPubKeys))
	encryptedKeys := make([][]byte, len(keys))
	sort.Sort(sort.StringSlice(rawPubKeys))

	for i, rawPubKey := range rawPubKeys {
		key, err := hdkeychain.NewKeyFromString(rawPubKey)
		keys[i] = key
		if err != nil {
			str := fmt.Sprintf("Invalid extended public key %v", rawPubKey)
			return managerError(0, str, err)
		}
		if keys[i].IsPrivate() {
			return managerError(0, "Please only use public extended keys", nil)
		}
		encryptedKeys[i], err = vp.manager.cryptoKeyPub.Encrypt([]byte(key.String()))
		if err != nil {
			str := fmt.Sprintf("Key %v failed encryption", rawPubKey)
			return managerError(0, str, err)
		}

	}

	err := vp.manager.db.Update(func(tx *managerTx) error {
		// TODO: check error
		tx.PutSeries(vp.ID, seriesID, reqSigs, encryptedKeys, nil)
		return nil
	})
	if err != nil {
		str := fmt.Sprintf("Cannot put series #%d", seriesID)
		return managerError(0, str, nil)
	}

	vp.seriesLookup[seriesID] = &seriesData{
		publicKeys: keys,
		reqSigs:    reqSigs,
	}

	return nil
}

func (vp *VotingPool) ReplaceSeries(seriesID uint32, publicKeys []*hdkeychain.ExtendedKey, reqSigs uint32) error {
	// TODO(lars): !
	return ErrNotImplemented
}

// XXX: Should this just use seriesLookup instead?
func (vp *VotingPool) ExistsSeries(seriesID uint32) bool {
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

func (vp *VotingPool) LoadAllSeries() error {
	var allSeries map[uint32]*dbSeriesRow
	err := vp.manager.db.View(func(tx *managerTx) error {
		var err error
		allSeries, err = tx.LoadAllSeries(vp.ID)
		return err
	})
	if err != nil {
		return managerError(0, "FIXME", err)
	}
	for id, series := range allSeries {
		keys := make([]*hdkeychain.ExtendedKey, len(series.pubKeysEncrypted))
		for i, data := range series.pubKeysEncrypted {
			decrypted, err := vp.manager.cryptoKeyPub.Decrypt(data)
			if err != nil {
				return managerError(0, "FIXME", err)
			}
			key, err := hdkeychain.NewKeyFromString(string(decrypted))
			zero(decrypted)
			if err != nil {
				return managerError(0, "FIXME", err)
			}
			keys[i] = key
		}
		vp.seriesLookup[id] = &seriesData{
			publicKeys: keys,
			reqSigs:    series.reqSigs}
	}
	return nil
}

// Change the order of the pubkeys based on branch number
//  For 3 pubkeys ABC, this would mean:
// branch 0 - CBA (reversed)
// branch 1 - ABC (first key priority)
// branch 2 - BAC (second key priority)
// branch 3 - CAB (third key priority)
func branchOrder(pks []*btcutil.AddressPubKey, branch uint32) []*btcutil.AddressPubKey {
	if pks == nil {
		return nil
	}

	//change the order of pks based on branch number.
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

	series, ok := vp.seriesLookup[seriesID]
	if !ok {
		str := fmt.Sprintf("Series #%d does not exist", seriesID)
		return nil, managerError(0, str, nil)
	}

	pks := make([]*btcutil.AddressPubKey, len(series.publicKeys))

	for i, key := range series.publicKeys {
		child, err := key.Child(index)
		// implement getting the next index until we find a valid one in case
		// there is a hdkeychain.ErrInvalidChild
		if err != nil {
			str := fmt.Sprintf("Child #%d for this pubkey %d does not exist", index, i)
			return nil, managerError(0, str, err)
		}
		pubkey, err := child.ECPubKey()
		if err != nil {
			str := fmt.Sprintf("Child #%d for this pubkey %d does not exist", index, i)
			return nil, managerError(0, str, err)
		}
		pks[i], err = btcutil.NewAddressPubKey(pubkey.SerializeCompressed(), vp.manager.net)

		if err != nil {
			str := fmt.Sprintf("Child #%d for this pubkey %d could not be converted to an address", index, i)
			return nil, managerError(0, str, err)
		}
	}

	pks = branchOrder(pks, branch)

	script, err := btcscript.MultiSigScript(pks, int(series.reqSigs))
	if err != nil {
		str := fmt.Sprintf("error while making multisig script hash, %d", len(pks))
		return nil, managerError(0, str, err)
	}

	encryptedScript, err := vp.manager.cryptoKeyScript.Encrypt(script)
	if err != nil {
		str := fmt.Sprintf("error while encrypting multisig script hash")
		return nil, managerError(0, str, err)
	}

	scriptHash := btcutil.Hash160(script)

	return newScriptAddress(vp.manager, ImportedAddrAccount, scriptHash, encryptedScript)
}

func (vp *VotingPool) EmpowerBranch(seriesID, branch uint32, key *hdkeychain.ExtendedKey) error {
	// TODO(jimmy): Ensure extended key is private (.IsPrivate)
	return ErrNotImplemented
}

// XXX: Is this necessary or should we just expose seriesData.PublicKeys?
func (s *seriesData) GetPublicKeys() []*hdkeychain.ExtendedKey {
	return s.publicKeys
}
