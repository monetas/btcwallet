package chroma

import (
	"bytes"
	"errors"

	"github.com/monetas/btcwire"
	"github.com/monetas/gochroma"
)

var (
	UncoloredColorId = []byte{0, 0, 0, 0}
)

type ColorOutPoint struct {
	Id            OutPointId
	Tx            []byte
	Index         uint32
	Value         uint64
	Color         ColorId
	ColorValue    gochroma.ColorValue
	SpendingTx    []byte
	SpendingIndex uint32
	Spent         bool
	PkScript      []byte
}

func (cop *ColorOutPoint) IsUncolored() bool {
	return bytes.Compare(cop.Color, UncoloredColorId) == 0
}

func (cop *ColorOutPoint) OutPoint() (*btcwire.OutPoint, error) {
	shaHash, err := gochroma.NewShaHash(cop.Tx)
	if err != nil {
		return nil, err
	}
	return btcwire.NewOutPoint(shaHash, cop.Index), nil
}

func (cop *ColorOutPoint) ColorIn() (*gochroma.ColorIn, error) {
	if cop.IsUncolored() {
		return nil, errors.New("Cannot make an uncolored out point into a color in")
	}
	outPoint, err := cop.OutPoint()
	if err != nil {
		return nil, err
	}

	return &gochroma.ColorIn{outPoint, cop.ColorValue}, nil
}
