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

package votingpool

import "fmt"

// ErrorCode identifies a kind of error
type ErrorCode int

const (
	// ErrInputSelection indicates an error in the input selection
	// algorithm.
	ErrInputSelection ErrorCode = iota

	// ErrInvalidAddressRange indicates an invalid address range.
	ErrInvalidAddressRange

	// ErrWithdrawalProcessing indicates an internal error when processing a withdrawal
	// request.
	ErrWithdrawalProcessing

	// lastErr is used for testing, making it possible to iterate over
	// the error codes in order to check that they all have proper
	// translations in errorCodeStrings.
	lastErr
)

// Map of ErrorCode values back to their constant names for pretty printing.
var errorCodeStrings = map[ErrorCode]string{
	ErrInputSelection:       "ErrInputSelection",
	ErrInvalidAddressRange:  "ErrInvalidAddressRange",
	ErrWithdrawalProcessing: "ErrWithdrawalProcessing",
}

// String returns the ErrorCode as a human-readable name.
func (e ErrorCode) String() string {
	if s := errorCodeStrings[e]; s != "" {
		return s
	}
	return fmt.Sprintf("Unknown ErrorCode (%d)", int(e))
}

// Error is a typed error for all errors arising during the
// operation of the voting pool.
type Error struct {
	ErrorCode   ErrorCode // Describes the kind of error
	Description string    // Human readable description of the issue
	Err         error     // Underlying error
}

// Error satisfies the error interface and prints human-readable errors.
func (e Error) Error() string {
	if e.Err != nil {
		return e.Description + ": " + e.Err.Error()
	}
	return e.Description
}

// newError creates a new Error.
func newError(c ErrorCode, desc string, err error) Error {
	return Error{ErrorCode: c, Description: desc, Err: err}
}
