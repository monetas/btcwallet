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

import (
	"bytes"
	"sort"
)

// sortByOutBailmentID is a type used solely for sorting slices of
// *OutputRequest.
type sortByOutBailmentID []*OutputRequest

// Len returns the length of the underlying slice.
func (s sortByOutBailmentID) Len() int {
	return len(s)
}

// Swap swaps the elements at position i and j.
func (s sortByOutBailmentID) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns true if the hash of the outBailmentID of the ith
// element is less than the hash of the outBailmentID of the jth
// element.
func (s sortByOutBailmentID) Less(i, j int) bool {
	return bytes.Compare(s[i].outBailmentID.hash(), s[j].outBailmentID.hash()) < 0
}

// Check at compile time that sortByOutBailmentID implements sort.Interface.
var _ sort.Interface = (*sortByOutBailmentID)(nil)
