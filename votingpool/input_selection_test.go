package votingpool_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/conformal/btcutil"
	"github.com/conformal/btcwallet/txstore"
	vp "github.com/conformal/btcwallet/votingpool"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

var (
	minConf int = 100

	// random small number of satoshis used as dustThreshold
	dustThreshold btcutil.Amount = 1e4
)

// A test version of credit implementing the CreditInterface.
type FakeTxShaCredit struct {
	addr        vp.WithdrawalAddress
	txid        *btcwire.ShaHash
	outputIndex uint32
}

func newFakeTxShaCredit(t *testing.T, pool *vp.Pool, series, index vp.Index, branch vp.Branch, txid []byte, outputIdx int) FakeTxShaCredit {
	var hash btcwire.ShaHash
	copy(hash[:], txid)
	addr, err := pool.WithdrawalAddress(uint32(series), branch, index)
	if err != nil {
		t.Fatalf("WithdrawalAddress failed: %v", err)
	}
	return FakeTxShaCredit{
		addr:        *addr,
		txid:        &hash,
		outputIndex: uint32(outputIdx),
	}
}

func (c FakeTxShaCredit) TxSha() *btcwire.ShaHash {
	return c.txid
}

func (c FakeTxShaCredit) OutputIndex() uint32 {
	return c.outputIndex
}

func (c FakeTxShaCredit) Address() vp.WithdrawalAddress {
	return c.addr
}

func (c FakeTxShaCredit) Amount() btcutil.Amount {
	return btcutil.Amount(0)
}

func (c FakeTxShaCredit) TxOut() *btcwire.TxOut {
	return nil
}

func (c FakeTxShaCredit) OutPoint() *btcwire.OutPoint {
	return &btcwire.OutPoint{Hash: *c.txid, Index: c.outputIndex}
}

// Compile time check that FakeTxShaCredit implements the
// CreditInterface.
var _ vp.CreditInterface = (*FakeTxShaCredit)(nil)

// TestCreditInterfaceSort checks that the sorting algorithm correctly
// sorts lexicographically by series, index, branch, txid,
// outputindex.
func TestCreditInterfaceSort(t *testing.T) {
	teardown, _, pool := vp.TstSetUp(t)
	defer teardown()

	// Create the series 0 and 1 as they are needed for creaing the
	// fake credits.
	series := []vp.TstSeriesDef{
		{2, []string{pubKey1, pubKey2, pubKey3}, 0},
		{2, []string{pubKey3, pubKey4, pubKey5}, 1},
	}
	vp.TstCreateSeries(t, pool, series)

	c0 := newFakeTxShaCredit(t, pool, 0, 0, 0, []byte{0x00, 0x00}, 0)
	c1 := newFakeTxShaCredit(t, pool, 0, 0, 0, []byte{0x00, 0x00}, 1)
	c2 := newFakeTxShaCredit(t, pool, 0, 0, 0, []byte{0x00, 0x01}, 0)
	c3 := newFakeTxShaCredit(t, pool, 0, 0, 0, []byte{0x01, 0x00}, 0)
	c4 := newFakeTxShaCredit(t, pool, 0, 0, 1, []byte{0x00, 0x00}, 0)
	c5 := newFakeTxShaCredit(t, pool, 0, 1, 0, []byte{0x00, 0x00}, 0)
	c6 := newFakeTxShaCredit(t, pool, 1, 0, 0, []byte{0x00, 0x00}, 0)

	randomCredits := []vp.Credits{
		vp.Credits{c6, c5, c4, c3, c2, c1, c0},
		vp.Credits{c2, c1, c0, c6, c5, c4, c3},
		vp.Credits{c6, c4, c5, c2, c3, c0, c1},
	}

	want := vp.Credits{c0, c1, c2, c3, c4, c5, c6}

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

func checkUniqueness(t *testing.T, credits vp.Credits) {
	type uniq struct {
		series      uint32
		branch      vp.Branch
		index       vp.Index
		hash        btcwire.ShaHash
		outputIndex uint32
	}

	uniqMap := make(map[uniq]bool)
	for _, c := range credits {
		u := uniq{
			series:      c.Address().SeriesID(),
			branch:      c.Address().Branch(),
			index:       c.Address().Index(),
			hash:        *c.TxSha(),
			outputIndex: c.OutputIndex(),
		}
		if _, exists := uniqMap[u]; exists {
			t.Fatalf("Duplicate found: %v", u)
		} else {
			uniqMap[u] = true
		}
	}
}

func createScripts(t *testing.T, mgr *waddrmgr.Manager, pool *vp.Pool, ranges []vp.AddressRange) [][]byte {
	var scripts [][]byte
	for _, r := range ranges {
		// create expNoAddrs number of scripts.
		expNoAddrs, err := r.NumAddresses()
		if err != nil {
			t.Fatal("Calculating the range failed")
		}
		newScripts := vp.TstCreatePkScripts(t, pool, r)
		if uint64(len(newScripts)) != expNoAddrs {
			t.Fatalf("Wrong number of scripts generated. Got: %d, want: %d",
				len(scripts), expNoAddrs)
		}
		scripts = append(scripts, newScripts...)
	}
	return scripts
}

func TestGetEligibleInputs(t *testing.T) {
	teardown, mgr, pool := vp.TstSetUp(t)
	store, storeTearDown := vp.TstCreateTxStore(t)
	defer teardown()
	defer storeTearDown()

	// create some eligible inputs in a specified range.
	aRanges := []vp.AddressRange{
		{
			SeriesID:    0,
			StartBranch: 0,
			StopBranch:  3,
			StartIndex:  0,
			StopIndex:   4,
		},
		{
			SeriesID:    1,
			StartBranch: 0,
			StopBranch:  3,
			StartIndex:  0,
			StopIndex:   6,
		},
	}
	// define two series.
	series := []vp.TstSeriesDef{
		{2, []string{pubKey1, pubKey2, pubKey3}, aRanges[0].SeriesID},
		{2, []string{pubKey3, pubKey4, pubKey5}, aRanges[1].SeriesID},
	}
	oldChainHeight := 11112
	chainHeight := oldChainHeight + minConf + 10

	// create the series.
	vp.TstCreateSeries(t, pool, series)

	// create all the scripts.
	scripts := createScripts(t, mgr, pool, aRanges)

	// let's make two eligible inputs pr. script/address.
	expNoEligibleInputs := 2 * len(scripts)
	eligibleAmounts := []int64{int64(dustThreshold + 1), int64(dustThreshold + 1)}
	var inputs []txstore.Credit
	for i := 0; i < len(scripts); i++ {
		blockIndex := int(i) + 1
		created := vp.TstCreateInputsOnBlock(t, store, blockIndex, oldChainHeight,
			scripts[i], eligibleAmounts)
		inputs = append(inputs, created...)
	}

	// Call InputSelection on the range.
	eligibles, err := pool.TstGetEligibleInputs(
		store, aRanges, dustThreshold, int32(chainHeight), minConf)
	if err != nil {
		t.Fatal("InputSelection failed:", err)
	}

	// Check we got the expected number of eligible inputs.
	if len(eligibles) != expNoEligibleInputs {
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

func TestGetEligibleInputsFromSeries(t *testing.T) {
	teardown, mgr, pool := vp.TstSetUp(t)
	defer teardown()
	// create some eligible inputs in a specified range.
	aRange := vp.AddressRange{
		SeriesID:    0,
		StartBranch: 0,
		StopBranch:  2,
		StartIndex:  0,
		StopIndex:   4,
	}
	blockHeight := 11112
	currentChainHeight := blockHeight + minConf + 10
	store := txstore.New("/tmp/tx.bin")
	eligibleAmounts := []int64{int64(dustThreshold + 1), int64(dustThreshold + 1)}

	// define a series.
	series := []vp.TstSeriesDef{
		{2, []string{pubKey1, pubKey2, pubKey3}, aRange.SeriesID},
	}
	vp.TstCreateSeries(t, pool, series)

	// create all the scripts.
	scripts := createScripts(t, mgr, pool, []vp.AddressRange{aRange})

	// Let's create two eligible inputs for each of the scripts.
	expNumberOfEligibleInputs := 2 * len(scripts)
	var inputs []txstore.Credit
	for i := 0; i < len(scripts); i++ {
		blockIndex := int(i) + 1
		created := vp.TstCreateInputsOnBlock(t, store, blockIndex, blockHeight,
			scripts[i], eligibleAmounts)
		inputs = append(inputs, created...)
	}

	// Call InputSelection on the range.
	eligibles, err := pool.TstGetEligibleInputsFromSeries(
		store, aRange, dustThreshold, int32(currentChainHeight), minConf)
	if err != nil {
		t.Fatal("InputSelection failed:", err)
	}

	// Check we got the expected number of eligible inputs.
	if len(eligibles) != expNumberOfEligibleInputs {
		t.Fatalf("Wrong number of eligible inputs returned. Got: %d, want: %d.",
			len(eligibles), expNumberOfEligibleInputs)
	}

	// Check that the returned eligibles have the proper sort order.
	if !sort.IsSorted(eligibles) {
		t.Fatal("Eligible inputs are not sorted.")
	}

	// Check that all credits are unique
	checkUniqueness(t, eligibles)
}

func TestEligibleInputsAreEligible(t *testing.T) {
	teardown, _, pool := vp.TstSetUp(t)
	store, storeTearDown := vp.TstCreateTxStore(t)
	defer teardown()
	defer storeTearDown()
	var seriesID uint32 = 0
	var branch vp.Branch = 0
	var index vp.Index = 0

	// create the series
	series := []vp.TstSeriesDef{
		{3, []string{pubKey1, pubKey2, pubKey3, pubKey4, pubKey5}, seriesID},
	}
	vp.TstCreateSeries(t, pool, series)

	// Create the input.
	pkScript := vp.TstCreatePkScript(t, pool, seriesID, branch, index)
	var chainHeight int32 = 1000
	c := vp.TstCreateInputs(t, store, pkScript, []int64{int64(dustThreshold)})[0]

	// Make sure credits is old enough to pass the minConf check.
	c.BlockHeight = int32(100)

	if !pool.TstIsCreditEligible(c, minConf, chainHeight, dustThreshold) {
		t.Errorf("Input is not eligible and it should be.")
	}
}

func TestNonEligibleInputsAreNotEligible(t *testing.T) {
	teardown, _, pool := vp.TstSetUp(t)
	store1, storeTearDown1 := vp.TstCreateTxStore(t)
	store2, storeTearDown2 := vp.TstCreateTxStore(t)
	defer teardown()
	defer storeTearDown1()
	defer storeTearDown2()
	var seriesID uint32 = 0
	var branch vp.Branch = 0
	var index vp.Index = 0

	// create the series
	series := []vp.TstSeriesDef{
		{3, []string{pubKey1, pubKey2, pubKey3, pubKey4, pubKey5}, seriesID},
	}
	vp.TstCreateSeries(t, pool, series)

	pkScript := vp.TstCreatePkScript(t, pool, seriesID, branch, index)
	var chainHeight int32 = 1000

	// Check that credit below dustThreshold is rejected.
	c1 := vp.TstCreateInputs(t, store1, pkScript, []int64{int64(dustThreshold - 1)})[0]
	c1.BlockHeight = int32(100) // make sure it has enough confirmations.
	if pool.TstIsCreditEligible(c1, minConf, chainHeight, dustThreshold) {
		t.Errorf("Input is eligible and it should not be.")
	}

	// Check that a credit with not enough confirmations is rejected.
	c2 := vp.TstCreateInputs(t, store2, pkScript, []int64{int64(dustThreshold)})[0]
	// the calculation of if it has been confirmed does this:
	// chainheigt - bh + 1 >= target, which is quite weird, but the
	// reason why I need to put 902 as *that* makes 1000 - 902 +1 = 99 >=
	// 100 false
	c2.BlockHeight = int32(902)
	if pool.TstIsCreditEligible(c2, minConf, chainHeight, dustThreshold) {
		t.Errorf("Input is eligible and it should not be.")
	}

}

func TestAddressRange(t *testing.T) {
	one := vp.AddressRange{
		SeriesID:    0,
		StartBranch: 0,
		StopBranch:  0,
		StartIndex:  0,
		StopIndex:   0,
	}
	two := vp.AddressRange{
		SeriesID:    0,
		StartBranch: 0,
		StopBranch:  0,
		StartIndex:  0,
		StopIndex:   1,
	}
	four := vp.AddressRange{
		SeriesID:    0,
		StartBranch: 0,
		StopBranch:  1,
		StartIndex:  0,
		StopIndex:   1,
	}

	invalidBranch := vp.AddressRange{
		StartBranch: 1,
		StopBranch:  0,
	}

	invalidIndex := vp.AddressRange{
		StartIndex: 1,
		StopIndex:  0,
	}

	got, err := one.NumAddresses()
	if err != nil {
		t.Fatalf("NumAddresses failed: %v", err)
	}
	exp := uint64(1)
	if got != exp {
		t.Fatalf("Wrong range. Got %d, want: %d", got, exp)
	}
	got, err = two.NumAddresses()
	if err != nil {
		t.Fatalf("NumAddresses failed: %v", err)
	}
	exp = 2
	if got != exp {
		t.Fatalf("Wrong range. Got %d, want: %d", got, exp)
	}
	got, err = four.NumAddresses()
	if err != nil {
		t.Fatalf("NumAddresses failed: %v", err)
	}
	exp = 4
	if got != exp {
		t.Fatalf("Wrong range. Got %d, want: %d", got, exp)
	}

	// Finally test invalid ranges
	got, err = invalidIndex.NumAddresses()
	if err == nil {
		t.Fatalf("Expected failure, but got nil")
	}
	got, err = invalidBranch.NumAddresses()
	if err == nil {
		t.Fatalf("Expected failure, but got nil")
	}
}
