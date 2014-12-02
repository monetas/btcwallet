package chroma

import (
	"fmt"
)

const (
	ErrUnimplemented = iota
	ErrSerialization
	ErrReadDB
	ErrWriteDB
	ErrHDKey
	ErrCreateBucket
	ErrAcct
	ErrScript
	ErrColorOutPoint
	ErrOutPointExists
	ErrBlockExplorer
	ErrColor
	ErrSpend
	ErrAddress
	ErrShaHash
)

type ErrorCode int

var errCodeStrings = map[ErrorCode]string{
	ErrUnimplemented:  "unimplemented",
	ErrSerialization:  "Error serializing or deserializing",
	ErrReadDB:         "Error reading to database",
	ErrWriteDB:        "Error writing to database",
	ErrHDKey:          "Error with hd key",
	ErrCreateBucket:   "Error creating bucket",
	ErrAcct:           "Error with account",
	ErrScript:         "Error with script",
	ErrColorOutPoint:  "Error with color out point",
	ErrOutPointExists: "Out Point exists",
	ErrBlockExplorer:  "Error with block explorer",
	ErrColor:          "Error in color library",
	ErrSpend:          "Error trying to spend",
	ErrAddress:        "Error with address",
	ErrShaHash:        "Error with the hash",
}

func (e ErrorCode) String() string {
	s, ok := errCodeStrings[e]
	if ok {
		return s
	} else {
		return fmt.Sprintf("Unknown ErrorCode: %d", int(e))
	}
}

type ChromaError struct {
	ErrorCode   ErrorCode
	Description string
	Err         error
}

func (e ChromaError) Error() string {
	if e.Err != nil {
		return e.ErrorCode.String() + " " + e.Description + ": " + e.Err.Error()
	}
	return e.ErrorCode.String() + " " + e.Description
}

func MakeError(c ErrorCode, d string, e error) ChromaError {
	return ChromaError{
		ErrorCode:   c,
		Description: d,
		Err:         e,
	}
}
