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

import "testing"

// TestErrorCodeStringer tests that all error codes has a text
// representation and that text representation is still correct,
// ie. that a refactoring and renaming of the error code has not
// drifted from the textual representation.
func TestErrorCodeStringer(t *testing.T) {
	// All the errors in ths
	tests := []struct {
		in   ErrorCode
		want string
	}{
		{ErrInputSelection, "ErrInputSelection"},
		{ErrInvalidAddressRange, "ErrInvalidAddressRange"},
	}

	if int(lastErr) != len(tests) {
		t.Errorf("Wrong number of errorCodeStrings. Got: %d, want: %d",
			int(lastErr), len(tests))
	}

	for i, test := range tests {
		result := test.in.String()
		if result != test.want {
			t.Errorf("String #%d\ngot: %s\nwant: %s", i, result,
				test.want)
		}
	}
}
