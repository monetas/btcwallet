/*
 * Copyright (c) 2013, 2014 Conformal Systems LLC <info@conformal.com>
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

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/monetas/btcnet"
	"github.com/monetas/btcwire"
)

// networkDir returns the directory name of a network directory to hold account
// files.
func networkDir(net *btcnet.Params) string {
	netname := net.Name

	// For now, we must always name the testnet data directory as "testnet"
	// and not "testnet3" or any other version, as the btcnet testnet3
	// paramaters will likely be switched to being named "testnet3" in the
	// future.  This is done to future proof that change, and an upgrade
	// plan to move the testnet3 data directory can be worked out later.
	if net.Net == btcwire.TestNet3 {
		netname = "testnet"
	}

	return filepath.Join(cfg.DataDir, netname)
}

// tmpNetworkDir returns the temporary directory name for a given network.
func tmpNetworkDir(net *btcnet.Params) string {
	return networkDir(net) + "_tmp"
}

// freshDir creates a new directory specified by path if it does not
// exist.  If the directory already exists, all files contained in the
// directory are removed.
func freshDir(path string) error {
	if err := checkCreateDir(path); err != nil {
		return err
	}

	// Remove all files in the directory.
	fd, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if err := fd.Close(); err != nil {
			log.Warnf("Cannot close directory: %v", err)
		}
	}()
	names, err := fd.Readdirnames(0)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := os.RemoveAll(name); err != nil {
			return err
		}
	}

	return nil
}

// checkCreateDir checks that the path exists and is a directory.
// If path does not exist, it is created.
func checkCreateDir(path string) error {
	if fi, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			// Attempt data directory creation
			if err = os.MkdirAll(path, 0700); err != nil {
				return fmt.Errorf("cannot create directory: %s", err)
			}
		} else {
			return fmt.Errorf("error checking directory: %s", err)
		}
	} else {
		if !fi.IsDir() {
			return fmt.Errorf("path '%s' is not a directory", path)
		}
	}

	return nil
}

// accountFilename returns the filepath of an account file given the
// filename suffix ("wallet.bin", or "tx.bin"), account name and the
// network directory holding the file.
func accountFilename(suffix, account, netdir string) string {
	if account == "" {
		// default account
		return filepath.Join(netdir, suffix)
	}

	// non-default account
	return filepath.Join(netdir, fmt.Sprintf("%v-%v", account, suffix))
}

// syncSchedule references the account files which have been
// scheduled to be written and the directory to write to.
type syncSchedule struct {
	dir     string
	wallets map[*Account]struct{}
	txs     map[*Account]struct{}
}

func newSyncSchedule(dir string) *syncSchedule {
	s := &syncSchedule{
		dir:     dir,
		wallets: make(map[*Account]struct{}),
		txs:     make(map[*Account]struct{}),
	}
	return s
}

// flushAccount writes all scheduled account files to disk for
// a single account and removes them from the schedule.
func (s *syncSchedule) flushAccount(a *Account) error {
	if _, ok := s.txs[a]; ok {
		if err := a.writeTxStore(s.dir); err != nil {
			return err
		}
		delete(s.txs, a)
	}
	if _, ok := s.wallets[a]; ok {
		if err := a.writeWallet(s.dir); err != nil {
			return err
		}
		delete(s.wallets, a)
	}

	return nil
}

// flush writes all scheduled account files and removes each
// from the schedule.
func (s *syncSchedule) flush() error {
	for a := range s.txs {
		if err := a.writeTxStore(s.dir); err != nil {
			return err
		}
		delete(s.txs, a)
	}

	for a := range s.wallets {
		if err := a.writeWallet(s.dir); err != nil {
			return err
		}
		delete(s.wallets, a)
	}

	return nil
}

type flushAccountRequest struct {
	a   *Account
	err chan error
}

type writeBatchRequest struct {
	a   []*Account
	err chan error
}

type exportRequest struct {
	dir string
	a   *Account
	err chan error
}

// DiskSyncer manages all disk write operations for a collection of accounts.
type DiskSyncer struct {
	// Flush scheduled account writes.
	flushAccount chan *flushAccountRequest

	// Schedule file writes for an account.
	scheduleWallet  chan *Account
	scheduleTxStore chan *Account

	// Write a collection of accounts all at once.
	writeBatch chan *writeBatchRequest

	// Write an account export.
	exportAccount chan *exportRequest

	// Account manager for this DiskSyncer.  This is only
	// needed to grab the account manager semaphore.
	am *AccountManager

	quit     chan struct{}
	shutdown chan struct{}
}

// NewDiskSyncer creates a new DiskSyncer.
func NewDiskSyncer(am *AccountManager) *DiskSyncer {
	return &DiskSyncer{
		flushAccount:    make(chan *flushAccountRequest),
		scheduleWallet:  make(chan *Account),
		scheduleTxStore: make(chan *Account),
		writeBatch:      make(chan *writeBatchRequest),
		exportAccount:   make(chan *exportRequest),
		am:              am,
		quit:            make(chan struct{}),
		shutdown:        make(chan struct{}),
	}
}

// Start starts the goroutines required to run the DiskSyncer.
func (ds *DiskSyncer) Start() {
	go ds.handler()
}

func (ds *DiskSyncer) Stop() {
	close(ds.quit)
}

func (ds *DiskSyncer) WaitForShutdown() {
	<-ds.shutdown
}

// handler runs the disk syncer.  It manages a set of "dirty" account files
// which must be written to disk, and synchronizes all writes in a single
// goroutine.  Periodic flush operations may be signaled by an AccountManager.
//
// This never returns and is should be called from a new goroutine.
func (ds *DiskSyncer) handler() {
	netdir := networkDir(activeNet.Params)
	if err := checkCreateDir(netdir); err != nil {
		log.Errorf("Unable to create or write to account directory: %v", err)
	}
	tmpnetdir := tmpNetworkDir(activeNet.Params)

	const wait = 10 * time.Second
	var timer <-chan time.Time
	var sem chan struct{}
	schedule := newSyncSchedule(netdir)
out:
	for {
		select {
		case <-sem: // Now have exclusive access of the account manager
			err := schedule.flush()
			if err != nil {
				log.Errorf("Cannot write accounts: %v", err)
			}

			timer = nil

			// Do not grab semaphore again until another flush is needed.
			sem = nil

			// Release semaphore.
			ds.am.bsem <- struct{}{}

		case <-timer:
			// Grab AccountManager semaphore when ready so flush can occur.
			sem = ds.am.bsem

		case fr := <-ds.flushAccount:
			fr.err <- schedule.flushAccount(fr.a)

		case a := <-ds.scheduleWallet:
			schedule.wallets[a] = struct{}{}
			if timer == nil {
				timer = time.After(wait)
			}

		case a := <-ds.scheduleTxStore:
			schedule.txs[a] = struct{}{}
			if timer == nil {
				timer = time.After(wait)
			}

		case sr := <-ds.writeBatch:
			err := batchWriteAccounts(sr.a, tmpnetdir, netdir)
			if err == nil {
				// All accounts have been synced, old schedule
				// can be discarded.
				schedule = newSyncSchedule(netdir)
				timer = nil
			}
			sr.err <- err

		case er := <-ds.exportAccount:
			a := er.a
			dir := er.dir
			er.err <- a.writeAll(dir)

		case <-ds.quit:
			err := schedule.flush()
			if err != nil {
				log.Errorf("Cannot write accounts: %v", err)
			}
			break out
		}
	}
	close(ds.shutdown)
}

// FlushAccount writes all scheduled account files to disk for a single
// account.
func (ds *DiskSyncer) FlushAccount(a *Account) error {
	err := make(chan error)
	ds.flushAccount <- &flushAccountRequest{a: a, err: err}
	return <-err
}

// ScheduleWalletWrite schedules an account's wallet to be written to disk.
func (ds *DiskSyncer) ScheduleWalletWrite(a *Account) {
	ds.scheduleWallet <- a
}

// ScheduleTxStoreWrite schedules an account's transaction store to be
// written to disk.
func (ds *DiskSyncer) ScheduleTxStoreWrite(a *Account) {
	ds.scheduleTxStore <- a
}

// WriteBatch safely replaces all account files in the network directory
// with new files created from all accounts in a.
func (ds *DiskSyncer) WriteBatch(a []*Account) error {
	err := make(chan error)
	ds.writeBatch <- &writeBatchRequest{
		a:   a,
		err: err,
	}
	return <-err
}

// ExportAccount writes all account files for a to a new directory.
func (ds *DiskSyncer) ExportAccount(a *Account, dir string) error {
	err := make(chan error)
	er := &exportRequest{
		dir: dir,
		a:   a,
		err: err,
	}
	ds.exportAccount <- er
	return <-err
}

func batchWriteAccounts(accts []*Account, tmpdir, netdir string) error {
	if err := freshDir(tmpdir); err != nil {
		return err
	}
	for _, a := range accts {
		if err := a.writeAll(tmpdir); err != nil {
			return err
		}
	}
	// This is technically NOT an atomic operation, but at startup, if the
	// network directory is missing but the temporary network directory
	// exists, the temporary is moved before accounts are opened.
	if err := os.RemoveAll(netdir); err != nil {
		return err
	}
	if err := Rename(tmpdir, netdir); err != nil {
		return err
	}
	return nil
}

func (a *Account) writeAll(dir string) error {
	if err := a.writeTxStore(dir); err != nil {
		return err
	}
	if err := a.writeWallet(dir); err != nil {
		return err
	}
	return nil
}

func (a *Account) writeWallet(dir string) error {
	wfilepath := accountFilename("wallet.bin", a.name, dir)
	_, filename := filepath.Split(wfilepath)
	tmpfile, err := ioutil.TempFile(dir, filename)
	if err != nil {
		return err
	}
	if _, err = a.Wallet.WriteTo(tmpfile); err != nil {
		return err
	}

	tmppath := tmpfile.Name()
	if err := tmpfile.Sync(); err != nil {
		log.Warnf("Failed to sync temporary wallet file %s: %v",
			tmppath, err)
	}

	if err := tmpfile.Close(); err != nil {
		log.Warnf("Cannot close temporary wallet file %s: %v",
			tmppath, err)
	}

	return Rename(tmppath, wfilepath)
}

func (a *Account) writeTxStore(dir string) error {
	txfilepath := accountFilename("tx.bin", a.name, dir)
	_, filename := filepath.Split(txfilepath)
	tmpfile, err := ioutil.TempFile(dir, filename)
	if err != nil {
		return err
	}

	if _, err = a.TxStore.WriteTo(tmpfile); err != nil {
		return err
	}

	tmppath := tmpfile.Name()
	if err := tmpfile.Sync(); err != nil {
		log.Warnf("Failed to sync temporary txstore file %s: %v",
			tmppath, err)
	}

	if err := tmpfile.Close(); err != nil {
		log.Warnf("Cannot close temporary txstore file %s: %v",
			tmppath, err)
	}

	return Rename(tmppath, txfilepath)
}
