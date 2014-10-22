// Utility package for making signing easier

package cc

import (
	"errors"
	"fmt"

	"github.com/monetas/btcec"
	"github.com/monetas/btcscript"
	"github.com/monetas/btcwire"
)

type NiceAddr struct {
	Addr     string
	PkScript []byte
	Key      *btcec.PrivateKey
}

func (na *NiceAddr) Sign(tx *btcwire.MsgTx, index int) error {
	sigScript, err := btcscript.SignatureScript(
		tx, index, na.PkScript, btcscript.SigHashAll, na.Key, true)
	if err != nil {
		str := fmt.Sprintf("cannot create sigScript: %s", err)
		return errors.New(str)
	}
	tx.TxIn[index].SignatureScript = sigScript
	return nil
}
