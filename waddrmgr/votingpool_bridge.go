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

// XXX(lars): This file exists only to expose functionality needed in
// the votingpool package, hence the name votingpool_bridge.  The
// functionality herein is supposed to be obsoleted as soon as
// conformal adds the new database framework to the master branch, and
// users of these functions need to be updated accordingly.

import (
	"fmt"

	"github.com/conformal/bolt"
)

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
// XXX(lars): maybe we can get dbSeriesRow made public in db.go?
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
