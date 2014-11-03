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
		return 0, errors.New("Distance not defined when a.SeriesID > b.SeriesID")
	}
	if a.Branch > b.Branch {
		// TODO: define a proper error message
		return 0, errors.New("Distance not defined when a.Branch > b.Branch")
	}
	if a.Index > b.Index {
		// TODO: define a proper error message
		return 0, errors.New("Distance not defined when a.Index > b.Index")
	}

	return uint64((b.SeriesID - a.SeriesID + 1)) *
		uint64((b.Branch - a.Branch + 1)) *
		uint64((b.Index - a.Index + 1)), nil
}

type VotingPoolAddress struct {
	SeriesID uint32
	Index    uint32
	Branch   uint32
}

type CreditInterface interface {
	TxID() *btcwire.ShaHash
	OutputIndex() uint32
	Address() VotingPoolAddress
}

type VotingPoolCredit struct {
	Addr   VotingPoolAddress
	Credit txstore.Credit
}

func (c VotingPoolCredit) TxID() *btcwire.ShaHash {
	return c.Credit.TxRecord.Tx().Sha()
}

func (c VotingPoolCredit) OutputIndex() uint32 {
	return c.Credit.OutputIndex
}

func (c VotingPoolCredit) Address() VotingPoolAddress {
	return c.Addr
}

func newCredit(credit txstore.Credit, seriesID, branch, index uint32) VotingPoolCredit {
	return VotingPoolCredit{
		Credit: credit,
		Addr: VotingPoolAddress{
			SeriesID: seriesID,
			Branch:   branch,
			Index:    index,
		},
	}
}

type VotingPoolCredits []CreditInterface

func (c VotingPoolCredits) Len() int {
	return len(c)
}

func (c VotingPoolCredits) Less(i, j int) bool {
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

func (c VotingPoolCredits) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

var _ sort.Interface = (*VotingPoolCredits)(nil)

func InputSelection(store *txstore.Store, vp *VotingPool,
	start, stop VotingPoolAddress,
	dustThreshold btcutil.Amount, chainHeight int32,
	minConf int) (VotingPoolCredits, error) {
	unspents, err := store.UnspentOutputs()
	if err != nil {
		// TODO: consider if we need to create a new error.
		return nil, err
	}

	addrMap, err := UtxosToAddrMap(unspents, vp.manager.Net())
	if err != nil {
		// TODO: consider if we need to create a new error.
		return nil, err
	}
	var inputs VotingPoolCredits
	for series := start.SeriesID; series <= stop.SeriesID; series++ {
		for index := start.Index; index <= stop.Index; index++ {
			for branch := start.Branch; branch <= stop.Branch; branch++ {
				addr, err := vp.DepositScriptAddress(series, branch, index)
				if err != nil {
					// TODO: should we just try and skip this input?
					// Also consider if we need to create a new error.
					return nil, err
				}
				encAddr := addr.EncodeAddress()

				if candidates, ok := addrMap[encAddr]; ok {
					var eligibles VotingPoolCredits
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

// UtxosToAddrMap converts a slice of credits to a map from the string
// representation of an encoded address to the unspent outputs
// associated with that address.
func UtxosToAddrMap(utxos []txstore.Credit, net *btcnet.Params) (map[string][]txstore.Credit, error) {
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
