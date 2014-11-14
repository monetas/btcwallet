package votingpool

import (
	"bytes"
	"errors"
	"sort"

	"github.com/conformal/btcnet"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwire"
)

// CreditInterface is an abstraction over credits used in a voting
// pool.
type CreditInterface interface {
	TxSha() *btcwire.ShaHash
	OutputIndex() uint32
	Address() WithdrawalAddress
	Amount() btcutil.Amount
	OutPoint() *btcwire.OutPoint
	TxOut() *btcwire.TxOut
}

// Credit implements the CreditInterface.
type Credit struct {
	txstore.Credit
	addr WithdrawalAddress
}

func NewCredit(credit txstore.Credit, addr WithdrawalAddress) Credit {
	return Credit{Credit: credit, addr: addr}
}

// TxSha returns the sha hash of the underlying transaction.
func (c Credit) TxSha() *btcwire.ShaHash {
	return c.Credit.TxRecord.Tx().Sha()
}

// OutputIndex returns the outputindex of the ouput in the underlying
// transaction.
func (c Credit) OutputIndex() uint32 {
	return c.Credit.OutputIndex
}

// Address returns the voting pool address.
func (c Credit) Address() WithdrawalAddress {
	return c.addr
}

// newCredit initialises a new Credit.
func newCredit(credit txstore.Credit, addr WithdrawalAddress) Credit {
	return Credit{
		Credit: credit,
		addr:   addr,
	}
}

// Credits is a type defined as a slice of CreditInterface
// implementing the sort.Interface.
type Credits []CreditInterface

// Len returns the length of the underlying slice.
func (c Credits) Len() int {
	return len(c)
}

// Less returns true if the element at positions i is smaller than the
// element at position j. The 'smaller-than' relation is defined to be
// the lexicographic ordering defined on the tuple (SeriesID, Index,
// Branch, TxSha, OutputIndex).
func (c Credits) Less(i, j int) bool {
	if c[i].Address().seriesID < c[j].Address().seriesID {
		return true
	}

	if c[i].Address().seriesID == c[j].Address().seriesID &&
		c[i].Address().index < c[j].Address().index {
		return true
	}

	if c[i].Address().seriesID == c[j].Address().seriesID &&
		c[i].Address().index == c[j].Address().index &&
		c[i].Address().branch < c[j].Address().branch {
		return true
	}

	txidComparison := bytes.Compare(c[i].TxSha().Bytes(), c[j].TxSha().Bytes())

	if c[i].Address().seriesID == c[j].Address().seriesID &&
		c[i].Address().index == c[j].Address().index &&
		c[i].Address().branch == c[j].Address().branch &&
		txidComparison < 0 {
		return true
	}

	if c[i].Address().seriesID == c[j].Address().seriesID &&
		c[i].Address().index == c[j].Address().index &&
		c[i].Address().branch == c[j].Address().branch &&
		txidComparison == 0 &&
		c[i].OutputIndex() < c[j].OutputIndex() {
		return true
	}
	return false
}

// Swap swaps the elements at position i and j.
func (c Credits) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

// Check at compile time that Credits implements sort.Interface.
var _ sort.Interface = (*Credits)(nil)

// AddressRange defines a range in the address space of the series.
type AddressRange struct {
	SeriesID                uint32
	StartBranch, StopBranch Branch
	StartIndex, StopIndex   Index
}

// NumAddresses returns the number of addresses this range represents.
func (r AddressRange) NumAddresses() (uint64, error) {
	// XXX(lars): the range validity checking should be moved to a
	// NewAddressRange constructor function instead and then the
	// AddressRange should be made private.
	if r.StartBranch > r.StopBranch {
		str := "range not defined when StartBranch > StopBranch"
		return 0, newError(ErrInvalidAddressRange, str, nil)
	}
	if r.StartIndex > r.StopIndex {
		str := "range not defined when StartIndex > StopIndex"
		return 0, newError(ErrInvalidAddressRange, str, nil)
	}

	return uint64((r.StopBranch - r.StartBranch + 1)) *
		uint64((r.StopIndex - r.StartIndex + 1)), nil
}

// getEligibleInputs returns all the eligible inputs from the
// specified ranges.
//
// XXX(lars): We might want to return []txstore.Credit instead of
// Credits. The caller has no use of getting the sortable type.
func (vp *Pool) getEligibleInputs(store *txstore.Store,
	ranges []AddressRange,
	dustThreshold btcutil.Amount, chainHeight int32,
	minConf int) (Credits, error) {

	var inputs Credits
	for _, r := range ranges {
		credits, err := vp.getEligibleInputsFromSeries(store, r, dustThreshold, chainHeight, minConf)
		if err != nil {
			return nil, err
		}

		inputs = append(inputs, credits...)
	}
	return inputs, nil
}

// getEligibleInputsFromSeries returns a slice of eligible inputs for a series.
//
// XXX(lars): We might want to return []txstore.Credit instead of
// Credits. The caller has no use of getting the sortable type.
func (vp *Pool) getEligibleInputsFromSeries(store *txstore.Store,
	aRange AddressRange,
	dustThreshold btcutil.Amount, chainHeight int32,
	minConf int) (Credits, error) {
	unspents, err := store.UnspentOutputs()
	if err != nil {
		return nil, newError(ErrInputSelection, "failed to get unspent outputs", err)
	}

	addrMap, err := groupCreditsByAddr(unspents, vp.manager.Net())
	if err != nil {
		return nil, newError(ErrInputSelection, "grouping credits by address failed", err)
	}
	var inputs Credits
	for index := aRange.StartIndex; index <= aRange.StopIndex; index++ {
		for branch := aRange.StartBranch; branch <= aRange.StopBranch; branch++ {
			addr, err := vp.WithdrawalAddress(aRange.SeriesID, branch, index)
			if err != nil {
				return nil, newError(ErrInputSelection, "failed to create deposit address", err)
			}
			encAddr := addr.Addr().EncodeAddress()

			if candidates, ok := addrMap[encAddr]; ok {
				var eligibles Credits
				for _, c := range candidates {
					if vp.isCreditEligible(c, minConf, chainHeight, dustThreshold) {
						vpc := newCredit(c, *addr)
						eligibles = append(eligibles, vpc)
					}
				}
				// Make sure the eligibles are correctly sorted.
				sort.Sort(eligibles)
				inputs = append(inputs, eligibles...)
			}
		}
	}

	return inputs, nil
}

// groupCreditsByAddr converts a slice of credits to a map from the
// string representation of an encoded address to the unspent outputs
// associated with that address.
func groupCreditsByAddr(utxos []txstore.Credit, net *btcnet.Params) (map[string][]txstore.Credit, error) {
	addrMap := make(map[string][]txstore.Credit)
	for _, o := range utxos {
		_, addrs, _, err := o.Addresses(net)
		if err != nil {
			return nil, err
		}
		// As our utxos are all scripthashes we should never have more
		// than one address per output, so let's error out if that
		// assumption is violated.
		if len(addrs) != 1 {
			// TODO: This should probably be a VotingPoolError, but
			// this function only exists because we don't have a good
			// way to lookup credits by addr on the store, and this
			// will most likely change, so leaving it for now.
			return nil, errors.New("one address per unspent output assumption violated")
		}
		encAddr := addrs[0].EncodeAddress()
		if v, ok := addrMap[encAddr]; ok {
			addrMap[encAddr] = append(v, o)
		} else {
			addrMap[encAddr] = []txstore.Credit{o}
		}
	}

	return addrMap, nil
}

// isCreditEligible tests a given credit for eligibilty with respect
// to number of confirmations, the dust threshold and that it is not
// the charter output.
func (vp *Pool) isCreditEligible(c txstore.Credit, minConf int, chainHeight int32, dustThreshold btcutil.Amount) bool {
	if c.Amount() < dustThreshold {
		return false
	}
	if !c.Confirmed(minConf, chainHeight) {
		return false
	}
	if vp.isCharterOutput(c) {
		return false
	}

	return true
}

// isCharterInput - TODO: In order to determine this, we need the txid
// and the output index of the current charter output
func (vp *Pool) isCharterOutput(c txstore.Credit) bool {
	return false
}
