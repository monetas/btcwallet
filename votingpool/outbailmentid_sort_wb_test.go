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
	"reflect"
	"sort"
	"testing"
)

// TestOutBailmentIDSort tests that the we can correctly sort a slice
// of output requests by the hash of the outbailmentID.
func TestOutBailmentIDSort(t *testing.T) {
	or00 := &OutputRequest{cachedHash: []byte{0, 0}}
	or01 := &OutputRequest{cachedHash: []byte{0, 1}}
	or10 := &OutputRequest{cachedHash: []byte{1, 0}}
	or11 := &OutputRequest{cachedHash: []byte{1, 1}}

	want := []*OutputRequest{or00, or01, or10, or11}
	random := []*OutputRequest{or11, or00, or10, or01}

	sort.Sort(byOutBailmentID(random))

	if !reflect.DeepEqual(random, want) {
		t.Fatalf("Sort failed; got %v, want %v", random, want)
	}
}
