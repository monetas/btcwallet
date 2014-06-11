/************************************************
TEMPORARY FILE! SHOULD BE DELETED EVENTUALLY

The functions in this file are to be used
until the HD Wallet is fully integrated into
btcwallet.
*************************************************/

package main

import (
	"fmt"

	"github.com/monetas/btcwallet/waddrmgr"
)

const (
	tempLocation = "manager.bin"
)

var (
	pubPassphrase  = []byte("public")
	privPassphrase = []byte("private")
	seed           = make([]byte, 32)
)

// creates a manager, prints error if something goes wrong
func TempManager() *waddrmgr.Manager {
	manager, err := waddrmgr.Create(tempLocation, seed, pubPassphrase, privPassphrase, activeNet.Params, nil)
	if err != nil {
		manager, err = waddrmgr.Open(tempLocation, pubPassphrase, activeNet.Params, nil)
		if err != nil {
			fmt.Printf("Error while creating HD Wallet Manager: %v\n", err)
			return nil
		}
	}
	fmt.Printf("successfully got the manager\n")
	return manager
}
