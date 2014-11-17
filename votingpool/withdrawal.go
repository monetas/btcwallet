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
		ErrUnknownPubKey, fmt.Sprintf("Unknown public key '%s'", pubKey.String()), nil)
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
	p        *Pool
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
			p: p, seriesID: seriesID, branch: branch, index: index, addr: addr, script: script},
		nil
}

func (a *votingPoolAddress) Addr() btcutil.Address {
	return a.addr
}

func (a *votingPoolAddress) RedeemScript() []byte {
	return a.script
}

func (a *votingPoolAddress) Series() *SeriesData {
	return a.p.GetSeries(a.seriesID)
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
	return a.p.ChangeAddress(a.seriesID, a.index+1)
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
	hasher.Write([]byte(str))
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
	transactions   []*btcwire.MsgTx
	pendingOutputs []*OutputRequest
	eligibleInputs []CreditInterface
	current        *currentTx
	// A map of ntxids to lists of CreditInterface, needed to sign the tx inputs.
	usedInputs map[string][]CreditInterface
}

// The not-yet-finalized transaction to which new inputs/outputs are being added, and
// some supporting data structures that apply only to it.
type currentTx struct {
	tx          *btcwire.MsgTx
	inputs      []CreditInterface
	outputs     []*WithdrawalOutput
	inputTotal  btcutil.Amount
	outputTotal btcutil.Amount
}

func (c *currentTx) addTxOut(output *WithdrawalOutput, pkScript []byte) uint32 {
	c.tx.AddTxOut(btcwire.NewTxOut(int64(output.Amount()), pkScript))
	c.outputTotal += output.Amount()
	c.outputs = append(c.outputs, output)
	return uint32(len(c.tx.TxOut) - 1)
}

func (c *currentTx) addTxIn(input CreditInterface) {
	c.tx.AddTxIn(btcwire.NewTxIn(input.OutPoint(), nil))
	log.Infof("Added input with amount %v", input.Amount())
	c.inputs = append(c.inputs, input)
	c.inputTotal += input.Amount()
}

func (c *currentTx) rollBackLastOutput() {
	// TODO: Remove output from tx.TxOut
	// TODO: Subtract its amount from outputTotal
}

func (c *currentTx) isTooBig() bool {
	// TODO: Implement me!
	return estimateSize(c.tx) > 1000
}

func newWithdrawal(roundID uint32, outputs []*OutputRequest, inputs []CreditInterface,
	changeStart *ChangeAddress, net *btcnet.Params) *withdrawal {
	return &withdrawal{
		roundID:        roundID,
		current:        &currentTx{tx: btcwire.NewMsgTx()},
		pendingOutputs: outputs,
		usedInputs:     make(map[string][]CreditInterface),
		eligibleInputs: inputs,
		status:         &WithdrawalStatus{},
		changeStart:    changeStart,
		net:            net,
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
	sigs, err := getRawSigs(w.transactions, w.usedInputs)
	if err != nil {
		return nil, nil, err
	}

	// Store the transactions in the txStore and write it to disk.
	for _, tx := range w.transactions {
		txr, err := txStore.InsertTx(btcutil.NewTx(tx), nil)
		if err != nil {
			return nil, nil, err
		}
		if _, err = txr.AddDebits(); err != nil {
			return nil, nil, err
		}
		// XXX: Must only do this if the transaction has a change output.
		if _, err = txr.AddCredit(uint32(len(tx.TxOut)-1), true); err != nil {
			return nil, nil, err
		}
	}
	txStore.MarkDirty()
	if err := txStore.WriteIfDirty(); err != nil {
		return nil, nil, err
	}

	return w.status, sigs, nil
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
	log.Infof("Added output sending %s to %s", output.Amount(), output.Address())

	if w.current.isTooBig() {
		// TODO: Roll back last added output, finalize w.currentTx and assign a new
		// tx to currentTx.
		panic("Oversize TX not yet implemented")
	}

	fee := calculateFee(w.current.tx)
	for w.current.inputTotal < w.current.outputTotal+fee {
		if len(w.eligibleInputs) == 0 {
			// TODO: Implement Split Output procedure
			panic("Split Output not yet implemented")
		}
		input := w.eligibleInputs[0]
		w.eligibleInputs = w.eligibleInputs[1:]
		w.current.addTxIn(input)
		fee = calculateFee(w.current.tx)

		if w.current.isTooBig() {
			// TODO: Roll back last added output plus all inputs added to support it.
			if len(w.current.tx.TxOut) > 1 {
				w.finalizeCurrentTx()
				// TODO: Finalize w.currentTx and assign a new tx to currentTx.
			} else if len(w.current.tx.TxOut) == 1 {
				// TODO: Split last output in two, and continue the loop.
			}
			panic("Oversize TX not yet implemented")
		}
	}

	outpoint := OutBailmentOutpoint{index: outputIndex, amount: output.Amount()}
	output.addOutpoint(outpoint)
	output.status = "success"
	return nil
}

func (w *withdrawal) finalizeCurrentTx() error {
	if len(w.current.tx.TxOut) == 0 {
		return nil
	}
	fee := calculateFee(w.current.tx)
	change := w.current.inputTotal - w.current.outputTotal - fee
	if change > 0 {
		addr := w.changeStart.addr
		pkScript, err := btcscript.PayToAddrScript(addr)
		if err != nil {
			return newError(
				ErrWithdrawalProcessing, "Failed to generate pkScript for change address", err)
		}
		w.current.tx.AddTxOut(btcwire.NewTxOut(int64(change), pkScript))
		log.Infof("Added change output with amount %v", change)
		w.changeStart, err = w.changeStart.Next()
		if err != nil {
			return newError(
				ErrWithdrawalProcessing, "Failed to get next change address", err)
		}
	}

	w.usedInputs[Ntxid(w.current.tx)] = w.current.inputs
	w.transactions = append(w.transactions, w.current.tx)
	w.status.fees += fee

	// TODO: Update the ntxid of all WithdrawalOutput entries fulfilled by this transaction

	w.current = &currentTx{tx: btcwire.NewMsgTx()}
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
	}
	return nil
}

func (w *withdrawal) updateStatusFor(tx *btcwire.MsgTx) {
	// TODO
}

// XXX: This assumes that the voting pool deposit script was imported into waddrmgr
// This function must be called with the manager unlocked.
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
func getRawSigs(transactions []*btcwire.MsgTx, creditsUsed map[string][]CreditInterface,
) (map[string]TxSigs, error) {
	sigs := make(map[string]TxSigs)
	for _, tx := range transactions {
		txSigs := make(TxSigs, len(tx.TxIn))
		ntxid := Ntxid(tx)
		for idx := range tx.TxIn {
			creditAddr := creditsUsed[ntxid][idx].Address()
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
						return nil, newError(
							ErrWithdrawalProcessing, "Failed to derive key", err)
					}
					ecPrivKey, err := childKey.ECPrivKey()
					if err != nil {
						return nil, newError(
							ErrWithdrawalProcessing, "Failed to derive key", err)
					}
					log.Infof("Signing input %d of tx %s with privkey of %s",
						idx, ntxid, pubKey.String())
					sig, err = btcscript.RawTxInSignature(
						tx, idx, redeemScript, btcscript.SigHashAll, ecPrivKey)
					if err != nil {
						return nil, newError(
							ErrWithdrawalProcessing, "Failed to generate raw signature", err)
					}
				} else {
					log.Infof(
						"Not signing input %d of %s because private key for %s is "+
							"not available: %v", idx, ntxid, pubKey.String(), err)
					sig = []byte{}
				}
				txInSigs[i] = sig
			}
			txSigs[idx] = txInSigs
		}
		sigs[ntxid] = txSigs
	}
	return sigs, nil
}

// SignMultiSigUTXO signs the P2SH UTXO with the given index by constructing a
// script containing all given signatures plus the redeem (multi-sig) script. The
// redeem script is obtained by looking up the address of the given P2SH pkScript
// on the address manager.
// The order of the signatures must match that of the public keys in the multi-sig
// script as OP_CHECKMULTISIG expects that.
func SignMultiSigUTXO(mgr *waddrmgr.Manager, tx *btcwire.MsgTx, idx int, pkScript []byte, sigs []RawSig, net *btcnet.Params) error {
	class, addresses, _, err := btcscript.ExtractPkScriptAddrs(pkScript, net)
	if err != nil {
		panic(err) // XXX: Again, no idea what's the correct thing to do here.
	}
	if class != btcscript.ScriptHashTy {
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

// XXX: This should be private.
// ValidateSigScripts executes the signature script of every input in the given transaction
// and returns an error if any of them fail.
func ValidateSigScripts(msgtx *btcwire.MsgTx, pkScripts [][]byte) error {
	flags := btcscript.ScriptCanonicalSignatures | btcscript.ScriptStrictMultiSig | btcscript.ScriptBip16
	for i, txin := range msgtx.TxIn {
		engine, err := btcscript.NewScript(txin.SignatureScript, pkScripts[i], i, msgtx, flags)
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
