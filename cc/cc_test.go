// This is an integration test for showing how to use IFOC
// kernel with btcwallet. The smart property gets issued and
// goes through three different addresses.

package cc_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/monetas/btcnet"
	"github.com/monetas/btcrpcclient"
	"github.com/monetas/btcscript"
	"github.com/monetas/btcwallet/cc"
	"github.com/monetas/btcwallet/waddrmgr"
	"github.com/monetas/btcwire"
	"github.com/monetas/gochroma"
)

var (
	pubPassphrase  = []byte("public")
	privPassphrase = []byte("private")
	seed           = make([]byte, 32)
)

var _ = spew.Dump

func setUp(t *testing.T) (*gochroma.BlockExplorer, []*cc.NiceAddr, func()) {
	// create some temporary files and directories on the system
	systemTmp := os.TempDir()
	mgrTmp, err := ioutil.TempDir(systemTmp, "cc_test")
	if err != nil {
		t.Fatalf("failed to create temporary dir at %v", systemTmp)
	}
	tempLocation := filepath.Join(mgrTmp, "manager.bin")
	btcd1Tmp, err := ioutil.TempDir(systemTmp, "cc_test")
	if err != nil {
		t.Fatalf("failed to create temporary dir at %v", systemTmp)
	}
	btcd2Tmp, err := ioutil.TempDir(systemTmp, "cc_test")
	if err != nil {
		t.Fatalf("failed to create temporary dir at %v", systemTmp)
	}

	// create some addresses to work with
	net := &btcnet.SimNetParams
	manager, err := waddrmgr.Create(
		tempLocation, seed, pubPassphrase, privPassphrase, net)
	if err != nil {
		t.Fatalf("Error while creating HD Wallet Manager: %v\n", err)
	}
	manager.Unlock(privPassphrase)
	acct := uint32(0)
	addrsTmp, err := manager.NextExternalAddresses(acct, 5)
	addrs := make([]*cc.NiceAddr, 5)

	for i, addr := range addrsTmp {
		pkAddr := addr.(waddrmgr.ManagedPubKeyAddress)
		pkScript, err := btcscript.PayToAddrScript(pkAddr.Address())
		if err != nil {
			t.Fatalf("Error while deriving pkscript: %v\n", err)
		}
		key, err := pkAddr.PrivKey()
		if err != nil {
			t.Fatalf("Error while getting private key: %v\n", err)
		}
		addrs[i] = &cc.NiceAddr{pkAddr.Address().String(), pkScript, key}
	}

	// run btcd so that it starts mining
	path := os.Getenv("GOPATH")
	btcd := filepath.Join(path, "bin", "btcd")
	btcctl := filepath.Join(path, "bin", "btcctl")

	btcd1RPCPort := "47321"
	btcd1Port := "47311"
	btcd1 := exec.Command(btcd, "--datadir="+btcd1Tmp+"/",
		"--logdir="+btcd1Tmp+"/", "--debuglevel=debug", "--simnet",
		"--listen=:"+btcd1Port, "--rpclisten=:"+btcd1RPCPort, "--rpcuser=user",
		"--rpcpass=pass", "--rpccert=rpc.cert", "--rpckey=rpc.key")
	btcd2RPCPort := "47322"
	btcd2Port := "47312"
	miningStr := fmt.Sprintf("--miningaddr=%v", addrs[0].Addr)
	btcd2 := exec.Command(btcd, "--datadir="+btcd2Tmp+"/",
		"--logdir="+btcd2Tmp+"/", "--debuglevel=debug", "--simnet",
		"--listen=:"+btcd2Port, "--rpclisten=:"+btcd2RPCPort,
		"--connect=localhost:"+btcd1Port, "--rpcuser=user", "--rpcpass=pass",
		"--rpccert=rpc.cert", "--rpckey=rpc.key", "--generate", miningStr)

	if err = btcd1.Start(); err != nil {
		t.Fatal("failed to run btcd1")
	}
	if err = btcd2.Start(); err != nil {
		t.Fatal("failed to run btcd2")
	}
	time.Sleep(1000 * time.Millisecond)

	// Now make a connection to the btcd for colored coins
	certs, err := ioutil.ReadFile("rpc.cert")
	if err != nil {
		t.Fatalf("%v\n", err)
	}
	connConfig := &btcrpcclient.ConnConfig{
		Host:         "localhost:" + btcd2RPCPort,
		User:         "user",
		Pass:         "pass",
		Certificates: certs,
		HttpPostMode: true,
	}
	blockReaderWriter, err := gochroma.NewBtcdBlockExplorer(net, connConfig)
	if err != nil {
		t.Fatalf("%v\n", err)
	}
	b := &gochroma.BlockExplorer{blockReaderWriter}

	tearDown := func() {
		exec.Command(btcctl, "-C", "btcctl2.conf", "stop").Output()
		exec.Command(btcctl, "-C", "btcctl1.conf", "stop").Output()
		os.RemoveAll(btcd1Tmp)
		os.RemoveAll(btcd2Tmp)
		os.RemoveAll(mgrTmp)
	}

	return b, addrs, tearDown
}

func TestCC(t *testing.T) {

	b, addrs, tearDown := setUp(t)
	defer tearDown()

	// grab an outpoint we can sign (we're mining to the first addr)
	block, err := b.BlockAtHeight(1)
	if err != nil {
		t.Fatalf("%v\n", err)
	}
	coinbase := block.Transactions()[0]
	// designate the mining address as our change address
	changeAddr := addrs[0]

	// grab the kernel
	ifoc, err := gochroma.GetColorKernel("IFOC")
	if err != nil {
		t.Fatalf("error getting ifoc kernel: %v", err)
	}

	// Create the issuing tx
	inputs := []*btcwire.OutPoint{&btcwire.OutPoint{*coinbase.Sha(), 0}}
	outputs := []*gochroma.ColorOut{&gochroma.ColorOut{addrs[1].PkScript, 1}}
	tx, err := ifoc.IssuingTx(b, inputs, outputs, changeAddr.PkScript, 10000)
	if err != nil {
		t.Fatalf("failure issuing: %v", err)
	}
	if err = changeAddr.Sign(tx, 0); err != nil {
		t.Fatalf("cannot sign: %v", err)
	}

	// Submit the tx to the BlockReaderWriter
	shaHash, err := b.PublishTx(tx)
	if err != nil {
		t.Fatalf("cannot publish: %s", err)
	}
	genesis := btcwire.NewOutPoint(shaHash, 0)
	currentOut := genesis

	// check that the color value of the outpoint is what we expect
	colorIn, err := ifoc.OutPointToColorIn(b, genesis, currentOut)
	if err != nil {
		t.Fatalf("cannot get color value: %v, %v", shaHash, err)
	}
	if colorIn.ColorValue != 1 {
		t.Fatalf("wrong color value: %v", colorIn)
	}

	// now that it's issued, we can make a color definition
	height, err := b.TxHeight(gochroma.BigEndianBytes(shaHash))
	if err != nil {
		t.Fatalf("cannot get height: %v", err)
	}
	color, err := gochroma.NewColorDefinition(ifoc, genesis, height)
	if err != nil {
		t.Fatalf("cannot make def %v", err)
	}

	// Hop 3 more times
	for i := 1; i < 4; i++ {
		// Create a transferring tx
		currentIn := &gochroma.ColorIn{currentOut, 1}
		uncoloredIn := &gochroma.ColorIn{btcwire.NewOutPoint(shaHash, 1), 0}
		ins := []*gochroma.ColorIn{currentIn, uncoloredIn}
		outs := []*gochroma.ColorOut{&gochroma.ColorOut{addrs[i+1].PkScript, 1}}
		tx, err = color.TransferringTx(
			b, ins, outs, changeAddr.PkScript, 10000, false)
		if err != nil {
			t.Fatalf("cannot create transferring tx: %s", err)
		}
		// add an arbitrary OP_RETURN (0x6a = OP_RETURN)
		tx.AddTxOut(btcwire.NewTxOut(0, []byte{0x6a, 0x01, byte(i)}))

		// Sign the transferring tx
		if err = addrs[i].Sign(tx, 0); err != nil {
			t.Fatalf("cannot sign tx: %s", err)
		}
		if err = changeAddr.Sign(tx, 1); err != nil {
			t.Fatalf("cannot sign tx: %s", err)
		}

		// Submit the tx to the BlockReaderWriter
		shaHash, err = b.PublishTx(tx)
		if err != nil {
			t.Fatalf("cannot publish: %v", err)
		}

		// old endpoint should have a value of 0
		cv, err := color.ColorValue(b, currentOut)
		if err != nil {
			t.Fatalf("cannot get color value: %s", err)
		}
		if *cv != 0 {
			t.Fatalf("wrong color value: %v", colorIn)
		}
		// new endpoint should have a value of 1
		currentOut = btcwire.NewOutPoint(shaHash, 0)
		cv, err = color.ColorValue(b, currentOut)
		if err != nil {
			t.Fatalf("cannot get color value: %s", err)
		}
		if *cv != 1 {
			t.Fatalf("wrong color value: %v", colorIn)
		}
	}
}
