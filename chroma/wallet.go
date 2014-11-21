package chroma

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/monetas/btcscript"
	"github.com/monetas/btcutil"
	"github.com/monetas/btcutil/hdkeychain"
	"github.com/monetas/btcwallet/waddrmgr"
	"github.com/monetas/btcwallet/walletdb"
	"github.com/monetas/btcwire"
	"github.com/monetas/gochroma"
)

type Wallet struct {
	manager   *waddrmgr.Manager
	namespace walletdb.Namespace
	privKey   *hdkeychain.ExtendedKey
	pubKey    *hdkeychain.ExtendedKey
}

func Create(namespace walletdb.Namespace, mgr *waddrmgr.Manager, seed []byte) (*Wallet, error) {
	var err error
	if seed == nil {
		seed, err = hdkeychain.GenerateSeed(32)
		if err != nil {
			return nil, errors.New("failed to generate seed")
		}
	}
	if len(seed) != 32 {
		return nil, errors.New("Need a 32 byte seed")
	}

	// get the hd root
	priv, err := hdkeychain.NewMaster(seed)
	if err != nil {
		return nil, errors.New("failed to derive master extended key")
	}
	pub, err := priv.Neuter()
	if err != nil {
		return nil, errors.New("failed to get extended public key")
	}

	err = namespace.Update(
		func(tx walletdb.Tx) error {
			return initialize(tx, priv, pub)
		})
	if err != nil {
		return nil, err
	}

	return &Wallet{mgr, namespace, priv, pub}, nil
}

func Load(namespace walletdb.Namespace, mgr *waddrmgr.Manager) (*Wallet, error) {
	var priv, pub *hdkeychain.ExtendedKey
	err := namespace.View(func(tx walletdb.Tx) error {
		var err error
		priv, pub, err = fetchKeys(tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &Wallet{mgr, namespace, priv, pub}, nil
}

func (w *Wallet) Close() error {
	w.pubKey = nil
	w.privKey = nil
	return w.manager.Close()
}

func (w *Wallet) newAddress(acct uint32) (btcutil.Address, error) {
	subKey, err := w.pubKey.Child(acct)
	if err != nil {
		return nil, err
	}
	var index *uint32
	err = w.namespace.View(func(tx walletdb.Tx) error {
		tmp, err := fetchAcctIndex(tx, acct)
		index = &(*tmp)
		return err
	})
	if err != nil {
		return nil, err
	}

	key, err := subKey.Child(*index)
	if err != nil {
		return nil, err
	}
	addr, err := key.Address(w.manager.Net())
	if err != nil {
		return nil, err
	}

	err = w.namespace.Update(func(tx walletdb.Tx) error {
		err := storeAcctIndex(tx, acct, *index+1)
		if err != nil {
			return err
		}
		return storeScriptIndex(tx, acct, *index, addr)
	})
	if err != nil {
		return nil, err
	}

	return addr, nil
}

func (w *Wallet) NewUncoloredAddress() (btcutil.Address, error) {
	return w.newAddress(uncoloredAcctNum)
}

func (w *Wallet) NewIssuingAddress() (btcutil.Address, error) {
	return w.newAddress(issuingAcctNum)
}

func (w *Wallet) NewColorAddress(cd *gochroma.ColorDefinition) (btcutil.Address, error) {
	return w.newAddress(cd.AccountNumber())
}

func (w *Wallet) FetchColorId(cd *gochroma.ColorDefinition) (ColorId, error) {
	var colorId ColorId
	err := w.namespace.Update(func(tx walletdb.Tx) error {
		tmp, err := fetchColorId(tx, cd)
		colorId = make(ColorId, len(tmp))
		copy(colorId, tmp)
		return err
	})
	if err != nil {
		return nil, err
	}
	return colorId, nil
}

func (w *Wallet) NewUncoloredOutPoint(b *gochroma.BlockExplorer, outPoint *btcwire.OutPoint) (*ColorOutPoint, error) {
	// look up the outpoint and see if it's already in the db
	var outPointId OutPointId
	err := w.namespace.Update(func(tx walletdb.Tx) error {
		tmp := fetchOutPointId(tx, outPoint)
		if tmp != nil {
			str := fmt.Sprintf("out point %v already in db", outPoint)
			return errors.New(str)
		}
		tmp, err := newOutPointId(tx)
		outPointId = make(OutPointId, len(tmp))
		copy(outPointId, tmp)
		return err
	})
	if err != nil {
		return nil, err
	}

	// get tx data
	tx, err := b.OutPointTx(outPoint)
	if err != nil {
		return nil, err
	}
	msgTx := tx.MsgTx()
	txOut := msgTx.TxOut[outPoint.Index]
	satoshiValue := uint64(txOut.Value)
	pkScript := txOut.PkScript

	// construct the outpoint
	colorOutPoint := &ColorOutPoint{
		Id:         outPointId,
		Tx:         gochroma.BigEndianBytes(&outPoint.Hash),
		Index:      outPoint.Index,
		Value:      satoshiValue,
		Color:      UncoloredColorId,
		ColorValue: gochroma.ColorValue(satoshiValue),
		PkScript:   pkScript,
	}

	// store outpoint in DB
	err = w.namespace.Update(func(tx walletdb.Tx) error {
		return storeOutPoint(tx, colorOutPoint)
	})
	if err != nil {
		return nil, err
	}

	return colorOutPoint, nil
}

func (w *Wallet) NewColorOutPoint(b *gochroma.BlockExplorer, outPoint *btcwire.OutPoint, cd *gochroma.ColorDefinition) (*ColorOutPoint, error) {
	// look up the outpoint and see if it's already in the db
	var outPointId OutPointId
	err := w.namespace.Update(func(tx walletdb.Tx) error {
		var err error
		outPointId = fetchOutPointId(tx, outPoint)
		if outPointId != nil {
			str := fmt.Sprintf("out point %v already in db", outPoint)
			return errors.New(str)
		}
		tmp, err := newOutPointId(tx)
		outPointId = make(OutPointId, len(tmp))
		copy(outPointId, tmp)
		return err
	})
	if err != nil {
		return nil, err
	}

	// get tx data
	tx, err := b.OutPointTx(outPoint)
	if err != nil {
		return nil, err
	}
	msgTx := tx.MsgTx()
	txOut := msgTx.TxOut[outPoint.Index]
	satoshiValue := uint64(txOut.Value)
	pkScript := txOut.PkScript

	// get color data
	var color ColorId
	err = w.namespace.Update(func(tx walletdb.Tx) error {
		tmp, err := fetchColorId(tx, cd)
		color = make(ColorId, len(tmp))
		copy(color, tmp)
		return err
	})
	if err != nil {
		return nil, err
	}
	colorValue, err := cd.ColorValue(b, outPoint)

	if err != nil {
		return nil, err
	}

	// construct the outpoint
	colorOutPoint := &ColorOutPoint{
		Id:         outPointId,
		Tx:         gochroma.BigEndianBytes(&outPoint.Hash),
		Index:      outPoint.Index,
		Value:      satoshiValue,
		Color:      color,
		ColorValue: *colorValue,
		PkScript:   pkScript,
	}

	// store outpoint in DB
	err = w.namespace.Update(func(tx walletdb.Tx) error {
		return storeOutPoint(tx, colorOutPoint)
	})
	if err != nil {
		return nil, err
	}

	return colorOutPoint, nil
}

func (w *Wallet) Sign(pkScript []byte, tx *btcwire.MsgTx, txIndex int) error {
	var acct, index *uint32
	err := w.namespace.View(func(tx walletdb.Tx) error {
		var err error
		acct, index, err = lookupScript(tx, pkScript)
		return err
	})
	if err != nil {
		return err
	}
	acctKey, err := w.privKey.Child(*acct)
	if err != nil {
		return err
	}
	indexKey, err := acctKey.Child(*index)
	if err != nil {
		return err
	}
	privateKey, err := indexKey.ECPrivKey()
	if err != nil {
		return err
	}

	sigScript, err := btcscript.SignatureScript(
		tx, txIndex, pkScript, btcscript.SigHashAll, privateKey, true)
	if err != nil {
		str := fmt.Sprintf("cannot create sigScript: %s", err)
		return errors.New(str)
	}
	tx.TxIn[txIndex].SignatureScript = sigScript
	return nil
}

func (w *Wallet) fetchSpendable(b *gochroma.BlockExplorer, colorId ColorId, needed gochroma.ColorValue) ([]*ColorOutPoint, error) {

	var colorOutPoints []*ColorOutPoint
	err := w.namespace.View(func(tx walletdb.Tx) error {
		var err error
		colorOutPoints, err = allColorOutPoints(tx)
		return err
	})
	if err != nil {
		return nil, err
	}

	var ret []*ColorOutPoint
	sum := gochroma.ColorValue(0)
	for _, colorOutPoint := range colorOutPoints {

		if !colorOutPoint.Spent && bytes.Compare(colorOutPoint.Color, colorId) == 0 {
			// check again if the colorOutPoint has been spent
			op, err := colorOutPoint.OutPoint()
			if err != nil {
				return nil, err
			}
			spent, err := b.OutPointSpent(op)
			if err != nil {
				return nil, err
			}

			if *spent {
				// update this outpoint
				colorOutPoint.Spent = true
				err := w.namespace.Update(func(tx walletdb.Tx) error {
					return storeOutPoint(tx, colorOutPoint)
				})
				if err != nil {
					return nil, err
				}
				continue
			}
			ret = append(ret, colorOutPoint)
			sum += colorOutPoint.ColorValue
			if sum >= needed {
				break
			}
		}
	}
	if sum < needed {
		str := fmt.Sprintf("Need %d value, only have %d value", needed, sum)
		return nil, errors.New(str)
	}

	return ret, nil
}

func (w *Wallet) ColorBalance(colorId ColorId) (*gochroma.ColorValue, error) {
	var colorOutPoints []*ColorOutPoint
	err := w.namespace.View(func(tx walletdb.Tx) error {
		var err error
		colorOutPoints, err = allColorOutPoints(tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	sum := gochroma.ColorValue(0)
	for _, colorOutPoint := range colorOutPoints {
		if bytes.Compare(colorOutPoint.Color, colorId) == 0 && !colorOutPoint.Spent {
			sum += colorOutPoint.ColorValue
		}
	}

	return &sum, nil
}

func (w *Wallet) AllColors() (map[*gochroma.ColorDefinition]ColorId, error) {
	var cds map[*gochroma.ColorDefinition]ColorId
	err := w.namespace.View(func(tx walletdb.Tx) error {
		var err error
		cds, err = allColors(tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return cds, nil
}

func (w *Wallet) IssueColor(b *gochroma.BlockExplorer, kernel gochroma.ColorKernel, value gochroma.ColorValue, fee int64) (*gochroma.ColorDefinition, error) {
	needed := gochroma.ColorValue(kernel.IssuingSatoshiNeeded(value) + fee)
	ins, err := w.fetchSpendable(b, UncoloredColorId, needed)
	if err != nil {
		return nil, err
	}
	inputs := make([]*btcwire.OutPoint, len(ins))
	for i, in := range ins {
		inputs[i], err = in.OutPoint()
		if err != nil {
			return nil, err
		}
	}

	outAddr, err := w.NewIssuingAddress()
	if err != nil {
		return nil, err
	}
	pkScript, err := btcscript.PayToAddrScript(outAddr)
	if err != nil {
		return nil, err
	}
	outputs := []*gochroma.ColorOut{&gochroma.ColorOut{pkScript, value}}
	changeAddr, err := w.NewUncoloredAddress()
	if err != nil {
		return nil, err
	}
	changeScript, err := btcscript.PayToAddrScript(changeAddr)
	if err != nil {
		return nil, err
	}

	tx, err := kernel.IssuingTx(b, inputs, outputs, changeScript, fee)
	if err != nil {
		return nil, err
	}

	// sign everything
	for i, in := range ins {
		err = w.Sign(in.PkScript, tx, i)
		if err != nil {
			return nil, err
		}
	}

	txHash, err := b.PublishTx(tx)
	if err != nil {
		return nil, err
	}

	var genesis *btcwire.OutPoint
	var cd *gochroma.ColorDefinition

	// mark everything as spent
	w.namespace.Update(func(tx walletdb.Tx) error {
		for i, in := range ins {
			in.Spent = true
			in.SpendingTx = gochroma.BigEndianBytes(txHash)
			in.SpendingIndex = uint32(i)
			err = storeOutPoint(tx, in)
			if err != nil {
				return err
			}
		}
		// make the new color definition
		genesis = btcwire.NewOutPoint(txHash, 0)
		var err error
		cd, err = gochroma.NewColorDefinition(kernel, genesis, 0)
		if err != nil {
			return err
		}
		_, err = fetchColorId(tx, cd)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// add genesis as color outpoint
	_, err = w.NewColorOutPoint(b, genesis, cd)

	if err != nil {
		return nil, err
	}

	// if there's any change, add them back as outpoints
	if len(tx.TxOut) > 1 {
		change := btcwire.NewOutPoint(txHash, 1)
		_, err = w.NewUncoloredOutPoint(b, change)
		if err != nil {
			return nil, err
		}
	}

	return cd, nil
}

func (w *Wallet) Send(b *gochroma.BlockExplorer, cd *gochroma.ColorDefinition, addrMap map[btcutil.Address]gochroma.ColorValue, fee int64) (*btcwire.MsgTx, error) {
	var colorId ColorId
	err := w.namespace.View(func(tx walletdb.Tx) error {
		var err error
		colorId, err = fetchColorId(tx, cd)
		return err
	})
	if err != nil {
		return nil, err
	}
	needed := gochroma.ColorValue(0)
	var outputs []*gochroma.ColorOut
	for addr, cv := range addrMap {
		needed += cv
		pkScript, err := btcscript.PayToAddrScript(addr)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, &gochroma.ColorOut{pkScript, cv})
	}

	coloredInputs, err := w.fetchSpendable(b, colorId, needed)
	if err != nil {
		return nil, err
	}

	var inputs []*gochroma.ColorIn
	inSum := gochroma.ColorValue(0)
	for _, ci := range coloredInputs {
		inSum += ci.ColorValue
		colorIn, err := ci.ColorIn()
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, colorIn)
	}

	coloredChangeNeeded := inSum > needed
	// see if we need colored change
	if coloredChangeNeeded {
		addr, err := w.NewColorAddress(cd)
		if err != nil {
			return nil, err
		}
		pkScript, err := btcscript.PayToAddrScript(addr)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, &gochroma.ColorOut{pkScript, inSum - needed})
	}
	uncoloredInputs, err := w.fetchSpendable(b, UncoloredColorId, gochroma.ColorValue(fee))
	if err != nil {
		return nil, err
	}
	for _, ui := range uncoloredInputs {
		colorOutPoint, err := ui.OutPoint()
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, &gochroma.ColorIn{colorOutPoint, gochroma.ColorValue(0)})
	}
	changeAddr, err := w.NewUncoloredAddress()
	if err != nil {
		return nil, err
	}
	changeScript, err := btcscript.PayToAddrScript(changeAddr)
	if err != nil {
		return nil, err
	}

	tx, err := cd.TransferringTx(b, inputs, outputs, changeScript, fee, false)

	if err != nil {
		return nil, err
	}

	// sign everything
	for i, in := range coloredInputs {
		err = w.Sign(in.PkScript, tx, i)
		if err != nil {
			return nil, err
		}
	}
	for i, in := range uncoloredInputs {
		err = w.Sign(in.PkScript, tx, i+len(coloredInputs))
		if err != nil {
			return nil, err
		}
	}

	txHash, err := b.PublishTx(tx)
	if err != nil {
		return nil, err
	}
	// mark everything as spent
	err = w.namespace.Update(func(tx walletdb.Tx) error {
		for i, in := range coloredInputs {
			in.Spent = true
			in.SpendingTx = gochroma.BigEndianBytes(txHash)
			in.SpendingIndex = uint32(i)
			err = storeOutPoint(tx, in)
			if err != nil {
				return err
			}

		}
		for i, in := range uncoloredInputs {
			in.Spent = true
			in.SpendingTx = gochroma.BigEndianBytes(txHash)
			in.SpendingIndex = uint32(i + len(coloredInputs))
			err = storeOutPoint(tx, in)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// if there's any change, add them back as outpoints
	// colored change
	if coloredChangeNeeded {
		change := btcwire.NewOutPoint(txHash, uint32(len(addrMap)))
		_, err = w.NewColorOutPoint(b, change, cd)
		if err != nil {
			return nil, err
		}
	}

	// uncolored change
	if len(tx.TxOut) > len(outputs) {
		change := btcwire.NewOutPoint(txHash, uint32(len(outputs)))
		_, err = w.NewUncoloredOutPoint(b, change)
		if err != nil {
			return nil, err
		}
	}

	return tx, nil
}
