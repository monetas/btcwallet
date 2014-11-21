package chroma_test

import (
	"fmt"
	"testing"

	"github.com/monetas/btcwallet/chroma"
	"github.com/monetas/gochroma"
)

var uncoloredCOP = &chroma.ColorOutPoint{
	Id:            chroma.OutPointId([]byte{1}),
	Tx:            []byte("fake transactionfake transaction"),
	Index:         uint32(0),
	Value:         uint64(10000),
	Color:         chroma.ColorId([]byte{0, 0, 0, 0}),
	ColorValue:    gochroma.ColorValue(100),
	SpendingTx:    nil,
	SpendingIndex: 0,
	Spent:         false,
	PkScript:      []byte{1},
}

var coloredCOP = &chroma.ColorOutPoint{
	Id:            chroma.OutPointId([]byte{1}),
	Tx:            []byte("blahblahblahblahblahblahblahblah"),
	Index:         uint32(0),
	Value:         uint64(100000),
	Color:         chroma.ColorId([]byte{1, 0, 0, 0}),
	ColorValue:    gochroma.ColorValue(10),
	SpendingTx:    nil,
	SpendingIndex: 0,
	Spent:         false,
	PkScript:      []byte{1},
}

func TestIsUncolored(t *testing.T) {
	// execute
	got := uncoloredCOP.IsUncolored()

	// validate
	if got != true {
		t.Fatalf("unexpected color: want %v, got %v", true, got)
	}
}

func TestOutPoint(t *testing.T) {
	// execute
	op, err := uncoloredCOP.OutPoint()

	// validate
	if err != nil {
		t.Fatalf("failed on outpoint: %v", err)
	}
	got := op.Hash.String()
	want := fmt.Sprintf("%x", uncoloredCOP.Tx)
	if got != want {
		t.Fatalf("shahash not what we expected: want %v, got %v", want, got)
	}
}

func TestColorInErr(t *testing.T) {
	// execute
	_, err := uncoloredCOP.ColorIn()

	//validate
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestColorIn(t *testing.T) {
	// execute
	ci, err := coloredCOP.ColorIn()

	//validate
	if err != nil {
		t.Fatalf("color in failed with: %v", err)
	}
	got := ci.OutPoint.Hash.String()
	want := fmt.Sprintf("%x", coloredCOP.Tx)
	if got != want {
		t.Fatalf("shahash not what we expected: want %v, got %v", want, got)
	}
}
