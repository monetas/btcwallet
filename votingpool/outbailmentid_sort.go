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

// byOutBailmentID defines the methods needed to satisify
// sort.Interface to sort a slice of OutputRequests by
// the value of outBailmentIDHash.
type byOutBailmentID []*OutputRequest

func (s byOutBailmentID) Len() int {
	return len(s)
}

func (s byOutBailmentID) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s byOutBailmentID) Less(i, j int) bool {
	return bytes.Compare(s[i].outBailmentIDHash(), s[j].outBailmentIDHash()) < 0
}

// Check at compile time that byOutBailmentID implements the
// sort.Interface.
var _ sort.Interface = (*byOutBailmentID)(nil)
