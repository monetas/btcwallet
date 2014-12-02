package chroma_test

import (
	"errors"
	"testing"

	"github.com/monetas/btcwallet/chroma"
)

func TestString1(t *testing.T) {
	// execute
	s := chroma.ErrorCode(0).String()

	// validate
	want := "unimplemented"
	if s != want {
		t.Fatalf("want %s, got %s", want, s)
	}
}

func TestString2(t *testing.T) {
	// execute
	s := chroma.ErrorCode(9000).String()

	// validate
	want := "Unknown ErrorCode: 9000"
	if s != want {
		t.Fatalf("want %s, got %s", want, s)
	}
}

func TestError1(t *testing.T) {
	// execute
	e := chroma.MakeError(0, "some error", nil)

	// validate
	got := e.Error()
	want := "unimplemented some error"
	if got != want {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestError2(t *testing.T) {
	// execute
	s := "some error"
	e := chroma.MakeError(0, s, errors.New("test"))

	// validate
	got := e.Error()
	want := "unimplemented " + s + ": " + "test"
	if got != want {
		t.Fatalf("want %s, got %s", want, got)
	}
}
