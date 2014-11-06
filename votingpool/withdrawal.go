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

var log btclog.Logger

func init() {
	// XXX: Make it possible to switch this on/off like in txstore/log.go
	log, _ = btclog.NewLoggerFromWriter(os.Stdout, btclog.DebugLvl)
}

func (vp *VotingPool) ChangeAddress(seriesID, index uint32) (*ChangeAddress, error) {
	// TODO: Ensure the given series is active.
	// Branch is always 0 for change addresses.
	branch := uint32(0)
	addr, err := vp.DepositScriptAddress(seriesID, branch, index)
	if err != nil {
		return nil, err
	}
	vpAddr := &votingPoolAddress{seriesID: seriesID, branch: branch, index: index, addr: addr}
	return &ChangeAddress{vp: vp, votingPoolAddress: vpAddr}, nil
}

func (vp *VotingPool) WithdrawalAddress(seriesID, branch, index uint32) (*WithdrawalAddress, error) {
	// TODO: Ensure the given series is hot.
	addr, err := vp.DepositScriptAddress(seriesID, branch, index)
	if err != nil {
		return nil, err
	}
	vpAddr := &votingPoolAddress{seriesID: seriesID, branch: branch, index: index, addr: addr}
	return &WithdrawalAddress{votingPoolAddress: vpAddr}, nil
}

type votingPoolAddress struct {
	addr     btcutil.Address
	seriesID uint32
	branch   uint32
	index    uint32
}

func (a *votingPoolAddress) Addr() btcutil.Address {
	return a.addr
}

type ChangeAddress struct {
	*votingPoolAddress
	vp *VotingPool
}

func (a *ChangeAddress) Next() (*ChangeAddress, error) {
	return a.vp.ChangeAddress(a.seriesID, a.index+1)
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

// OutputRequest represents one of the outputs (address/amount) requested by a
// withdrawal, and includes information about the user's outbailment request.
type OutputRequest struct {
	address string
	amount  btcutil.Amount
	// The notary server that received the outbailment request.
	server string
	// The server-specific transaction number for the outbailment request.
	transaction uint32
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
		server: server, transaction: transaction, address: address, amount: amount}
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

// A list of raw signatures (one for every pubkey in the multi-sig script)
// for a given transaction input. They should match the order of pubkeys in
// the script and an empty RawSig should be used when the private key for
// a pubkey is not known.
type TxInSignatures [][]RawSig

type RawSig []byte

// withdrawal holds all the state needed for VotingPool.Withdrawal() to do its job.
type withdrawal struct {
	roundID        uint32
	status         *WithdrawalStatus
	net            *btcnet.Params
	changeStart    *ChangeAddress
	transactions   []*btcwire.MsgTx
	pendingOutputs []*OutputRequest
	eligibleInputs []txstore.Credit
	current        *currentTx
	// A map of ntxids to lists of txstore.Credit, needed to sign the tx inputs.
	usedInputs map[string][]txstore.Credit
}

// The not-yet-finalized transaction to which new inputs/outputs are being added, and
// some supporting data structures that apply only to it.
type currentTx struct {
	tx          *btcwire.MsgTx
	inputs      []txstore.Credit
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

func (c *currentTx) addTxIn(input txstore.Credit) {
	c.tx.AddTxIn(btcwire.NewTxIn(input.OutPoint(), nil))
	log.Infof("Added input with amount %v", input.Amount())
	c.inputs = append(c.inputs, input)
	c.inputTotal += input.Amount()
}

func (c *currentTx) rollBackLastOutput() {
	// TODO: Remove output from tx.TxOut
	// TODO: Subtract its amount from outputTotal
}

func (c *currentTx) tooBig() bool {
	// TODO: Implement me!
	return estimateSize(c.tx) > 1000
}

// XXX: This should actually get the input start/stop addresses and pass them on to
// getEligibleInputs().
func (vp *VotingPool) Withdrawal(
	roundID uint32,
	outputs []*OutputRequest,
	inputs []txstore.Credit,
	changeStart *ChangeAddress,
	txStore *txstore.Store,
) (*WithdrawalStatus, map[string]TxInSignatures, error) {
	w := &withdrawal{
		roundID:        roundID,
		current:        &currentTx{tx: btcwire.NewMsgTx()},
		pendingOutputs: outputs,
		usedInputs:     make(map[string][]txstore.Credit),
		eligibleInputs: inputs,
		status:         &WithdrawalStatus{},
		changeStart:    changeStart,
		net:            vp.manager.Net(),
	}
	if err := w.fulfilOutputs(txStore); err != nil {
		return nil, nil, err
	}
	sigs, err := w.sign(vp.manager)
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

	// TODO: Store the raw signatures in the DB. We'll probably need to store the whole
	// WithdrawalStatus in the DB as well, in case we're called multiple times with the
	// same roundID.

	// XXX: Using separate DBs for the transactions and the signatures may be problematic.
	// What happens if we store the transactions and fail to store the signatures? We
	// could store the siglists first and roll them back (which should be easy) if we fail
	// to store the txs.
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

	if w.current.tooBig() {
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

		if w.current.tooBig() {
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

func (w *withdrawal) finalizeCurrentTx() {
	if len(w.current.tx.TxOut) == 0 {
		return
	}
	fee := calculateFee(w.current.tx)
	change := w.current.inputTotal - w.current.outputTotal - fee
	if change > 0 {
		addr := w.changeStart.addr
		pkScript, err := btcscript.PayToAddrScript(addr)
		if err != nil {
			panic(err) // XXX: Really no idea what to do if we get an error here...
		}
		w.current.tx.AddTxOut(btcwire.NewTxOut(int64(change), pkScript))
		log.Infof("Added change output with amount %v", change)
		w.changeStart, err = w.changeStart.Next()
		if err != nil {
			panic(err) // XXX: Really no idea what to do if we get an error here...
		}
	}

	w.usedInputs[Ntxid(w.current.tx)] = w.current.inputs
	w.transactions = append(w.transactions, w.current.tx)
	w.status.fees += fee

	// TODO: Update the ntxid of all WithdrawalOutput entries fulfilled by this transaction

	w.current = &currentTx{tx: btcwire.NewMsgTx()}
}

func (w *withdrawal) fulfilOutputs(store *txstore.Store) error {
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

// sign iterates over inputs in each transaction generated by this withdrawal,
// constructing the raw signature for them. It returns a map of ntxids to signature
// lists.
// TODO: Add a test that uses a fixed transaction and compares the well known signatures
// (including their order) against the list returned here.
func (w *withdrawal) sign(mgr *waddrmgr.Manager) (map[string]TxInSignatures, error) {
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
			txInSigs := make([]RawSig, len(addresses))
			for addrIdx, addr := range addresses {
				var sig RawSig
				privKey, err := getPrivKey(mgr, addr.(*btcutil.AddressPubKey))
				if err == nil {
					log.Infof("Signing input %d of tx %s with privkey of %s",
						idx, ntxid, addr)
					sig, err = btcscript.RawTxInSignature(
						tx, idx, redeemScript, btcscript.SigHashAll, privKey)
					if err != nil {
						panic(err) // XXX: Again, no idea what's the correct thing to do here.
					}
				} else {
					log.Infof(
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
