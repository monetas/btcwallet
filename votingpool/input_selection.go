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

// VotingPoolAddress reprents the unique data needed to generate a
// voting pool address.
type VotingPoolAddress struct {
	SeriesID uint32
	Index    uint32
	Branch   uint32
}

// CreditInterface is an abstraction over credits used in a voting
// pool.
type CreditInterface interface {
	TxID() *btcwire.ShaHash
	OutputIndex() uint32
	Address() VotingPoolAddress
}

// Credit implements the CreditInterface.
type Credit struct {
	Addr   VotingPoolAddress
	Credit txstore.Credit
}

// TxID returns the sha hash of the underlying transaction.
func (c Credit) TxID() *btcwire.ShaHash {
	return c.Credit.TxRecord.Tx().Sha()
}

// OutputIndex returns the outputindex of the ouput in the underlying
// transaction.
func (c Credit) OutputIndex() uint32 {
	return c.Credit.OutputIndex
}

// Address returns the voting pool address.
func (c Credit) Address() VotingPoolAddress {
	return c.Addr
}

// newCredit initialises a new Credit.
func newCredit(credit txstore.Credit, seriesID, branch, index uint32) Credit {
	return Credit{
		Credit: credit,
		Addr: VotingPoolAddress{
			SeriesID: seriesID,
			Branch:   branch,
			Index:    index,
		},
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
// Branch, TxID, OutputIndex).
func (c Credits) Less(i, j int) bool {
	if c[i].Address().SeriesID < c[j].Address().SeriesID {
		return true
	}

	if c[i].Address().SeriesID == c[j].Address().SeriesID &&
		c[i].Address().Index < c[j].Address().Index {
		return true
	}

	if c[i].Address().SeriesID == c[j].Address().SeriesID &&
		c[i].Address().Index == c[j].Address().Index &&
		c[i].Address().Branch < c[j].Address().Branch {
		return true
	}

	txidComparison := bytes.Compare(c[i].TxID().Bytes(), c[j].TxID().Bytes())

	if c[i].Address().SeriesID == c[j].Address().SeriesID &&
		c[i].Address().Index == c[j].Address().Index &&
		c[i].Address().Branch == c[j].Address().Branch &&
		txidComparison < 0 {
		return true
	}

	if c[i].Address().SeriesID == c[j].Address().SeriesID &&
		c[i].Address().Index == c[j].Address().Index &&
		c[i].Address().Branch == c[j].Address().Branch &&
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
	StartBranch, StopBranch uint32
	StartIndex, StopIndex   uint32
}

// NumAddresses returns the number of addresses this range represents.
func (r AddressRange) NumAddresses() (uint64, error) {
	if r.StartBranch > r.StopBranch {
		// TODO: define a proper error message
		return 0, errors.New("range not defined when StartBranch > StopBranch")
	}
	if r.StartIndex > r.StopIndex {
		// TODO: define a proper error message
		return 0, errors.New("range not defined when StartIndex > StopIndex")
	}

	return uint64((r.StopBranch - r.StartBranch + 1)) *
		uint64((r.StopIndex - r.StartIndex + 1)), nil
}

// getEligibleInputs returns all the eligible inputs from the
// specified ranges.
func (vp *VotingPool) getEligibleInputs(store *txstore.Store,
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
func (vp *VotingPool) getEligibleInputsFromSeries(store *txstore.Store,
	aRange AddressRange,
	dustThreshold btcutil.Amount, chainHeight int32,
	minConf int) (Credits, error) {
	unspents, err := store.UnspentOutputs()
	if err != nil {
		// TODO: consider if we need to create a new error.
		return nil, compositeError("input selection failed:", err)
	}

	addrMap, err := addrToUtxosMap(unspents, vp.manager.Net())
	if err != nil {
		// TODO: consider if we need to create a new error.
		return nil, compositeError("input selection failed:", err)
	}
	var inputs Credits
	for index := aRange.StartIndex; index <= aRange.StopIndex; index++ {
		for branch := aRange.StartBranch; branch <= aRange.StopBranch; branch++ {
			addr, err := vp.DepositScriptAddress(aRange.SeriesID, branch, index)
			if err != nil {
				// TODO: consider if we need to create a new error.
				return nil, compositeError("input selection failed:", err)

			}
			encAddr := addr.EncodeAddress()

			if candidates, ok := addrMap[encAddr]; ok {
				var eligibles Credits
				for _, c := range candidates {
					if vp.eligible(c, minConf, chainHeight, dustThreshold) {
						vpc := newCredit(c, aRange.SeriesID, branch, index)
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

func compositeError(errString string, err error) error {
	return errors.New(errString + ": " + err.Error())
}

// addrToUtxosMap converts a slice of credits to a map from the string
// representation of an encoded address to the unspent outputs
// associated with that address.
func addrToUtxosMap(utxos []txstore.Credit, net *btcnet.Params) (map[string][]txstore.Credit, error) {
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

// eligible tests a given credit for eligibilty with respect to number
// of confirmations, the dust threshold and that it is not the charter
// output.
func (vp *VotingPool) eligible(c txstore.Credit, minConf int, currentBlockHeight int32, dustThreshold btcutil.Amount) bool {
	if c.Amount() < dustThreshold {
		return false
	}
	if !c.Confirmed(minConf, currentBlockHeight) {
		return false
	}
	if vp.isCharterOutput(c) {
		return false
	}

	return true
}

// isCharterInput - TODO: In order to determine this, we need the txid
// and the output index of the current charter output
func (vp *VotingPool) isCharterOutput(c txstore.Credit) bool {
	return false
}
