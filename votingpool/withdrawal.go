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
	"fmt"
	"os"
	"sort"

	"github.com/conformal/btclog"
	"github.com/conformal/btcnet"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcutil/hdkeychain"
	"github.com/conformal/btcwallet/txstore"
	"github.com/conformal/btcwallet/waddrmgr"
	"github.com/conformal/btcwire"
	"github.com/conformal/fastsha256"
)

/*  ==== What needs to be stored in the DB, and other notes ====
(This is just a collection of notes about things we still need to do here)

== To be stored in the DB ==

Signature lists

All parameters of a startwithdrawal, to be able to return an error if we get two of them
with the same roundID but anything else different.

The whole WithdrawalStatus so that we can deal with multiple identical startwithdrawal
requests. We need to do that because the transactions created as part of a startwithdrawal
will mark outputs as spent and if we get a second identical startwithdrawal request we
won't be able construct the same transactions as we did in the first request.

== Other notes ==

Using separate DB Buckets for the transactions and the withdrawal registry (siglists, etc)
may be problematic.  We'll need to make sure both the transactions and the withdrawal
details are persisted atomically.

Since we're marking outputs as spent when we use them in transactions constructed in
startwithdrawal, we'll need a janitor process that eventually releases those if the
transactions are never confirmed.

*/
var log btclog.Logger

func init() {
	// XXX: Make it possible to switch this on/off like in txstore/log.go
	log, _ = btclog.NewLoggerFromWriter(os.Stdout, btclog.DebugLvl)
}

func (s *SeriesData) getPrivKeyFor(pubKey *hdkeychain.ExtendedKey) (*hdkeychain.ExtendedKey, error) {
	for i, key := range s.publicKeys {
		if key.String() == pubKey.String() {
			return s.privateKeys[i], nil
		}
	}
	return nil, newError(
		ErrUnknownPubKey, fmt.Sprintf("unknown public key '%s'", pubKey.String()), nil)
}

func (vp *Pool) ChangeAddress(seriesID uint32, index Index) (*ChangeAddress, error) {
	// TODO: Ensure the given series is active.
	// Branch is always 0 for change addresses.
	vpAddr, err := vp.newVotingPoolAddress(seriesID, Branch(0), index)
	if err != nil {
		return nil, err
	}
	return &ChangeAddress{votingPoolAddress: vpAddr}, nil
}

func (vp *Pool) WithdrawalAddress(seriesID uint32, branch Branch, index Index) (*WithdrawalAddress, error) {
	// TODO: Ensure the given series is hot.
	vpAddr, err := vp.newVotingPoolAddress(seriesID, branch, index)
	if err != nil {
		return nil, err
	}
	return &WithdrawalAddress{votingPoolAddress: vpAddr}, nil
}

type votingPoolAddress struct {
	pool     *Pool
	addr     btcutil.Address
	script   []byte
	seriesID uint32
	branch   Branch
	index    Index
}

func (p *Pool) newVotingPoolAddress(seriesID uint32, branch Branch, index Index) (*votingPoolAddress, error) {
	script, err := p.DepositScript(seriesID, branch, index)
	if err != nil {
		return nil, err
	}
	addr, err := p.addressFor(script)
	if err != nil {
		return nil, err
	}
	return &votingPoolAddress{
			pool: p, seriesID: seriesID, branch: branch, index: index, addr: addr, script: script},
		nil
}

// String returns a string encoding of the underlying bitcoin payment address.
func (a *votingPoolAddress) String() string {
	return a.Addr().EncodeAddress()
}

func (a *votingPoolAddress) Addr() btcutil.Address {
	return a.addr
}

func (a *votingPoolAddress) RedeemScript() []byte {
	return a.script
}

func (a *votingPoolAddress) Series() *SeriesData {
	return a.pool.GetSeries(a.seriesID)
}

func (a *votingPoolAddress) SeriesID() uint32 {
	return a.seriesID
}

func (a *votingPoolAddress) Branch() Branch {
	return a.branch
}

func (a *votingPoolAddress) Index() Index {
	return a.index
}

type ChangeAddress struct {
	*votingPoolAddress
}

func (a *ChangeAddress) Next() (*ChangeAddress, error) {
	return a.pool.ChangeAddress(a.seriesID, a.index+1)
}

type WithdrawalAddress struct {
	*votingPoolAddress
}

type WithdrawalStatus struct {
	nextInputStart  WithdrawalAddress
	nextChangeStart ChangeAddress
	fees            btcutil.Amount
	outputs         []*WithdrawalOutput
}

func (s *WithdrawalStatus) Outputs() []*WithdrawalOutput {
	return s.outputs
}

// byAmount defines the methods needed to satisify sort.Interface to
// sort a slice of OutputRequests by their amount.
type byAmount []*OutputRequest

func (u byAmount) Len() int           { return len(u) }
func (u byAmount) Less(i, j int) bool { return u[i].amount < u[j].amount }
func (u byAmount) Swap(i, j int)      { u[i], u[j] = u[j], u[i] }

// OutputRequest represents one of the outputs (address/amount) requested by a
// withdrawal, and includes information about the user's outbailment request.
type OutputRequest struct {
	address string
	amount  btcutil.Amount

	// The notary server that received the outbailment request.
	server string

	// The server-specific transaction number for the outbailment request.
	transaction uint32

	// cachedHash is used to cache the hash of the outBailmentID so it
	// only has to be calculated once.
	cachedHash []byte
}

// String makes OutputRequest satisfy the Stringer interface.
func (o *OutputRequest) String() string {
	return fmt.Sprintf("OutputRequest to send %v to %s", o.amount, o.address)
}

// outBailmentIDHash returns a byte slice which is used when sorting
// OutputRequests.
func (o *OutputRequest) outBailmentIDHash() []byte {
	if o.cachedHash != nil {
		return o.cachedHash
	}
	str := fmt.Sprintf("%s%d", o.server, o.transaction)
	hasher := fastsha256.New()
	// hasher.Write() always returns nil as the error, so it's safe to ignore it here.
	_, _ = hasher.Write([]byte(str))
	id := hasher.Sum(nil)
	o.cachedHash = id
	return id
}

func (o *OutputRequest) pkScript(net *btcnet.Params) ([]byte, error) {
	address, err := btcutil.DecodeAddress(o.address, net)
	if err != nil {
		return nil, err
	}
	return btcscript.PayToAddrScript(address)
}

func NewOutputRequest(
	server string, transaction uint32, address string, amount btcutil.Amount) *OutputRequest {
	return &OutputRequest{
		address:     address,
		amount:      amount,
		server:      server,
		transaction: transaction,
	}
}

// WithdrawalOutput represents a possibly fulfilled OutputRequest.
type WithdrawalOutput struct {
	request *OutputRequest
	status  string
	// The outpoints that fulfil the OutputRequest. There will be more than one in case we
	// need to split the request across multiple transactions.
	outpoints []OutBailmentOutpoint
}

func (o *WithdrawalOutput) String() string {
	return fmt.Sprintf("WithdrawalOutput for %s", o.request)
}

func (o *WithdrawalOutput) addOutpoint(outpoint OutBailmentOutpoint) {
	o.outpoints = append(o.outpoints, outpoint)
}

func (o *WithdrawalOutput) Status() string {
	return o.status
}

func (o *WithdrawalOutput) Amount() btcutil.Amount {
	return o.request.amount
}

func (o *WithdrawalOutput) Address() string {
	return o.request.address
}

func (o *WithdrawalOutput) Outpoints() []OutBailmentOutpoint {
	return o.outpoints
}

// XXX: This is a horrible name, really.
// OutBailmentOutpoint represents one of the outpoints created to fulfil an OutputRequest.
type OutBailmentOutpoint struct {
	ntxid  string
	index  uint32
	amount btcutil.Amount
}

func (o OutBailmentOutpoint) Amount() btcutil.Amount {
	return o.amount
}

// A list of raw signatures (one for every pubkey in the multi-sig script)
// for a given transaction input. They should match the order of pubkeys in
// the script and an empty RawSig should be used when the private key for
// a pubkey is not known.
type TxSigs [][]RawSig

type RawSig []byte

// withdrawal holds all the state needed for Pool.Withdrawal() to do its job.
type withdrawal struct {
	roundID        uint32
	status         *WithdrawalStatus
	net            *btcnet.Params
	changeStart    *ChangeAddress
	transactions   []*decoratedTx
	pendingOutputs []*OutputRequest
	eligibleInputs []CreditInterface
	current        *decoratedTx
	// newDecoratedTx is a member of the structure so it can be replaced for
	// testing purposes.
	newDecoratedTx func() *decoratedTx
}

type changeOutput struct {
	pkScript []byte
	amount   btcutil.Amount
}

// A btcwire.MsgTx decorated with some supporting data structures needed throughout the
// withdrawal process.
type decoratedTx struct {
	inputs      []CreditInterface
	outputs     []*WithdrawalOutput
	inputTotal  btcutil.Amount
	outputTotal btcutil.Amount
	fee         btcutil.Amount

	// calculateFee calculates the expected network fees for this transaction.
	// We use a func() field instead of a method so that it can be replaced in
	// tests.
	calculateFee func() btcutil.Amount

	// isTooBig is a member of the structure so it can be replaced for testing
	// purposes.
	isTooBig func() bool

	changeOutput *changeOutput
}

func (d *decoratedTx) hasChange() bool {
	return d.changeOutput != nil
}

func (d *decoratedTx) changeIndex() (uint32, error) {
	if d.hasChange() {
		return uint32(len(d.outputs)), nil
	}
	// XXX(lars): Need a better error here
	return 0, newError(ErrWithdrawalProcessing, "", nil)
}

func (d *decoratedTx) toMsgTx(net *btcnet.Params) (*btcwire.MsgTx, error) {
	msgtx := btcwire.NewMsgTx()
	// add outputs
	for _, o := range d.outputs {
		pkScript, err := o.request.pkScript(net)
		if err != nil {
			o.status = "invalid"
			// XXX(lars): check if this is the correct error.
			return nil, newError(ErrWithdrawalProcessing, "failed to generate pkScript", err)
		}
		msgtx.AddTxOut(btcwire.NewTxOut(int64(o.Amount()), pkScript))
	}

	// add change output
	if d.hasChange() {
		msgtx.AddTxOut(btcwire.NewTxOut(int64(d.changeOutput.amount), d.changeOutput.pkScript))
	}

	// add inputs
	for _, i := range d.inputs {
		msgtx.AddTxIn(btcwire.NewTxIn(i.OutPoint(), nil))
	}
	return msgtx, nil
}

func newDecoratedTx() *decoratedTx {
	tx := &decoratedTx{}
	tx.calculateFee = func() btcutil.Amount {
		// TODO:
		return btcutil.Amount(1)
	}
	tx.isTooBig = func() bool {
		return isTooBig(tx)
	}
	return tx
}

func (d *decoratedTx) addTxOut(output *WithdrawalOutput, pkScript []byte) uint32 {
	log.Infof("Added output sending %s to %s", output.Amount(), output.Address())
	d.outputTotal += output.Amount()
	d.outputs = append(d.outputs, output)
	return uint32(len(d.outputs) - 1)
}

// popOutput will pop the last added output and return it as well as
// update the outputTotal value.  The corresponding output will be
// removed from the underlying MsgTx
func (d *decoratedTx) popOutput() *WithdrawalOutput {
	removed := d.outputs[len(d.outputs)-1]
	d.outputs = d.outputs[:len(d.outputs)-1]
	d.outputTotal -= removed.Amount()
	return removed
}

// popInput will pop the last added input and return it as well as
// update the inputTotal value.  The corresponding input will be
// removed from the underlying MsgTx
func (d *decoratedTx) popInput() CreditInterface {
	removed := d.inputs[len(d.inputs)-1]
	d.inputs = d.inputs[:len(d.inputs)-1]
	d.inputTotal -= removed.Amount()
	return removed
}

func (d *decoratedTx) addTxIn(input CreditInterface) {
	log.Infof("Added input with amount %v", input.Amount())
	d.inputs = append(d.inputs, input)
	d.inputTotal += input.Amount()
}

// addChange adds a change output if there are any satoshis left after paying all the
// outputs and network fees. It returns true if a change output was added, and in that
// case the change output will be the last one in msgtx.TxOut.
// This method must be called only once, and no extra inputs/outputs should be added after
// it's called. Also, callsites must make sure adding a change output won't cause the tx
// to exceed the size limit.
func (d *decoratedTx) addChange(pkScript []byte) bool {
	d.fee = d.calculateFee()
	change := d.inputTotal - d.outputTotal - d.fee
	if change > 0 {
		d.changeOutput = &changeOutput{
			pkScript: pkScript,
			amount:   change,
		}
		log.Infof("Added change output with amount %v", change)
	}
	return d.hasChange()
}

// rollBackLastOutput will roll back the last added output and possibly remove
// inputs that are no longer needed to cover the remaining outputs. The method
// returns the removed output and the removed inputs, if any.
//
// The decorated tx needs to have two or more outputs. The case with only one
// output must be handled separately (by the split output procedure).
func (d *decoratedTx) rollBackLastOutput() ([]CreditInterface, *WithdrawalOutput, error) {
	// Check precondition: At least two outputs are required in the transaction.
	if len(d.outputs) < 2 {
		str := fmt.Sprintf("at least two outputs expected; got %d", len(d.outputs))
		return nil, nil, newError(ErrPreconditionNotMet, str, nil)
	}

	removedOutput := d.popOutput()

	var removedInputs []CreditInterface
	// Continue until sum(in) < sum(out) + fee
	for d.inputTotal >= d.outputTotal+d.calculateFee() {
		removed := d.popInput()
		removedInputs = append(removedInputs, removed)
	}

	// Re-add the last one
	inputTop := removedInputs[len(removedInputs)-1]
	removedInputs = removedInputs[:len(removedInputs)-1]
	d.addTxIn(inputTop)
	return removedInputs, removedOutput, nil
}

func isTooBig(d *decoratedTx) bool {
	// TODO: Implement me!
	return estimateSize(d) > 1000
}

func newWithdrawal(roundID uint32, outputs []*OutputRequest, inputs []CreditInterface,
	changeStart *ChangeAddress, net *btcnet.Params) *withdrawal {
	return &withdrawal{
		roundID:        roundID,
		current:        newDecoratedTx(),
		pendingOutputs: outputs,
		eligibleInputs: inputs,
		status:         &WithdrawalStatus{},
		changeStart:    changeStart,
		net:            net,
		newDecoratedTx: newDecoratedTx,
	}
}

// XXX: This should actually get the input start/stop addresses and pass them on to
// getEligibleInputs().
func (vp *Pool) Withdrawal(
	roundID uint32,
	outputs []*OutputRequest,
	inputs []CreditInterface,
	changeStart *ChangeAddress,
	txStore *txstore.Store,
) (*WithdrawalStatus, map[string]TxSigs, error) {
	w := newWithdrawal(roundID, outputs, inputs, changeStart, vp.manager.Net())
	if err := w.fulfilOutputs(); err != nil {
		return nil, nil, err
	}
	sigs, err := getRawSigs(w.transactions, w.net)
	if err != nil {
		return nil, nil, err
	}

	if err := storeTransactions(txStore, w.transactions, w.net); err != nil {
		return nil, nil, err
	}

	return w.status, sigs, nil
}

// storeTransactions adds the given transactions to the txStore and writes it to
// disk. The credits used in each transaction are removed from the store's
// unspent list, and if a transaction includes a change output, it is added to
// the store as a credit.
// TODO: Wrap the errors we catch here in a custom votingpool.Error before
// returning.
func storeTransactions(txStore *txstore.Store, transactions []*decoratedTx, net *btcnet.Params) error {
	for _, tx := range transactions {
		msgtx, err := tx.toMsgTx(net)
		if err != nil {
			return err
		}
		// XXX(lars): replaced tx.msgtx with tx.toMsgTx here.  not sure if
		// that's the right thing to do performancewise. We could store the
		// generated msgtxs somewhere.
		txr, err := txStore.InsertTx(btcutil.NewTx(msgtx), nil)
		if err != nil {
			return err
		}
		if _, err = txr.AddDebits(); err != nil {
			return err
		}
		if tx.hasChange() {
			idx, err := tx.changeIndex()
			if err != nil {
				return err
			}
			if _, err = txr.AddCredit(idx, true); err != nil {
				return err
			}
		}
	}
	txStore.MarkDirty()
	if err := txStore.WriteIfDirty(); err != nil {
		return err
	}
	return nil
}

// If this returns it means we have added an output and the necessary inputs to fulfil that
// output plus the required fees. It also means the tx won't reach the size limit even
// after we add a change output and sign all inputs.
func (w *withdrawal) fulfilNextOutput() error {
	request := w.pendingOutputs[0]
	w.pendingOutputs = w.pendingOutputs[1:]

	output := &WithdrawalOutput{request: request}
	// Add output to w.status in case we exit early due to on an invalid request.
	w.status.outputs = append(w.status.outputs, output)

	pkScript, err := request.pkScript(w.net)
	if err != nil {
		output.status = "invalid"
		return nil
	}
	outputIndex := w.current.addTxOut(output, pkScript)

	if w.current.isTooBig() {
		if err := w.handleOversizeTx(); err != nil {
			return err
		}
	}

	fee := w.current.calculateFee()
	for w.current.inputTotal < w.current.outputTotal+fee {
		if len(w.eligibleInputs) == 0 {
			// TODO: Implement Split Output procedure
			panic("Split Output not yet implemented")
		}
		input := w.eligibleInputs[0]
		w.eligibleInputs = w.eligibleInputs[1:]
		w.current.addTxIn(input)
		fee = w.current.calculateFee()

		if w.current.isTooBig() {
			if err := w.handleOversizeTx(); err != nil {
				return err
			}
		}
	}

	outpoint := OutBailmentOutpoint{index: outputIndex, amount: output.Amount()}
	output.addOutpoint(outpoint)
	output.status = "success"
	return nil
}

// handleOversizeTx handles the case when a transaction has become too
// big by either rolling back an output or splitting it.
func (w *withdrawal) handleOversizeTx() error {
	if len(w.current.outputs) > 1 {
		inputs, output, err := w.current.rollBackLastOutput()
		if err != nil {
			return newError(ErrWithdrawalProcessing,
				"failed to rollback last output", err)
		}
		w.eligibleInputs = append(w.eligibleInputs, inputs...)
		w.pendingOutputs = append(w.pendingOutputs, output.request)
		w.finalizeCurrentTx()
	} else if len(w.current.outputs) == 1 {
		// TODO: Split last output in two, and continue the loop.
		panic("Oversize TX ouput split not yet implemented")
	}
	return nil
}

// finalizeCurrentTx finalizes the transaction in w.current, moves it to the
// list of finalized transactions and replaces w.current with a new empty
// transaction.
func (w *withdrawal) finalizeCurrentTx() error {
	tx := w.current
	if len(tx.outputs) == 0 {
		return nil
	}

	pkScript, err := btcscript.PayToAddrScript(w.changeStart.addr)
	if err != nil {
		return newError(
			ErrWithdrawalProcessing, "failed to generate pkScript for change address", err)
	}
	// XXX(lars) Change this to check if there was change - then we need to get
	// the next change address.
	if tx.addChange(pkScript) {
		var err error
		w.changeStart, err = w.changeStart.Next()
		if err != nil {
			return newError(
				ErrWithdrawalProcessing, "failed to get next change address", err)
		}
	}
	w.transactions = append(w.transactions, tx)

	// TODO: Update the ntxid of all WithdrawalOutput entries fulfilled by this transaction

	w.current = w.newDecoratedTx()
	return nil
}

// maybeDropOutputs will check the total amount we have in eligible inputs and drop
// requested outputs (in descending amount order) if we don't have enough to fulfil them
// all. For every dropped output request we add an entry to w.status.outputs with the
// status string set to "partial-".
func (w *withdrawal) maybeDropOutputs() {
	inputAmount := btcutil.Amount(0)
	for _, input := range w.eligibleInputs {
		inputAmount += input.Amount()
	}
	outputAmount := btcutil.Amount(0)
	for _, output := range w.pendingOutputs {
		outputAmount += output.amount
	}
	sort.Sort(sort.Reverse(byAmount(w.pendingOutputs)))
	for inputAmount < outputAmount {
		output := w.pendingOutputs[0]
		log.Infof("Not fulfilling request to send %v to %v; not enough credits.",
			output.amount, output.address)
		w.pendingOutputs = w.pendingOutputs[1:]
		outputAmount -= output.amount
		// XXX: Do not hardcode the status strings here, nor in tests.
		w.status.outputs = append(
			w.status.outputs, &WithdrawalOutput{request: output, status: "partial-"})
	}
}

func (w *withdrawal) fulfilOutputs() error {
	w.maybeDropOutputs()
	if len(w.pendingOutputs) == 0 {
		return nil
	}

	// Sort outputs by outBailmentID (hash(server ID, tx #))
	sort.Sort(byOutBailmentID(w.pendingOutputs))

	for len(w.pendingOutputs) > 0 {
		if err := w.fulfilNextOutput(); err != nil {
			return err
		}
	}

	if err := w.finalizeCurrentTx(); err != nil {
		return err
	}

	for _, tx := range w.transactions {
		w.updateStatusFor(tx)
		w.status.fees += tx.fee
	}
	return nil
}

func (w *withdrawal) updateStatusFor(tx *decoratedTx) {
	// TODO
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

// getRawSigs iterates over the inputs of each transaction given, constructing the
// raw signatures for them using the private keys available to us.
// creditsUsed must have one entry for every transaction, with the transaction's ntxid
// as the key and a slice of credits spent by that transaction as the value.
// It returns a map of ntxids to signature lists.
func getRawSigs(transactions []*decoratedTx, net *btcnet.Params) (map[string]TxSigs, error) {
	sigs := make(map[string]TxSigs)
	for _, tx := range transactions {
		txSigs := make(TxSigs, len(tx.inputs))
		// XXX(lars): replaced an msgtx instance with toMsgTx() which is hardly
		// good for performance. This should be replaced with something else at
		// some point.
		msgtx, err := tx.toMsgTx(net)
		if err != nil {
			return nil, newError(ErrRawSigning, "failed to generate msgtx", err)
		}
		ntxid := Ntxid(msgtx)
		for inputIdx, input := range tx.inputs {
			creditAddr := input.Address()
			redeemScript := creditAddr.RedeemScript()
			series := creditAddr.Series()
			// The order of the raw signatures in the signature script must match the
			// order of the public keys in the redeem script, so we sort the public keys
			// here using the same API used to sort them in the redeem script and use
			// series.getPrivKeyFor() to lookup the corresponding private keys.
			pubKeys, err := branchOrder(series.publicKeys, creditAddr.Branch())
			if err != nil {
				return nil, err
			}
			txInSigs := make([]RawSig, len(pubKeys))
			for i, pubKey := range pubKeys {
				var sig RawSig
				privKey, err := series.getPrivKeyFor(pubKey)
				if err != nil {
					return nil, err
				}
				if privKey != nil {
					childKey, err := privKey.Child(uint32(creditAddr.Index()))
					if err != nil {
						return nil, newError(ErrKeyChain, "failed to derive private key", err)
					}
					ecPrivKey, err := childKey.ECPrivKey()
					if err != nil {
						return nil, newError(ErrKeyChain, "failed to obtain ECPrivKey", err)
					}
					log.Infof("Signing input %d of tx %s with privkey of %s",
						inputIdx, ntxid, pubKey.String())
					sig, err = btcscript.RawTxInSignature(
						msgtx, inputIdx, redeemScript, btcscript.SigHashAll, ecPrivKey)
					if err != nil {
						return nil, newError(
							ErrRawSigning, "failed to generate raw signature", err)
					}
				} else {
					log.Infof(
						"Not signing input %d of %s because private key for %s is "+
							"not available: %v", inputIdx, ntxid, pubKey.String(), err)
					sig = []byte{}
				}
				txInSigs[i] = sig
			}
			txSigs[inputIdx] = txInSigs
		}
		sigs[ntxid] = txSigs
	}
	return sigs, nil
}

// XXX: This assumes that the voting pool deposit script was imported into
// waddrmgr, which is currently not done automatically when we generate a new
// deposit script/address.
// SignTx signs every input of the given MsgTx by looking up (on the addr
// manager) the redeem script for each of them and constructing the signature
// script using that and the given raw signatures.
// This function must be called with the manager unlocked.
func SignTx(msgtx *btcwire.MsgTx, sigs TxSigs, mgr *waddrmgr.Manager, store *txstore.Store) error {
	for i, txIn := range msgtx.TxIn {
		txOut, err := store.UnconfirmedSpent(txIn.PreviousOutPoint)
		if err != nil {
			errStr := fmt.Sprintf("unable to find previous outpoint of tx input #%d", i)
			return newError(ErrTxSigning, errStr, err)
		}
		if err = signMultiSigUTXO(mgr, msgtx, i, txOut.PkScript, sigs[i]); err != nil {
			return err
		}
	}
	return nil
}

// getRedeemScript returns the redeem script for the given P2SH address. It must
// be called with the manager unlocked.
func getRedeemScript(mgr *waddrmgr.Manager, addr *btcutil.AddressScriptHash) ([]byte, error) {
	address, err := mgr.Address(addr)
	if err != nil {
		return nil, err
	}
	return address.(waddrmgr.ManagedScriptAddress).Script()
}

// signMultiSigUTXO signs the P2SH UTXO with the given index by constructing a
// script containing all given signatures plus the redeem (multi-sig) script. The
// redeem script is obtained by looking up the address of the given P2SH pkScript
// on the address manager.
// The order of the signatures must match that of the public keys in the multi-sig
// script as OP_CHECKMULTISIG expects that.
// This function must be called with the manager unlocked.
func signMultiSigUTXO(mgr *waddrmgr.Manager, tx *btcwire.MsgTx, idx int, pkScript []byte, sigs []RawSig) error {
	class, addresses, _, err := btcscript.ExtractPkScriptAddrs(pkScript, mgr.Net())
	if err != nil {
		return newError(ErrTxSigning, "unparseable pkScript", err)
	}
	if class != btcscript.ScriptHashTy {
		return newError(ErrTxSigning, fmt.Sprintf("pkScript is not P2SH: %s", class), nil)
	}
	redeemScript, err := getRedeemScript(mgr, addresses[0].(*btcutil.AddressScriptHash))
	if err != nil {
		return newError(ErrTxSigning, "unable to retrieve redeem script", err)
	}

	class, _, nRequired, err := btcscript.ExtractPkScriptAddrs(redeemScript, mgr.Net())
	if err != nil {
		return newError(ErrTxSigning, "unparseable redeem script", err)
	}
	if class != btcscript.MultiSigTy {
		return newError(
			ErrTxSigning, fmt.Sprintf("redeem script is not multi-sig: %v", class), nil)
	}
	if len(sigs) < nRequired {
		errStr := fmt.Sprintf(
			"not enough signatures; need %d but got only %d", nRequired, len(sigs))
		return newError(ErrTxSigning, errStr, nil)
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

	if err := validateSigScript(tx, idx, pkScript); err != nil {
		return err
	}
	return nil
}

// validateSigScripts executes the signature script of the tx input with the
// given index, returning an error if it fails.
func validateSigScript(msgtx *btcwire.MsgTx, idx int, pkScript []byte) error {
	flags := btcscript.ScriptCanonicalSignatures | btcscript.ScriptStrictMultiSig | btcscript.ScriptBip16
	txIn := msgtx.TxIn[idx]
	engine, err := btcscript.NewScript(txIn.SignatureScript, pkScript, idx, msgtx, flags)
	if err != nil {
		return newError(ErrTxSigning, "cannot create script engine", err)
	}
	if err = engine.Execute(); err != nil {
		return newError(ErrTxSigning, "cannot validate tx signature", err)
	}
	return nil
}

func estimateSize(tx *decoratedTx) uint32 {
	// TODO: Implement me
	// This function could estimate the size given the number of inputs/outputs, similarly
	// to estimateTxSize() (in createtx.go), or it could copy the tx, add a stub change
	// output, fill the SignatureScript for every input and serialize it.
	return 0
}
