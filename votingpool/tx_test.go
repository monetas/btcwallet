package votingpool_test

import (
	"errors"
	"os"
	"testing"

	"github.com/conformal/btclog"
	"github.com/conformal/btcnet"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcutil/hdkeychain"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/votingpool"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
	"github.com/davecgh/go-spew/spew"
)

var bsHeight int32 = 11112
var bs *waddrmgr.BlockStamp = &waddrmgr.BlockStamp{Height: bsHeight}
var netParams *btcnet.Params = &btcnet.MainNetParams
var logger btclog.Logger

func init() {
	logger, _ = btclog.NewLoggerFromWriter(os.Stdout, btclog.DebugLvl)
}

func TestStartWithdrawal(t *testing.T) {
	teardown, mgr, pool := setUp(t)
	defer teardown()

	getEligibleInputs = func(inputStart, inputStop VotingPoolAddress, dustThreshold uint32, bsHeight int32) []txstore.Credit {
		key1, _ := hdkeychain.NewMaster(seed)
		key2, _ := key1.Child(0)
		key3, _ := key1.Child(1)
		pubKeys := make([]string, 3)
		for i, key := range []*hdkeychain.ExtendedKey{key1, key2, key3} {
			pubkey, _ := key.Neuter()
			pubKeys[i] = pubkey.String()
		}
		reqSigs := uint32(2)
		eligible, _ := createCredits(t, mgr, pool, []int64{5e6, 4e6}, pubKeys, reqSigs)
		return eligible
	}
	outputs := []*WithdrawalOutput{&WithdrawalOutput{
		outBailment: &OutBailment{poolID: []byte{0x00}, server: "foo", transaction: 1},
		address:     "1MirQ9bwyQcGVJPwKUgapu5ouK2E2Ey4gX",
		amount:      4e6},
	}

	_, _, err := startWithdrawal(0, VotingPoolAddress{}, VotingPoolAddress{}, VotingPoolAddress{}, 1, outputs)
	if err != nil {
		t.Fatal(err)
	}

	// TODO: Validate the transactions
}

type VotingPoolAddress struct {
	seriesID uint32
	branch   uint32
	index    uint32
}

type OutBailment struct {
	poolID      []byte
	server      string
	transaction uint32
}

type WithdrawalStatus struct {
	nextInputStart  VotingPoolAddress
	nextChangeStart VotingPoolAddress
	fees            btcutil.Amount
	outputs         map[*OutBailment]*WithdrawalOutput
}

type WithdrawalOutput struct {
	outBailment *OutBailment
	address     string
	amount      btcutil.Amount
	status      string
	outpoints   []OutBailmentOutpoint
}

func (o *WithdrawalOutput) addOutpoint(outpoint OutBailmentOutpoint) {
	o.outpoints = append(o.outpoints, outpoint)
}

type OutBailmentOutpoint struct {
	ntxid  string
	index  uint32
	amount btcutil.Amount
}

type TxInSignatures struct {
	// For every private key controlled by this wallet, there will be one list
	// containing one entry for every input in the transaction.
	sigs [][]string
}

type Withdrawal struct {
	roundID        uint32
	transactions   []*btcwire.MsgTx
	pendingOutputs []*WithdrawalOutput
	currentOutputs []*WithdrawalOutput
	eligibleInputs []txstore.Credit
	status         WithdrawalStatus
	changeStart    VotingPoolAddress
	currentTx      *btcwire.MsgTx
	// Totals for the current transaction
	inputTotal  btcutil.Amount
	outputTotal btcutil.Amount
}

func NewWithdrawal(roundID uint32, outputs []*WithdrawalOutput, inputs []txstore.Credit, changeStart VotingPoolAddress) *Withdrawal {
	return &Withdrawal{
		roundID:        roundID,
		currentTx:      btcwire.NewMsgTx(),
		pendingOutputs: outputs,
		currentOutputs: make([]*WithdrawalOutput, 10),
		eligibleInputs: inputs,
		status:         WithdrawalStatus{outputs: make(map[*OutBailment]*WithdrawalOutput)},
		changeStart:    changeStart,
	}
}

// Add the given output to the current Tx.
func (w *Withdrawal) addOutput(output *WithdrawalOutput, pkScript []byte) uint32 {
	w.currentTx.AddTxOut(btcwire.NewTxOut(int64(output.amount), pkScript))
	w.outputTotal += output.amount
	w.currentOutputs = append(w.currentOutputs, output)
	return uint32(len(w.currentTx.TxOut) - 1)
}

func (w *Withdrawal) rollBackLastOutput() {
	// TODO: Remove output from w.currentTx.TxOut
	// TODO: Subtract its amount from w.outputTotal
}

func (w *Withdrawal) currentTxTooBig() bool {
	// TODO: Implement me!
	return estimateSize(w.currentTx) > 1000
}

// If this returns it means we have added an output and the necessary inputs to fulfil that
// output plus the required fees. It also means the tx won't reach the size limit even
// after we add a change output and sign all inputs.
func (w *Withdrawal) fulfilNextOutput() error {
	output := w.pendingOutputs[0]
	w.pendingOutputs = w.pendingOutputs[1:]

	// XXX: Consider moving all this into w.addOutput()
	w.status.outputs[output.outBailment] = output
	address, err := btcutil.DecodeAddress(output.address, netParams)
	if err != nil {
		output.status = "invalid"
		return nil
	}
	pkScript, err := btcscript.PayToAddrScript(address)
	if err != nil {
		output.status = "invalid"
		return nil
	}
	outputIndex := w.addOutput(output, pkScript)
	logger.Infof("Added output sending %s to %s", output.amount, output.address)

	if w.currentTxTooBig() {
		// TODO: Roll back last added output, finalize w.currentTx and assign a new
		// tx to currentTx.
		return errors.New("Oversize TX not yet implemented")
	}

	feeEstimate := estimateFee(w.currentTx)
	for w.inputTotal < w.outputTotal+feeEstimate {
		if len(w.eligibleInputs) == 0 {
			// TODO: Implement Split Output procedure
			return errors.New("Split Output not yet implemented")
		}
		input := w.eligibleInputs[0]
		w.eligibleInputs = w.eligibleInputs[1:]
		w.currentTx.AddTxIn(btcwire.NewTxIn(input.OutPoint(), nil))
		w.inputTotal += input.Amount()
		feeEstimate = estimateFee(w.currentTx)
		logger.Infof("Added input containing %s", output.amount)

		if w.currentTxTooBig() {
			// TODO: Roll back last added output plus all inputs added to support it.
			if len(w.currentTx.TxOut) > 1 {
				w.finalizeCurrentTx()
				// TODO: Finalize w.currentTx and assign a new tx to currentTx.
			} else if len(w.currentTx.TxOut) == 1 {
				// TODO: Split last output in two, and continue the loop.
			}
			return errors.New("Oversize TX not yet implemented")
		}
	}

	outpoint := OutBailmentOutpoint{index: outputIndex, amount: output.amount}
	output.addOutpoint(outpoint)
	return nil
}

func (w *Withdrawal) finalizeCurrentTx() {
	// TODO: Calculate fee

	// TODO: Maybe add a change output, and increment w.changeStart if we do so

	// TODO: Append currentTx to w.transactions

	// TODO: Increment w.status.fees

	// TODO: Calculate the ntxid for this tx and update the ntxid of all WithdrawalOutput entries
	// fulfilled by this transaction
	// https://blockchain.info/q <-- info on how to generate ntxid

	w.currentTx = btcwire.NewMsgTx()
	w.currentOutputs = make([]*WithdrawalOutput, 10)
	w.inputTotal = btcutil.Amount(0)
	w.outputTotal = btcutil.Amount(0)
}

func (w *Withdrawal) fulfilOutputs() error {
	// TODO: Possibly drop outputs (in descending amount order) if the input total is smaller than output total

	// TODO: Check that there are > 0 outputs left, returning if it doesn't

	// TODO: Sort outputs by outBailmentID (hash(server ID, tx #))

	for len(w.pendingOutputs) > 0 {
		// XXX: fulfilNextOutput() should probably never return an error because
		// it can just set the status of a given output to failed.
		if err := w.fulfilNextOutput(); err != nil {
			return err
		}
	}

	w.transactions = append(w.transactions, w.currentTx)
	for _, tx := range w.transactions {
		if len(tx.TxOut) == 0 {
			// XXX: Is this the right thing to do? In which cases this can happen?
			continue
		}
		w.addChangeOutput(tx)

		w.updateStatusFor(tx)
	}

	// TODO: Iterate over outputs in every tx, adding a change when necessary and updating
	// their status in w.status

	spew.Dump(w.status)
	return nil
}

func (w *Withdrawal) addChangeOutput(tx *btcwire.MsgTx) {
	// TODO
}

func (w *Withdrawal) updateStatusFor(tx *btcwire.MsgTx) {
	// TODO
}

func (w *Withdrawal) sign() map[string]TxInSignatures {
	// TODO: Iterate over inputs in every tx and generate signatures for them
	// A map of ntxid to siglists.

	/*
		privKey1, err := key1.ECPrivKey()
		btcscript.SignUTXO(msgTx, 0, redeemScript, privKey1)
	*/

	return make(map[string]TxInSignatures)
}

func estimateSize(tx *btcwire.MsgTx) uint32 {
	// TODO: Implement me
	// This function could estimate the size given the number of inputs/outputs, similarly
	// to estimateTxSize() (in createtx.go), or it could copy the tx, add a stub change
	// output, fill the SignatureScript for every input and serialize it.
	// We'll also need to fill the SignatureScript (with zeroes) of every tx's txin before
	// we serialize.
	return 0
}

func estimateFee(tx *btcwire.MsgTx) btcutil.Amount {
	// TODO
	return btcutil.Amount(1)
}

func startWithdrawal(roundID uint32, inputStart, inputStop, changeStart VotingPoolAddress,
	dustThreshold uint32, outputs []*WithdrawalOutput) (*WithdrawalStatus, map[string]TxInSignatures, error) {

	eligible := getEligibleInputs(inputStart, inputStop, dustThreshold, bsHeight)
	w := NewWithdrawal(roundID, outputs, eligible, changeStart)

	if err := w.fulfilOutputs(); err != nil {
		return nil, nil, err
	}

	return &w.status, w.sign(), nil
}

// TODO: Must add an activeNet argument here as well.
func getEligibleInputsDefault(inputStart, inputStop VotingPoolAddress, dustThreshold uint32, bsHeight int32) []txstore.Credit {
	// TODO:
	return make([]txstore.Credit, 0)
}

var getEligibleInputs func(VotingPoolAddress, VotingPoolAddress, uint32, int32) []txstore.Credit = getEligibleInputsDefault

func createCredits(t *testing.T, mgr *waddrmgr.Manager, pool *votingpool.VotingPool,
	amounts []int64, pubKeys []string, reqSigs uint32) (credits []txstore.Credit, redeemScript []byte) {
	seriesID := uint32(0)
	if err := pool.CreateSeries(1, seriesID, reqSigs, pubKeys); err != nil {
		t.Fatalf("Cannot creates series: %v", err)
	}
	branch := uint32(0)
	idx := uint32(0)
	// XXX: This is not nice because the redeemScript is also generated inside
	// createVotingPoolPkScript, so maybe it should be returned from there.
	redeemScript, err := pool.DepositScript(seriesID, branch, idx)
	if err != nil {
		t.Fatal(err)
	}
	pkScript := createVotingPoolPkScript(t, mgr, pool, bsHeight, seriesID, branch, idx)
	return createInputs(t, pkScript, amounts), redeemScript
}
