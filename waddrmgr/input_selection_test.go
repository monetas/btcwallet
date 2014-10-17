package waddrmgr_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/conformal/btcutil"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

var (
	version uint32 = 1
	minConf int    = 100

	// random small number of satoshis used as dustThreshold
	dustThreshold btcutil.Amount = 1e4
)

// A test version of credit implementing the CreditInterface.
type FakeTxIDCredit struct {
	addr        waddrmgr.VotingPoolAddress
	txid        *btcwire.ShaHash
	outputIndex uint32
}

func newFakeTxIDCredit(series, index, branch int, txid []byte, outputIdx int) FakeTxIDCredit {
	var hash btcwire.ShaHash
	copy(hash[:], txid)
	return FakeTxIDCredit{
		addr: waddrmgr.VotingPoolAddress{
			SeriesID: uint32(series),
			Index:    uint32(index),
			Branch:   uint32(branch),
		},
		txid:        &hash,
		outputIndex: uint32(outputIdx),
	}
}

func (c FakeTxIDCredit) TxID() *btcwire.ShaHash {
	return c.txid
}

func (c FakeTxIDCredit) OutputIndex() uint32 {
	return c.outputIndex
}

func (c FakeTxIDCredit) Address() waddrmgr.VotingPoolAddress {
	return c.addr
}

// Compile time check that FakeTxIDCredit implements the
// CreditInterface.
var _ waddrmgr.CreditInterface = (*FakeTxIDCredit)(nil)

// TestCreditInterfaceSort checks that the sorting algorithm correctly
// sorts lexicographically by series, index, branch, txid,
// outputindex.
func TestCreditInterfaceSort(t *testing.T) {
	c0 := newFakeTxIDCredit(0, 0, 0, []byte{0x00, 0x00}, 0)
	c1 := newFakeTxIDCredit(0, 0, 0, []byte{0x00, 0x00}, 1)
	c2 := newFakeTxIDCredit(0, 0, 0, []byte{0x00, 0x01}, 0)
	c3 := newFakeTxIDCredit(0, 0, 0, []byte{0x01, 0x00}, 0)
	c4 := newFakeTxIDCredit(0, 0, 1, []byte{0x00, 0x00}, 0)
	c5 := newFakeTxIDCredit(0, 1, 0, []byte{0x00, 0x00}, 0)
	c6 := newFakeTxIDCredit(1, 0, 0, []byte{0x00, 0x00}, 0)

	randomCredits := []waddrmgr.VotingPoolCredits{
		waddrmgr.VotingPoolCredits{c6, c5, c4, c3, c2, c1, c0},
		waddrmgr.VotingPoolCredits{c2, c1, c0, c6, c5, c4, c3},
		waddrmgr.VotingPoolCredits{c6, c4, c5, c2, c3, c0, c1},
	}

	want := waddrmgr.VotingPoolCredits{c0, c1, c2, c3, c4, c5, c6}

	for _, random := range randomCredits {
		sort.Sort(random)
		got := random

		if len(got) != len(want) {
			t.Fatalf("Sorted credit slice size wrong: Got: %d, want: %d",
				len(got), len(want))
		}

		for idx := 0; idx < len(want); idx++ {
			if !reflect.DeepEqual(got[idx], want[idx]) {
				t.Errorf("Wrong output index. Got: %v, want: %v",
					got[idx], want[idx])
			}
		}
	}
}

func checkUniqueness(t *testing.T, credits waddrmgr.VotingPoolCredits) {
	type uniq struct {
		series      uint32
		branch      uint32
		index       uint32
		hash        btcwire.ShaHash
		outputIndex uint32
	}

	uniqMap := make(map[uniq]bool)
	for _, c := range credits {
		u := uniq{
			series:      c.Address().SeriesID,
			branch:      c.Address().Branch,
			index:       c.Address().Index,
			hash:        *c.TxID(),
			outputIndex: c.OutputIndex(),
		}
		if _, exists := uniqMap[u]; exists {
			t.Fatalf("Duplicate found: %v", u)
		} else {
			uniqMap[u] = true
		}
	}
}

func TestInputSelectionOneSeriesOnly(t *testing.T) {
	teardown, mgr, pool := setUp(t)
	defer teardown()
	// create some eligible inputs in a specified range.
	start := waddrmgr.VotingPoolAddress{
		SeriesID: 0,
		Branch:   0,
		Index:    0,
	}
	stop := waddrmgr.VotingPoolAddress{
		SeriesID: 0,
		Branch:   2,
		Index:    4,
	}
	var reqSigs uint32 = 2
	blockHeight := 11112
	currentBlockHeight := blockHeight + minConf + 10
	pubKeys := []string{pubKey1, pubKey2, pubKey3}
	store := txstore.New("/tmp/tx.bin")
	eligibleAmounts := []int64{int64(dustThreshold + 1), int64(dustThreshold + 1)}

	createSeries(t, pool, []seriesDef{{reqSigs, pubKeys, start.SeriesID}})

	// create expNoAddrs number of scripts.
	expNoAddrs, err := start.Distance(stop)
	if err != nil {
		t.Fatal("Calculating the distance failed")
	}
	scripts := createPkScripts(t, mgr, pool, start, stop)
	if uint64(len(scripts)) != expNoAddrs {
		t.Fatalf("Wrong number of scripts generated. Got: %d, want: %d",
			len(scripts), expNoAddrs)
	}

	// Now we have expNoAddrs number of scripts, let's make two
	// eligible inputs pr. script/address.
	expNoEligibleInputs := 2 * expNoAddrs
	var inputs []txstore.Credit
	for i := uint64(0); i < expNoAddrs; i++ {
		blockIndex := int(i) + 1
		created := createInputsStore(t, store, blockIndex, blockHeight,
			scripts[i], eligibleAmounts)
		inputs = append(inputs, created...)
	}

	// Call InputSelection on that range.
	eligibles, err := waddrmgr.InputSelection(
		store, pool, start, stop, dustThreshold, int32(currentBlockHeight), minConf)
	if err != nil {
		t.Fatal("InputSelection failed:", err)
	}

	if uint64(len(eligibles)) != expNoEligibleInputs {
		t.Fatalf("Wrong number of eligible inputs returned. Got: %d, want: %d.",
			len(eligibles), expNoEligibleInputs)
	}

	// Check that the returned eligibles have the proper sort order.
	if !sort.IsSorted(eligibles) {
		t.Fatal("Eligible inputs are not sorted.")
	}

	// Check that all credits are unique
	checkUniqueness(t, eligibles)
}

func TestEligibleInputsAreEligible(t *testing.T) {
	teardown, mgr, pool := setUp(t)
	defer teardown()
	var series, branch, index uint32 = 0, 0, 0
	var reqSigs uint32 = 3
	pubKeys := []string{pubKey1, pubKey2, pubKey3, pubKey4, pubKey5}
	if err := pool.CreateSeries(version, series, reqSigs, pubKeys); err != nil {
		t.Fatalf("Cannot creates series %v", series)
	}

	pkScript := createVotingPoolPkScript(t, mgr, pool, series, branch, index)

	var chainHeight int32 = 1000
	c := createInputs(t, pkScript, []int64{int64(dustThreshold)})[0]
	c.BlockHeight = int32(100)

	if !waddrmgr.Eligible(c, minConf, chainHeight, dustThreshold) {
		t.Errorf("Input is not eligible and it should be.")
	}
}

func TestNonEligibleInputsAreNotEligible(t *testing.T) {
	teardown, mgr, pool := setUp(t)
	defer teardown()
	var series, branch, index uint32 = 0, 0, 0
	var reqSigs uint32 = 3
	pubKeys := []string{pubKey1, pubKey2, pubKey3, pubKey4, pubKey5}
	if err := pool.CreateSeries(version, series, reqSigs, pubKeys); err != nil {
		t.Fatalf("Cannot creates series %v", series)
	}
	pkScript := createVotingPoolPkScript(t, mgr, pool, series, branch, index)
	var currentBlockHeight int32 = 1000

	c1 := createInputs(t, pkScript, []int64{int64(dustThreshold - 1)})[0]
	c1.BlockHeight = int32(100)
	if waddrmgr.Eligible(c1, minConf, currentBlockHeight, dustThreshold) {
		t.Errorf("Input is eligible and it should not be.")
	}

	c2 := createInputs(t, pkScript, []int64{int64(dustThreshold)})[0]
	// the calculation of if it has been confirmed does this:
	// chainheigt - bh + 1 >= target, which is quite weird, but the
	// reason why I need to put 902 as *that* makes 1000 - 902 +1 = 99 >=
	// 100 false
	c2.BlockHeight = int32(902)
	if waddrmgr.Eligible(c2, minConf, currentBlockHeight, dustThreshold) {
		t.Errorf("Input is eligible and it should not be.")
	}

}

func TestDistance(t *testing.T) {
	zero := waddrmgr.VotingPoolAddress{
		SeriesID: 0,
		Branch:   0,
		Index:    0,
	}

	two := waddrmgr.VotingPoolAddress{
		SeriesID: 0,
		Branch:   0,
		Index:    1,
	}

	four := waddrmgr.VotingPoolAddress{
		SeriesID: 0,
		Branch:   1,
		Index:    1,
	}

	eight := waddrmgr.VotingPoolAddress{
		SeriesID: 1,
		Branch:   1,
		Index:    1,
	}

	got, err := zero.Distance(zero)
	if err != nil {
		t.Fatalf("Distance failed: %v", err)
	}
	exp := uint64(1)
	if got != exp {
		t.Fatalf("Wrong distance. Got %d, want: %d", got, exp)
	}
	got, err = zero.Distance(two)
	if err != nil {
		t.Fatalf("Distance failed: %v", err)
	}
	exp = 2
	if got != exp {
		t.Fatalf("Wrong distance. Got %d, want: %d", got, exp)
	}
	got, err = zero.Distance(four)
	if err != nil {
		t.Fatalf("Distance failed: %v", err)
	}
	exp = 4
	if got != exp {
		t.Fatalf("Wrong distance. Got %d, want: %d", got, exp)
	}
	got, err = zero.Distance(eight)
	if err != nil {
		t.Fatalf("Distance failed: %v", err)
	}
	exp = 8
	if got != exp {
		t.Fatalf("Wrong distance. Got %d, want: %d", got, exp)
	}

	// Finally test that we get an error if arguments a,b are passed
	// where "a > b"
	got, err = four.Distance(zero)
	if err == nil {
		t.Fatalf("Distance should have failed, but did not")
	}
}
