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
	"strconv"

	"github.com/monetas/btcutil/hdkeychain"
)

var (
	// errAlreadyExists is the common error description used for the
	// ErrAlreadyExists error code.
	errAlreadyExists = "the specified address manager already exists"

	// errCoinTypeTooHigh is the common error description used for the
	// ErrCoinTypeTooHigh error code.
	errCoinTypeTooHigh = "coin type may not exceed " +
		strconv.FormatUint(hdkeychain.HardenedKeyStart-1, 10)

	// errAcctTooHigh is the common error description used for the
	// ErrAccountNumTooHigh error code.
	errAcctTooHigh = "account number may not exceed " +
		strconv.FormatUint(hdkeychain.HardenedKeyStart-1, 10)

	// errLocked is the common error description used for the ErrLocked
	// error code.
	errLocked = "address manager is locked"

	// errWatchingOnly is the common error description used for the
	// ErrWatchingOnly error code.
	errWatchingOnly = "address manager is watching-only"
)

// ErrorCode identifies a kind of error.
type ErrorCode int

// These constants are used to identify a specific ManagerError.
const (
	// ErrDatabase indicates an error with the underlying database.  When
	// this error code is set, the Err field of the ManagerError will be
	// set to the underlying error returned from the database.
	ErrDatabase ErrorCode = iota

	// ErrKeyChain indicates an error with the key chain typically either
	// due to the inability to create an extended key or deriving a child
	// extended key.  When this error code is set, the Err field of the
	// ManagerError will be set to the underlying error.
	ErrKeyChain

	// ErrCrypto indicates an error with the cryptography related operations
	// such as decrypting or encrypting data, parsing an EC public key,
	// or deriving a secret key from a password.  When this error code is
	// set, the Err field of the ManagerError will be set to the underlying
	// error.
	ErrCrypto

	// ErrInvalidKeyType indicates an error where an invalid crypto
	// key type has been selected.
	ErrInvalidKeyType

	// ErrNoExist indicates that the specified database does not exist.
	ErrNoExist

	// ErrAlreadyExists indicates that the specified database already exists.
	ErrAlreadyExists

	// ErrCoinTypeTooHigh indicates that the coin type specified in the provided
	// network parameters is higher than the max allowed value as defined
	// by the maxCoinType constant.
	ErrCoinTypeTooHigh

	// ErrAccountNumTooHigh indicates that the specified account number is higher
	// than the max allowed value as defined by the MaxAccountNum constant.
	ErrAccountNumTooHigh

	// ErrLocked indicates that an operation, which requires the account
	// manager to be unlocked, was requested on a locked account manager.
	ErrLocked

	// ErrWatchingOnly indicates that an operation, which requires the
	// account manager to have access to private data, was requested on
	// a watching-only account manager.
	ErrWatchingOnly

	// ErrInvalidAccount indicates that the requested account is not valid.
	ErrInvalidAccount

	// ErrAddressNotFound indicates that the requested address is not known to
	// the account manager.
	ErrAddressNotFound

	// ErrAccountNotFound indicates that the requested account is not known to
	// the account manager.
	ErrAccountNotFound

	// ErrDuplicate indicates that an address already exists.
	ErrDuplicate

	// ErrTooManyAddresses indicates that more than the maximum allowed number of
	// addresses per account have been requested.
	ErrTooManyAddresses

	// ErrWrongPassphrase indicates that the specified passphrase is incorrect.
	// This could be for either public or private master keys.
	ErrWrongPassphrase

	// ErrWrongNet indicates that the private key to be imported is not for the
	// the same network the account manager is configured for.
	ErrWrongNet
)

// Map of ErrorCode values back to their constant names for pretty printing.
var errorCodeStrings = map[ErrorCode]string{
	ErrDatabase:          "ErrDatabase",
	ErrKeyChain:          "ErrKeyChain",
	ErrCrypto:            "ErrCrypto",
	ErrInvalidKeyType:    "ErrInvalidKeyType",
	ErrNoExist:           "ErrNoExist",
	ErrAlreadyExists:     "ErrAlreadyExists",
	ErrCoinTypeTooHigh:   "ErrCoinTypeTooHigh",
	ErrAccountNumTooHigh: "ErrAccountNumTooHigh",
	ErrLocked:            "ErrLocked",
	ErrWatchingOnly:      "ErrWatchingOnly",
	ErrInvalidAccount:    "ErrInvalidAccount",
	ErrAddressNotFound:   "ErrAddressNotFound",
	ErrAccountNotFound:   "ErrAccountNotFound",
	ErrDuplicate:         "ErrDuplicate",
	ErrTooManyAddresses:  "ErrTooManyAddresses",
	ErrWrongPassphrase:   "ErrWrongPassphrase",
	ErrWrongNet:          "ErrWrongNet",

	// The following error codes are defined in pool_error.go.
	ErrSeriesStorage:             "ErrSeriesStorage",
	ErrSeriesVersion:             "ErrSeriesVersion",
	ErrSeriesNotExists:           "ErrSeriesNotExists",
	ErrSeriesAlreadyExists:       "ErrSeriesAlreadyExists",
	ErrSeriesAlreadyEmpowered:    "ErrSeriesAlreadyEmpowered",
	ErrKeyIsPrivate:              "ErrKeyIsPrivate",
	ErrKeyIsPublic:               "ErrKeyIsPublic",
	ErrKeyNeuter:                 "ErrKeyNeuter",
	ErrKeyMismatch:               "ErrKeyMismatch",
	ErrKeysPrivatePublicMismatch: "ErrKeysPrivatePublicMismatch",
	ErrKeyDuplicate:              "ErrKeyDuplicate",
	ErrTooFewPublicKeys:          "ErrTooFewPublicKeys",
	ErrVotingPoolAlreadyExists:   "ErrVotingPoolAlreadyExists",
	ErrVotingPoolNotExists:       "ErrVotingPoolNotExists",
	ErrScriptCreation:            "ErrScriptCreation",
	ErrTooManyReqSignatures:      "ErrTooManyReqSignatures",
	ErrInvalidBranch:             "ErrInvalidBranch",
	ErrInvalidValue:              "ErrInvalidValue",
}

// String returns the ErrorCode as a human-readable name.
func (e ErrorCode) String() string {
	if s := errorCodeStrings[e]; s != "" {
		return s
	}
	return fmt.Sprintf("Unknown ErrorCode (%d)", int(e))
}

// ManagerError provides a single type for errors that can happen during address
// manager operation.  It is used to indicate several types of failures
// including errors with caller requests such as invalid accounts or requesting
// private keys against a locked address manager, errors with the database
// (ErrDatabase), errors with key chain derivation (ErrKeyChain), and errors
// related to crypto (ErrCrypto).
//
// The caller can use type assertions to determine if an error is a ManagerError
// and access the ErrorCode field to ascertain the specific reason for the
// failure.
//
// The ErrDatabase, ErrKeyChain, and ErrCrypto error codes will also have the
// Err field set with the underlying error.
type ManagerError struct {
	ErrorCode   ErrorCode // Describes the kind of error
	Description string    // Human readable description of the issue
	Err         error     // Underlying error
}

// Error satisfies the error interface and prints human-readable errors.
func (e ManagerError) Error() string {
	if e.Err != nil {
		return e.Description + ": " + e.Err.Error()
	}
	return e.Description
}

// managerError creates a ManagerError given a set of arguments.
func managerError(c ErrorCode, desc string, err error) ManagerError {
	return ManagerError{ErrorCode: c, Description: desc, Err: err}
}
