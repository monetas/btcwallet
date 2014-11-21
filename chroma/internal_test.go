package chroma_test

import (
	"encoding/hex"
	"fmt"

	"github.com/jimmysong/gochroma"
)

// example hashes and blocks that tests can use
var (
	blockHashStr = "00000000003583bc221e70c80ce8e3d67b49be70bb3b1fd6a191d2040babd3e8"

	txHashStr = "1d235c4ea39e7f3151e784283319485f4b5eb92e553ee6d307c0201b4125e09f"

	errHashStr = "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

	rawBlockStr = "020000009153031afe12d843b71b2a8a64ba0c516630e5fe34ee0a228d4b0400000000003f38188e708f2af4973972100e29b221c3c7c703ce12ad4c42d469aaf8267f2cc2e12e54c0ff3f1b1cc2312f0101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff2303057b04164b6e434d696e657242519dceb367fae996d0542ee1c200000000a0010000ffffffff0100f90295000000001976a9149e8985f82bc4e0f753d0492aa8d11cc39925774088ac00000000"

	rawBlockStr2 = "020000000548c8eb8c91c25c598f7bcb7e3d2f2f14971836c5796bb1023d1d0000000000836b81f78a4421c6bf663353fba5cf2a53d8ee3f76e4f47e96784e1ab1f3803dbee12e54c0ff3f1b995ba51c0101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff2503047b04184b6e434d696e657242519dceb367fae996d0542ee1be2b7d0000000009020000ffffffff0100f90295000000001976a9149e8985f82bc4e0f753d0492aa8d11cc39925774088ac00000000"

	normalTxStr = "0100000001aa570d9d285fe85030361b9704068b80bea89e49ad26079c2ecca8a555f8bbb8010000006c493046022100b09a37ead2637d8ffdbe2fb896a74a1c9e2f01ce306b24def2688cb7810ae609022100c019910aaf0a3317d4555441580bc5a5de6f7851d86e81aa854fef38debfefbc0121037843af5cf98718f57d6887f01d7b30bd0c6ed915eb6648ee30889861bd3a7feaffffffff0200e1f505000000001976a9149bbd3b6b3da61901454a9e3c0a22ac6c626cc0fa88ac32f8196f000000001976a9144d273d3a2ce1824d1c6db0764eebb03f368fd9af88ac00000000"

	genesisTxStr = "01000000011a932892802d8e1657bdc84feb3663a38ea64c33b0f5436606309d6f610a01fd000000006b483045022100d1fdca93b2074caf8fe329babe0472d381721384f183566cdf7ea34e8522df3402203e0176301ef6a192bccf94ae1c5ed50df67e1ff2227fb35b6da6adafbcc1321901210277d7813a44ee7325b9cdffd22e9b0f44ad3b5b0433cc69853c21cc7e6ebeb503ffffffff0210270000000000001976a914143caef14f63625b633b77dedac55f9deaedae6088acf0908800000000001976a9147d83495938585f3f9e01cfb2137f94b0f0f2ce2588ac00000000"

	specialTxStr = "0100000001aa570d9d285fe85030361b9704068b80bea89e49ad26079c2ecca8a555f8bbb8010000006c493046022100b09a37ead2637d8ffdbe2fb896a74a1c9e2f01ce306b24def2688cb7810ae609022100c019910aaf0a3317d4555441580bc5a5de6f7851d86e81aa854fef38debfefbc0121037843af5cf98718f57d6887f01d7b30bd0c6ed915eb6648ee30889861bd3a7feaffffffff0200e1f505000000001976a914fad24d236a111ca5b5838ab85db2c4bdaab7c3be88ac32f8196f000000001976a9144d273d3a2ce1824d1c6db0764eebb03f368fd9af88ac00000000"

	niceTxStr = "0100000001aa570d9d285fe85030361b9704068b80bea89e49ad26079c2ecca8a555f8bbb8010000006c493046022100b09a37ead2637d8ffdbe2fb896a74a1c9e2f01ce306b24def2688cb7810ae609022100c019910aaf0a3317d4555441580bc5a5de6f7851d86e81aa854fef38debfefbc0121037843af5cf98718f57d6887f01d7b30bd0c6ed915eb6648ee30889861bd3a7feaffffffff0210270000000000001976a914fad24d236a111ca5b5838ab85db2c4bdaab7c3be88ac32f8196f000000001976a9144d273d3a2ce1824d1c6db0764eebb03f368fd9af88ac00000000"
)
var (
	blockHash []byte
	txHash    []byte
	errHash   []byte
	rawBlock  []byte
	rawBlock2 []byte
	normalTx  []byte
	genesisTx []byte
	specialTx []byte
	niceTx    []byte
)

func init() {
	var err error
	blockHash, err = hex.DecodeString(blockHashStr)
	if err != nil {
		fmt.Errorf("failed to convert string to bytes :%v\n", err)
	}
	txHash, err = hex.DecodeString(txHashStr)
	if err != nil {
		fmt.Errorf("failed to convert string to bytes :%v\n", err)
	}
	errHash, err = hex.DecodeString(errHashStr)
	if err != nil {
		fmt.Errorf("failed to convert string to bytes :%v\n", err)
	}
	rawBlock, err = hex.DecodeString(rawBlockStr)
	if err != nil {
		fmt.Errorf("failed to convert string to bytes :%v\n", err)
	}
	rawBlock2, err = hex.DecodeString(rawBlockStr2)
	if err != nil {
		fmt.Errorf("failed to convert string to bytes :%v\n", err)
	}
	normalTx, err = hex.DecodeString(normalTxStr)
	if err != nil {
		fmt.Errorf("failed to convert string to bytes :%v\n", err)
	}
	genesisTx, err = hex.DecodeString(genesisTxStr)
	if err != nil {
		fmt.Errorf("failed to convert string to bytes :%v\n", err)
	}
	specialTx, err = hex.DecodeString(specialTxStr)
	if err != nil {
		fmt.Errorf("failed to convert string to bytes :%v\n", err)
	}
	niceTx, err = hex.DecodeString(niceTxStr)
	if err != nil {
		fmt.Errorf("failed to convert string to bytes :%v\n", err)
	}
}

// Test block read/writer that returns whatever you initialize with.
// Note each slice is a queue that shifts one off and throws an error
// if there's none left.
type TstBlockReaderWriter struct {
	blockCount  []int64
	blockHash   [][]byte
	block       [][]byte
	rawTx       [][]byte
	txBlockHash [][]byte
	mempoolTxs  [][][]byte
	txOutSpents []bool
	sendHash    [][]byte
}

func (b *TstBlockReaderWriter) BlockCount() (int64, error) {
	if len(b.blockCount) == 0 {
		return 0, gochroma.MakeError(gochroma.ErrBlockRead, "Block Error", nil)
	}
	ret := b.blockCount[0]
	b.blockCount = b.blockCount[1:]
	return ret, nil
}

func (b *TstBlockReaderWriter) BlockHash(_ int64) ([]byte, error) {

	if len(b.blockHash) == 0 {
		return nil, gochroma.MakeError(gochroma.ErrBlockRead, "BlockHash Error", nil)
	}
	ret := b.blockHash[0]
	b.blockHash = b.blockHash[1:]
	return ret, nil
}

func (b *TstBlockReaderWriter) RawBlock(_ []byte) ([]byte, error) {
	if len(b.block) == 0 {
		return nil, gochroma.MakeError(gochroma.ErrBlockRead, "RawBlock Error", nil)
	}
	ret := b.block[0]
	b.block = b.block[1:]
	return ret, nil
}

func (b *TstBlockReaderWriter) RawTx(_ []byte) ([]byte, error) {
	if len(b.rawTx) == 0 {
		return nil, gochroma.MakeError(gochroma.ErrBlockRead, "RawTx Error", nil)
	}
	ret := b.rawTx[0]
	b.rawTx = b.rawTx[1:]
	return ret, nil
}

func (b *TstBlockReaderWriter) TxBlockHash(_ []byte) ([]byte, error) {
	if len(b.txBlockHash) == 0 {
		return nil, gochroma.MakeError(gochroma.ErrBlockRead, "TxBlockHash Error", nil)
	}
	ret := b.txBlockHash[0]
	b.txBlockHash = b.txBlockHash[1:]
	return ret, nil
}

func (b *TstBlockReaderWriter) MempoolTxs() ([][]byte, error) {
	if len(b.mempoolTxs) == 0 {
		return nil, gochroma.MakeError(gochroma.ErrBlockRead, "MempoolTxs Error", nil)
	}
	ret := b.mempoolTxs[0]
	b.mempoolTxs = b.mempoolTxs[1:]
	return ret, nil
}

func (b *TstBlockReaderWriter) TxOutSpent(_ []byte, _ uint32, _ bool) (*bool, error) {
	if len(b.txOutSpents) == 0 {
		return nil, gochroma.MakeError(gochroma.ErrBlockRead, "TxOutSpent Error", nil)
	}
	ret := b.txOutSpents[0]
	b.txOutSpents = b.txOutSpents[1:]
	return &ret, nil
}

func (b *TstBlockReaderWriter) PublishRawTx(_ []byte) ([]byte, error) {
	if len(b.sendHash) == 0 {
		return nil, gochroma.MakeError(gochroma.ErrBlockWrite, "PublishRawTx Error", nil)
	}
	ret := b.sendHash[0]
	b.sendHash = b.sendHash[1:]
	return ret, nil
}
