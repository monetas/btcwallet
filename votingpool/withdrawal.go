/*
 * Copyright (c) 2014 Conformal Systems LLC <info@conformal.com>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package votingpool

import (
	"errors"
	"fmt"
	"os"

	"github.com/conformal/btcec"
	"github.com/conformal/btclog"
	"github.com/conformal/btcnet"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
)

type VotingPoolAddress struct {
	pool     *VotingPool
	seriesID uint32
	branch   uint32
	index    uint32
}

func NewVotingPoolAddress(
	pool *VotingPool, seriesID uint32, branch uint32, index uint32) *VotingPoolAddress {
	return &VotingPoolAddress{pool: pool, seriesID: seriesID, branch: branch, index: index}
}

func (a *VotingPoolAddress) Address() (btcutil.Address, error) {
	return a.pool.DepositScriptAddress(a.seriesID, a.branch, a.index)
}

func (a *VotingPoolAddress) Next() *VotingPoolAddress {
	// TODO:
	return a
}

type OutBailment struct {
	poolID      []byte
	server      string
	transaction uint32
}

func NewOutBailment(poolID []byte, server string, transaction uint32) *OutBailment {
	return &OutBailment{poolID: poolID, server: server, transaction: transaction}
}

type WithdrawalStatus struct {
	nextInputStart  VotingPoolAddress
	nextChangeStart VotingPoolAddress
	fees            btcutil.Amount
	outputs         map[*OutBailment]*WithdrawalOutput
}

func (s *WithdrawalStatus) Outputs() map[*OutBailment]*WithdrawalOutput {
	return s.outputs
}

type WithdrawalOutput struct {
	outBailment *OutBailment
	address     string
	amount      btcutil.Amount
	status      string
	outpoints   []OutBailmentOutpoint
}

func NewWithdrawalOutput(
	outBailment *OutBailment, address string, amount btcutil.Amount) *WithdrawalOutput {
	return &WithdrawalOutput{outBailment: outBailment, address: address, amount: amount}
}

func (o *WithdrawalOutput) addOutpoint(outpoint OutBailmentOutpoint) {
	o.outpoints = append(o.outpoints, outpoint)
}

func (o *WithdrawalOutput) pkScript(net *btcnet.Params) ([]byte, error) {
	address, err := btcutil.DecodeAddress(o.address, net)
	if err != nil {
		return nil, err
	}
	return btcscript.PayToAddrScript(address)
}

func (o *WithdrawalOutput) Status() string {
	return o.status
}

func (o *WithdrawalOutput) Address() string {
	return o.address
}

func (o *WithdrawalOutput) Outpoints() []OutBailmentOutpoint {
	return o.outpoints
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
	changeStart   *VotingPoolAddress
	currentTx     *btcwire.MsgTx
	net           *btcnet.Params
	// Totals for the current transaction
	inputTotal  btcutil.Amount
	outputTotal btcutil.Amount
	// XXX: This should probably not be here
	logger btclog.Logger
}

func NewWithdrawal(roundID uint32, outputs []*WithdrawalOutput, inputs []txstore.Credit, changeStart *VotingPoolAddress, net *btcnet.Params) *withdrawal {
	logger, _ := btclog.NewLoggerFromWriter(os.Stdout, btclog.DebugLvl)
	return &withdrawal{
		roundID:        roundID,
		currentTx:      btcwire.NewMsgTx(),
		pendingOutputs: outputs,
		usedInputs:     make(map[string][]txstore.Credit),
		eligibleInputs: inputs,
		status:         WithdrawalStatus{outputs: make(map[*OutBailment]*WithdrawalOutput)},
		changeStart:    changeStart,
		net:            net,
		logger:         logger,
	}
}

func (w *withdrawal) Transactions() []*btcwire.MsgTx {
	return w.transactions
}

func (w *withdrawal) Status() WithdrawalStatus {
	return w.status
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
	pkScript, err := output.pkScript(w.net)
	if err != nil {
		output.status = "invalid"
		return nil
	}
	outputIndex := w.addOutput(output, pkScript)
	w.logger.Infof("Added output sending %s to %s", output.amount, output.address)

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
		w.logger.Infof("Added input with amount %v", input.Amount())
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
		addr, err := w.changeStart.Address()
		if err != nil {
			panic(err) // XXX: Really no idea what to do if we get an error here...
		}
		pkScript, err := btcscript.PayToAddrScript(addr)
		if err != nil {
			panic(err) // XXX: Really no idea what to do if we get an error here...
		}
		w.currentTx.AddTxOut(btcwire.NewTxOut(int64(change), pkScript))
		w.logger.Infof("Added change output with amount %v", change)
		w.changeStart = w.changeStart.Next()
	}

	w.usedInputs[Ntxid(w.currentTx)] = w.currentInputs
	w.transactions = append(w.transactions, w.currentTx)
	w.status.fees += fee

	// TODO: Update the ntxid of all WithdrawalOutput entries fulfilled by this transaction

	w.currentTx = btcwire.NewMsgTx()
	w.currentOutputs = make([]*WithdrawalOutput, 0)
	w.currentInputs = make([]txstore.Credit, 0)
	w.inputTotal = btcutil.Amount(0)
	w.outputTotal = btcutil.Amount(0)
}

func (w *withdrawal) FulfilOutputs(store *txstore.Store) error {
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
		return nil, err
	}

	pka, ok := address.(waddrmgr.ManagedPubKeyAddress)
	if !ok {
		return nil, errors.New("address is not a pubkey address")
	}
	return pka.PrivKey()
}

// Ntxid returns a unique ID for the given transaction.
func Ntxid(tx *btcwire.MsgTx) string {
	// According to https://blockchain.info/q, the ntxid is the "hash of the serialized
	// transaction with its input scripts blank". But since we store the tx with
	// blank SignatureScripts anyway, we can use tx.TxSha() as the ntxid, which makes
	// our lives easier as that is what the txstore uses to lookup transactions.
	// Ignore the error as TxSha() can't fail.
	sha, _ := tx.TxSha()
	return sha.String()
}

// Sign iterates over inputs in each transaction generated by this withdrawal,
// constructing the raw signature for them. It returns a map of ntxids to signature
// lists.
// TODO: Add a test that uses a fixed transaction and compares the well known signatures
// (including their order) against the list returned here.
func (w *withdrawal) Sign(mgr *waddrmgr.Manager) (map[string]TxInSignatures, error) {
	sigs := make(map[string]TxInSignatures)
	for _, tx := range w.transactions {
		txSigs := make(TxInSignatures, len(tx.TxIn))
		ntxid := Ntxid(tx)
		for idx := range tx.TxIn {
			pkScript := w.usedInputs[ntxid][idx].TxOut().PkScript
			class, addresses, _, err := btcscript.ExtractPkScriptAddrs(pkScript, w.net)
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
			_, addresses, _, err = btcscript.ExtractPkScriptAddrs(redeemScript, w.net)
			txInSigs := make([]rawSig, len(addresses))
			for addrIdx, addr := range addresses {
				var sig rawSig
				privKey, err := getPrivKey(mgr, addr.(*btcutil.AddressPubKey))
				if err == nil {
					w.logger.Infof("Signing input %d of tx %s with privkey of %s",
						idx, ntxid, addr)
					sig, err = btcscript.RawTxInSignature(
						tx, idx, redeemScript, btcscript.SigHashAll, privKey)
					if err != nil {
						panic(err) // XXX: Again, no idea what's the correct thing to do here.
					}
				} else {
					w.logger.Infof(
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
func SignMultiSigUTXO(mgr *waddrmgr.Manager, tx *btcwire.MsgTx, idx int, pkScript []byte, sigs []rawSig, net *btcnet.Params) error {
	class, addresses, _, err := btcscript.ExtractPkScriptAddrs(pkScript, net)
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

	class, _, nRequired, err := btcscript.ExtractPkScriptAddrs(redeemScript, net)
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

// ValidateSigScripts executes the signature script of every input in the given transaction
// and returns an error if any of them fail.
func ValidateSigScripts(msgtx *btcwire.MsgTx, store *txstore.Store) error {
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
