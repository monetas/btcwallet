package waddrmgr_test

import (
	"fmt"
	"testing"
)

// TestCreateEligibles basically serves as an example of how you can
// use the functions in helpers_test to create eligible inputs.
func TestCreateEligibles(t *testing.T) {
	teardown, mgr, pool := setUp(t)
	defer teardown()

	var version uint32 = 1
	var series, branch, index uint32 = 0, 0, 0
	var reqSigs uint32 = 3

	pubKeys := []string{pubKey1, pubKey2, pubKey3, pubKey4, pubKey5}
	if err := pool.CreateSeries(version, series, reqSigs, pubKeys); err != nil {
		t.Fatalf("Cannot creates series %v", series)
	}

	var bsHeight int32 = 11112
	pkScript := createVotingPoolPkScript(t, mgr, pool, bsHeight, series, branch, index)
	eligible := createInputs(t, pkScript, []int64{5e6, 6e7})
	for _, e := range eligible {
		fmt.Println("e.Amount(): ", e.Amount())
	}
}
