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

// Distance calculates the number of addresses between a and b. If not
// each field on a is less than or equal the fiels on b an error is
// returned.
func (a VotingPoolAddress) Distance(b VotingPoolAddress) (uint64, error) {
	if a.SeriesID > b.SeriesID {
		// TODO: define a proper error message
		return 0, errors.New("distance not defined when a.SeriesID > b.SeriesID")
	}
	if a.Branch > b.Branch {
		// TODO: define a proper error message
		return 0, errors.New("distance not defined when a.Branch > b.Branch")
	}
	if a.Index > b.Index {
		// TODO: define a proper error message
		return 0, errors.New("distance not defined when a.Index > b.Index")
	}

	return uint64((b.SeriesID - a.SeriesID + 1)) *
		uint64((b.Branch - a.Branch + 1)) *
		uint64((b.Index - a.Index + 1)), nil
}

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
// element at position j.
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

// InputSelection returns a slice of eligible inputs in the address
// ranged specified by the start and stop parameters.
func (vp *VotingPool) InputSelection(store *txstore.Store,
	start, stop VotingPoolAddress,
	dustThreshold btcutil.Amount, chainHeight int32,
	minConf int) (Credits, error) {
	unspents, err := store.UnspentOutputs()
	if err != nil {
		// TODO: consider if we need to create a new error.
		compositeError("input selection failed:", err)
		return nil, err
	}

	addrMap, err := AddrToUtxosMap(unspents, vp.manager.Net())
	if err != nil {
		// TODO: consider if we need to create a new error.
		return nil, compositeError("input selection failed:", err)
	}
	var inputs Credits
	for series := start.SeriesID; series <= stop.SeriesID; series++ {
		for index := start.Index; index <= stop.Index; index++ {
			for branch := start.Branch; branch <= stop.Branch; branch++ {
				addr, err := vp.DepositScriptAddress(series, branch, index)
				if err != nil {
					// TODO: consider if we need to create a new error.
					return nil, compositeError("input selection failed:", err)

				}
				encAddr := addr.EncodeAddress()

				if candidates, ok := addrMap[encAddr]; ok {
					var eligibles Credits
					for _, c := range candidates {
						if Eligible(c, minConf, chainHeight, dustThreshold) {
							vpc := newCredit(c, series, branch, index)
							eligibles = append(eligibles, vpc)
						}
					}
					// Make sure the eligibles are correctly sorted.
					sort.Sort(eligibles)
					inputs = append(inputs, eligibles...)
				}
			}
		}
	}

	return inputs, nil
}

func compositeError(errString string, err error) error {
	return errors.New(errString + ": " + err.Error())
}

// AddrToUtxosMap converts a slice of credits to a map from the string
// representation of an encoded address to the unspent outputs
// associated with that address.
func AddrToUtxosMap(utxos []txstore.Credit, net *btcnet.Params) (map[string][]txstore.Credit, error) {
	addrMap := make(map[string][]txstore.Credit)
	for _, o := range utxos {
		_, addrs, _, err := o.Addresses(net)
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			encAddr := addr.EncodeAddress()
			if v, ok := addrMap[encAddr]; ok {
				addrMap[encAddr] = append(v, o)
			} else {
				addrMap[encAddr] = []txstore.Credit{o}
			}
		}
	}

	return addrMap, nil
}

// Eligible tests a given credit for eligibilty with respect to number
// of confirmations, the dust threshold and that it is not the charter
// output.
func Eligible(c txstore.Credit, minConf int, currentBlockHeight int32, dustThreshold btcutil.Amount) bool {
	if c.Amount() < dustThreshold {
		return false
	}
	if !c.Confirmed(minConf, currentBlockHeight) {
		return false
	}
	if isCharterOutput(c) {
		return false
	}

	return true
}

// isCharterInput - TODO: In order to determine this, we need the txid
// and the output index of the current charter output
func isCharterOutput(c txstore.Credit) bool {
	return false
}
