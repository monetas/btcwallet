package votingpool_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/conformal/btcec"
	"github.com/conformal/btclog"
	"github.com/conformal/btcnet"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcutil/hdkeychain"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/votingpool"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

var bsHeight int32 = 11112
var bs *waddrmgr.BlockStamp = &waddrmgr.BlockStamp{Height: bsHeight}
var netParams *btcnet.Params = &btcnet.MainNetParams
var logger btclog.Logger

// XXX: The txstore should not be a global, obviously.
var store *txstore.Store

func init() {
	logger, _ = btclog.NewLoggerFromWriter(os.Stdout, btclog.DebugLvl)
}

func TestFulfilOutputs(t *testing.T) {
	teardown, mgr, pool := setUp(t)
	defer teardown()

	var credits []txstore.Credit
	credits, store = createCredits(t, mgr, pool, []int64{5e6, 4e6})
	getEligibleInputs = func(inputStart, inputStop VotingPoolAddress, dustThreshold uint32, bsHeight int32) []txstore.Credit {
		return credits
	}
	outBailment := &OutBailment{poolID: pool.ID, server: "foo", transaction: 1}
	outBailment2 := &OutBailment{poolID: pool.ID, server: "foo", transaction: 2}
	address := "1MirQ9bwyQcGVJPwKUgapu5ouK2E2Ey4gX"
	outputs := []*WithdrawalOutput{
		&WithdrawalOutput{outBailment: outBailment, address: address, amount: btcutil.Amount(4e6)},
		&WithdrawalOutput{outBailment: outBailment2, address: address, amount: btcutil.Amount(1e6)},
	}

	changeStart := VotingPoolAddress{pool: pool, seriesID: 0, branch: 1, index: 0}
	dustThreshold := uint32(1)
	eligible := getEligibleInputs(VotingPoolAddress{}, VotingPoolAddress{}, dustThreshold, bsHeight)
	w := NewWithdrawal(0, outputs, eligible, changeStart)

	if err := w.fulfilOutputs(); err != nil {
		t.Fatal(err)
	}

	if len(w.transactions) != 1 {
		t.Fatalf("Unexpected number of transactions; got %d, want %d", len(w.transactions), 1)
	}

	tx := w.transactions[0]
	if len(tx.TxOut) != 3 {
		t.Fatalf("Unexpected number of tx outputs; got %d, want %d", len(tx.TxOut), 3)
	}

	status := w.status
	if len(status.outputs) != 2 {
		t.Fatalf("Unexpected number of outputs in WithdrawalStatus; got %d, want %d",
			len(status.outputs), 2)
	}

	for _, outb := range []*OutBailment{outBailment, outBailment2} {
		withdrawalOutput, found := status.outputs[outb]
		if !found {
			t.Fatalf("No output found for OutBailment %v", outb)
		}
		checkWithdrawalOutput(t, withdrawalOutput, address, "success", 1)
	}

	// XXX: There should be a separate test that generates raw signatures and checks them.
	sigs, err := w.sign(mgr)
	if err != nil {
		t.Fatal(err)
	}
	txSigs := sigs[ntxid(tx)]
	if len(txSigs) != 2 {
		t.Fatalf("Unexpected number of signature lists; got %d, want %d", len(txSigs), 2)
	}
	txInSigs := txSigs[0]
	if len(txInSigs) != 3 {
		t.Fatalf("Unexpected number of raw signatures; got %d, want %d", len(txInSigs), 3)
	}

	// XXX: There should be a separate test to check that signing of tx inputs works.
	sha, _ := btcwire.NewShaHashFromStr(ntxid(tx))
	t2 := store.UnminedTx(sha).MsgTx()
	if err = SignMultiSigUTXO(mgr, t2, 0, txInSigs); err != nil {
		t.Fatal(err)
	}
	if err = SignMultiSigUTXO(mgr, t2, 1, txSigs[1]); err != nil {
		t.Fatal(err)
	}

	if err = validateSigScripts(tx); err != nil {
		t.Fatal(err)
	}
}

func checkWithdrawalOutput(t *testing.T, withdrawalOutput *WithdrawalOutput, address, status string, nOutpoints int) {
	if withdrawalOutput.address != address {
		t.Fatalf("Unexpected address; got %s, want %s", withdrawalOutput.address, address)
	}

	if withdrawalOutput.status != status {
		t.Fatalf("Unexpected status; got '%s', want '%s'", withdrawalOutput.status, status)
	}

	if len(withdrawalOutput.outpoints) != nOutpoints {
		t.Fatalf("Unexpected number of outpoints; got %d, want %d", len(withdrawalOutput.outpoints), nOutpoints)
	}
}

type VotingPoolAddress struct {
	pool     *votingpool.VotingPool
	seriesID uint32
	branch   uint32
	index    uint32
}

func (a *VotingPoolAddress) Address(netParams *btcnet.Params) (btcutil.Address, error) {
	return a.pool.DepositScriptAddress(a.seriesID, a.branch, a.index)
}

func (a *VotingPoolAddress) Next() VotingPoolAddress {
	// TODO:
	return *a
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

func (o *WithdrawalOutput) pkScript() ([]byte, error) {
	address, err := btcutil.DecodeAddress(o.address, netParams)
	if err != nil {
		return nil, err
	}
	return btcscript.PayToAddrScript(address)
}

type OutBailmentOutpoint struct {
	ntxid  string
	index  uint32
	amount btcutil.Amount
}

// A list of raw signatures (one for every pubkey in the multi-sig script)
// for a given transaction input. They should match the order of pubkeys in
// the script and an empty rawSig should be used when the private key for
// a pubkey is not known.
type TxInSignatures [][]rawSig

type rawSig []byte

type withdrawal struct {
	roundID        uint32
	transactions   []*btcwire.MsgTx
	pendingOutputs []*WithdrawalOutput
	currentOutputs []*WithdrawalOutput
	eligibleInputs []txstore.Credit
	// A map of ntxids to lists of txstore.Credit
	usedInputs map[string][]txstore.Credit
	// A list containing the Credits added as inputs to currentTx; needed so that we
	// can sign them later on.
	currentInputs []txstore.Credit
	status        WithdrawalStatus
	changeStart   VotingPoolAddress
	currentTx     *btcwire.MsgTx
	// Totals for the current transaction
	inputTotal  btcutil.Amount
	outputTotal btcutil.Amount
}

func NewWithdrawal(roundID uint32, outputs []*WithdrawalOutput, inputs []txstore.Credit, changeStart VotingPoolAddress) *withdrawal {
	return &withdrawal{
		roundID:        roundID,
		currentTx:      btcwire.NewMsgTx(),
		pendingOutputs: outputs,
		usedInputs:     make(map[string][]txstore.Credit),
		eligibleInputs: inputs,
		status:         WithdrawalStatus{outputs: make(map[*OutBailment]*WithdrawalOutput)},
		changeStart:    changeStart,
	}
}

// Add the given output to the current Tx.
func (w *withdrawal) addOutput(output *WithdrawalOutput, pkScript []byte) uint32 {
	w.currentTx.AddTxOut(btcwire.NewTxOut(int64(output.amount), pkScript))
	w.outputTotal += output.amount
	w.currentOutputs = append(w.currentOutputs, output)
	return uint32(len(w.currentTx.TxOut) - 1)
}

func (w *withdrawal) rollBackLastOutput() {
	// TODO: Remove output from w.currentTx.TxOut
	// TODO: Subtract its amount from w.outputTotal
}

func (w *withdrawal) currentTxTooBig() bool {
	// TODO: Implement me!
	return estimateSize(w.currentTx) > 1000
}

// If this returns it means we have added an output and the necessary inputs to fulfil that
// output plus the required fees. It also means the tx won't reach the size limit even
// after we add a change output and sign all inputs.
func (w *withdrawal) fulfilNextOutput() error {
	output := w.pendingOutputs[0]
	w.pendingOutputs = w.pendingOutputs[1:]

	w.status.outputs[output.outBailment] = output
	pkScript, err := output.pkScript()
	if err != nil {
		output.status = "invalid"
		return nil
	}
	outputIndex := w.addOutput(output, pkScript)
	logger.Infof("Added output sending %s to %s", output.amount, output.address)

	if w.currentTxTooBig() {
		// TODO: Roll back last added output, finalize w.currentTx and assign a new
		// tx to currentTx.
		panic("Oversize TX not yet implemented")
	}

	fee := calculateFee(w.currentTx)
	for w.inputTotal < w.outputTotal+fee {
		if len(w.eligibleInputs) == 0 {
			// TODO: Implement Split Output procedure
			panic("Split Output not yet implemented")
		}
		input := w.eligibleInputs[0]
		w.eligibleInputs = w.eligibleInputs[1:]
		w.currentTx.AddTxIn(btcwire.NewTxIn(input.OutPoint(), nil))
		logger.Infof("Added input with amount %v", input.Amount())
		w.currentInputs = append(w.currentInputs, input)
		w.inputTotal += input.Amount()
		fee = calculateFee(w.currentTx)

		if w.currentTxTooBig() {
			// TODO: Roll back last added output plus all inputs added to support it.
			if len(w.currentTx.TxOut) > 1 {
				w.finalizeCurrentTx()
				// TODO: Finalize w.currentTx and assign a new tx to currentTx.
			} else if len(w.currentTx.TxOut) == 1 {
				// TODO: Split last output in two, and continue the loop.
			}
			panic("Oversize TX not yet implemented")
		}
	}

	outpoint := OutBailmentOutpoint{index: outputIndex, amount: output.amount}
	output.addOutpoint(outpoint)
	output.status = "success"
	return nil
}

func (w *withdrawal) finalizeCurrentTx() {
	if len(w.currentTx.TxOut) == 0 {
		return
	}
	fee := calculateFee(w.currentTx)
	change := w.inputTotal - w.outputTotal - fee
	if change > 0 {
		addr, err := w.changeStart.Address(netParams)
		if err != nil {
			panic(err) // XXX: Really no idea what to do if we get an error here...
		}
		pkScript, err := btcscript.PayToAddrScript(addr)
		if err != nil {
			panic(err) // XXX: Really no idea what to do if we get an error here...
		}
		w.currentTx.AddTxOut(btcwire.NewTxOut(int64(change), pkScript))
		logger.Infof("Added change output with amount %v", change)
		w.changeStart = w.changeStart.Next()
	}

	w.usedInputs[ntxid(w.currentTx)] = w.currentInputs
	w.transactions = append(w.transactions, w.currentTx)
	w.status.fees += fee

	// TODO: Update the ntxid of all WithdrawalOutput entries fulfilled by this transaction

	w.currentTx = btcwire.NewMsgTx()
	w.currentOutputs = make([]*WithdrawalOutput, 0)
	w.currentInputs = make([]txstore.Credit, 0)
	w.inputTotal = btcutil.Amount(0)
	w.outputTotal = btcutil.Amount(0)
}

func (w *withdrawal) fulfilOutputs() error {
	// TODO: Drop outputs (in descending amount order) if the input total is smaller than output total

	if len(w.pendingOutputs) == 0 {
		return errors.New("We don't seem to have inputs to cover any of the requested outputs")
	}

	// TODO: Sort outputs by outBailmentID (hash(server ID, tx #))

	for len(w.pendingOutputs) > 0 {
		// XXX: fulfilNextOutput() should probably never return an error because
		// it can just set the status of a given output to failed.
		if err := w.fulfilNextOutput(); err != nil {
			return err
		}
	}

	w.finalizeCurrentTx()

	for _, tx := range w.transactions {
		w.updateStatusFor(tx)

		// XXX: It'd make more sense to do this only after we have the raw signatures
		// and everything else we need to fulfil the startwithdrawal request.
		txr, err := store.InsertTx(btcutil.NewTx(tx), nil)
		if err != nil {
			return err
		}
		if _, err = txr.AddDebits(); err != nil {
			return err
		}
		// XXX: Must only do this if the transaction has a change output.
		if _, err = txr.AddCredit(uint32(len(tx.TxOut)-1), true); err != nil {
			return err
		}
	}
	store.MarkDirty()
	if err := store.WriteIfDirty(); err != nil {
		return err
	}
	return nil
}

func (w *withdrawal) updateStatusFor(tx *btcwire.MsgTx) {
	// TODO
}

// XXX: This assumes that the voting pool deposit script was imported into waddrmgr
func getRedeemScript(mgr *waddrmgr.Manager, addr *btcutil.AddressScriptHash) ([]byte, error) {
	address, err := mgr.Address(addr)
	if err != nil {
		return nil, err
	}
	sa, ok := address.(waddrmgr.ManagedScriptAddress)
	if !ok {
		return nil, errors.New("address is not a script address")
	}
	return sa.Script()
}

func getPrivKey(mgr *waddrmgr.Manager, addr *btcutil.AddressPubKey) (*btcec.PrivateKey, error) {
	address, err := mgr.Address(addr.AddressPubKeyHash())
	if err != nil {
		logger.Errorf("Address not found: %v", addr.AddressPubKeyHash())
		return nil, err
	}

	pka, ok := address.(waddrmgr.ManagedPubKeyAddress)
	if !ok {
		return nil, errors.New("address is not a pubkey address")
	}
	return pka.PrivKey()
}

// ntxid returns a unique ID for the given transaction.
func ntxid(tx *btcwire.MsgTx) string {
	// According to https://blockchain.info/q, the ntxid is the "hash of the serialized
	// transaction with its input scripts blank". But since we store the tx with
	// blank SignatureScripts anyway, we can use tx.TxSha() as the ntxid, which makes
	// our lives easier as that is what the txstore uses to lookup transactions.
	// Ignore the error as TxSha() can't fail.
	sha, _ := tx.TxSha()
	return sha.String()
}

// sign() iterates over inputs in each transaction generated by this withdrawal,
// constructing the raw signature for them. It returns a map of ntxids to signature
// lists.
// TODO: Add a test that uses a fixed transaction and compares the well known signatures
// (including their order) against the list returned here.
func (w *withdrawal) sign(mgr *waddrmgr.Manager) (map[string]TxInSignatures, error) {
	sigs := make(map[string]TxInSignatures)
	for _, tx := range w.transactions {
		txSigs := make(TxInSignatures, len(tx.TxIn))
		ntxid := ntxid(tx)
		for idx := range tx.TxIn {
			pkScript := w.usedInputs[ntxid][idx].TxOut().PkScript
			class, addresses, _, err := btcscript.ExtractPkScriptAddrs(pkScript, netParams)
			if err != nil {
				panic(err) // XXX: Again, no idea what's the correct thing to do here.
			}
			if class != btcscript.ScriptHashTy {
				// Assume pkScript is a P2SH because all voting pool addresses are P2SH.
				return nil, errors.New(fmt.Sprintf("Unexpected pkScript class: %v", class))
			}
			redeemScript, err := getRedeemScript(mgr, addresses[0].(*btcutil.AddressScriptHash))
			if err != nil {
				return nil, err // XXX: Again, no idea what's the correct thing to do here.
			}
			// The order of the signatures in txInSigs must match the order of the corresponding
			// pubkeys in the redeem script, but ExtractPkScriptAddrs() returns the pubkeys in
			// the original order, so we don't need to do anything special here.
			_, addresses, _, err = btcscript.ExtractPkScriptAddrs(redeemScript, netParams)
			txInSigs := make([]rawSig, len(addresses))
			for addrIdx, addr := range addresses {
				var sig rawSig
				privKey, err := getPrivKey(mgr, addr.(*btcutil.AddressPubKey))
				if err == nil {
					logger.Infof("Signing input %d of tx %s with privkey of %s",
						idx, ntxid, addr)
					sig, err = btcscript.RawTxInSignature(
						tx, idx, redeemScript, btcscript.SigHashAll, privKey)
					if err != nil {
						panic(err) // XXX: Again, no idea what's the correct thing to do here.
					}
				} else {
					logger.Infof(
						"Not signing input %d of %s because private key for %s was "+
							"not found: %v", idx, ntxid, addr, err)
					sig = []byte{}
				}
				txInSigs[addrIdx] = sig
			}
			txSigs[idx] = txInSigs
		}
		sigs[ntxid] = txSigs
	}
	// TODO: Need to store the raw signatures somewhere. Not sure this would be the correct
	// place to do that, though.
	return sigs, nil
}

// SignMultiSigUTXO signs the P2SH UTXO with the given index by constructing a
// script containing all given signatures plus the redeem (multi-sig) script.
// The order of the signatures must match that of the public keys in the multi-sig
// script as OP_CHECKMULTISIG expects that.
func SignMultiSigUTXO(mgr *waddrmgr.Manager, tx *btcwire.MsgTx, idx int, sigs []rawSig) error {
	txOut, err := store.UnconfirmedSpent(tx.TxIn[idx].PreviousOutPoint)
	if err != nil {
		return err
	}
	class, addresses, _, err := btcscript.ExtractPkScriptAddrs(txOut.PkScript, netParams)
	if err != nil {
		panic(err) // XXX: Again, no idea what's the correct thing to do here.
	}
	if class != btcscript.ScriptHashTy {
		// XXX: Is it ok to assume class is always a P2SH here?
		return errors.New(fmt.Sprintf("Unexpected pkScript class: %v", class))
	}
	redeemScript, err := getRedeemScript(mgr, addresses[0].(*btcutil.AddressScriptHash))
	if err != nil {
		panic(err) // XXX: Again, no idea what's the correct thing to do here.
	}

	class, _, nRequired, err := btcscript.ExtractPkScriptAddrs(redeemScript, netParams)
	if err != nil {
		panic(err) // XXX: Again, no idea what's the correct thing to do here.
	}
	if class != btcscript.MultiSigTy {
		// XXX: Is it ok to assume class is always a multi-sig here?
		return errors.New(fmt.Sprintf("Unexpected redeemScript class: %v", class))
	}
	if len(sigs) < nRequired {
		return errors.New("Not enough signatures")
	}

	// Construct the unlocking script.
	// Start with an OP_0 because of the bug in bitcoind, then add nRequired signatures.
	unlockingScript := btcscript.NewScriptBuilder().AddOp(btcscript.OP_FALSE)
	for _, sig := range sigs[:nRequired] {
		unlockingScript.AddData(sig)
	}

	// Combine the redeem script and the unlocking script to get the actual signature script.
	sigScript := unlockingScript.AddData(redeemScript)
	tx.TxIn[idx].SignatureScript = sigScript.Script()
	return nil
}

// validateSigScripts executes the signature script of every input in the given transaction
// and returns an error if any of them fail.
func validateSigScripts(msgtx *btcwire.MsgTx) error {
	flags := btcscript.ScriptCanonicalSignatures | btcscript.ScriptStrictMultiSig | btcscript.ScriptBip16
	for i, txin := range msgtx.TxIn {
		txOut, err := store.UnconfirmedSpent(msgtx.TxIn[i].PreviousOutPoint)
		if err != nil {
			return err
		}
		engine, err := btcscript.NewScript(txin.SignatureScript, txOut.PkScript, i, msgtx, flags)
		if err != nil {
			return fmt.Errorf("cannot create script engine: %s", err)
		}
		if err = engine.Execute(); err != nil {
			return fmt.Errorf("cannot validate transaction: %s", err)
		}
	}
	return nil
}

func estimateSize(tx *btcwire.MsgTx) uint32 {
	// TODO: Implement me
	// This function could estimate the size given the number of inputs/outputs, similarly
	// to estimateTxSize() (in createtx.go), or it could copy the tx, add a stub change
	// output, fill the SignatureScript for every input and serialize it.
	return 0
}

func calculateFee(tx *btcwire.MsgTx) btcutil.Amount {
	// TODO
	return btcutil.Amount(1)
}

func getEligibleInputsDefault(inputStart, inputStop VotingPoolAddress, dustThreshold uint32,
	bsHeight int32) []txstore.Credit {
	// TODO:
	return make([]txstore.Credit, 0)
}

var getEligibleInputs = getEligibleInputsDefault

func createCredits(t *testing.T, mgr *waddrmgr.Manager, pool *votingpool.VotingPool, amounts []int64) ([]txstore.Credit, *txstore.Store) {
	// Create 3 master extended keys, as if we had 3 voting pool members.
	master1, _ := hdkeychain.NewMaster(seed)
	master2, _ := hdkeychain.NewMaster(append(seed, byte(0x01)))
	master3, _ := hdkeychain.NewMaster(append(seed, byte(0x02)))
	masters := []*hdkeychain.ExtendedKey{master1, master2, master3}
	rawPubKeys := make([]string, 3)
	for i, key := range masters {
		pubkey, _ := key.Neuter()
		rawPubKeys[i] = pubkey.String()
	}

	// Create a series with the master pubkeys of our voting pool members.
	reqSigs := uint32(2)
	seriesID := uint32(0)
	if err := pool.CreateSeries(1, seriesID, reqSigs, rawPubKeys); err != nil {
		t.Fatalf("Cannot creates series: %v", err)
	}

	idx := uint32(0)
	// Import the 0th child of our master keys into the address manager as we're going
	// to need them when signing the transactions later on.
	wifs := make([]string, 3)
	for i, master := range masters {
		child, _ := master.Child(idx)
		ecPrivKey, _ := child.ECPrivKey()
		wif, _ := btcutil.NewWIF(ecPrivKey, netParams, true)
		wifs[i] = wif.String()
	}
	importPrivateKeys(t, mgr, wifs, bs)

	// Finally create the Credit instances, locked to the voting pool's deposit
	// address with branch==0, index==0.
	branch := uint32(0)
	pkScript := createVotingPoolPkScript(t, mgr, pool, bsHeight, seriesID, branch, idx)
	return createInputs(t, pkScript, amounts)
}
