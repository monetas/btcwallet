package votingpool_test

import (
	"fmt"
	"testing"
)

// TestCreateEligibles basically serves as an example of how you can
// use the functions in helpers_test to create eligible inputs.
func TestCreateEligibles(t *testing.T) {
	teardown, mgr, pool := setUp(t)
	store, storeTearDown := createTxStore(t)
	defer teardown()
	defer storeTearDown()

	var version uint32 = 1
	var series, branch, index uint32 = 0, 0, 0
	var reqSigs uint32 = 3

	pubKeys := []string{pubKey1, pubKey2, pubKey3, pubKey4, pubKey5}
	if err := pool.CreateSeries(version, series, reqSigs, pubKeys); err != nil {
		t.Fatalf("Cannot creates series %v", series)
	}

	pkScript := createVotingPoolPkScript(t, mgr, pool, series, branch, index)
	eligible := createInputs(t, pkScript, []int64{5e6, 6e7})
	for _, e := range eligible {
		fmt.Println("e.Amount(): ", e.Amount())
	}
}
