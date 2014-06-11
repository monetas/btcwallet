/*
 * Copyright (c) 2013, 2014 Conformal Systems LLC <info@conformal.com>
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

// This file implements the websocket client connection to a bitcoin RPC
// server.

package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/monetas/btcjson"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcwire"
	"github.com/monetas/btcws"
	"io"
)

// ServerConn is an interface representing a client connection to a bitcoin RPC
// server.
type ServerConn interface {
	// SendRequest sends a bitcoin RPC request, returning a channel to
	// read the reply.  A channel is used so both synchronous and
	// asynchronous RPC can be supported.
	SendRequest(request *ServerRequest) chan RawRPCResponse
}

// ErrBtcdDisconnected describes an error where an operation cannot
// successfully complete due to btcwallet not being connected to
// btcd.
var ErrBtcdDisconnected = btcjson.Error{
	Code:    -1,
	Message: "btcd disconnected",
}

// ErrBtcdDisconnectedRaw is the raw JSON encoding of ErrBtcdDisconnected.
var ErrBtcdDisconnectedRaw = json.RawMessage(`{"code":-1,"message":"btcd disconnected"}`)

// BtcdRPCConn is a type managing a client connection to a btcd RPC server
// over websockets.
type BtcdRPCConn struct {
	ws         *websocket.Conn
	addRequest chan *AddRPCRequest
	closed     chan struct{}
}

// Ensure that BtcdRPCConn can be used as an RPCConn.
var _ ServerConn = &BtcdRPCConn{}

// NewBtcdRPCConn creates a new RPC connection from a btcd websocket
// connection to btcd.
func NewBtcdRPCConn(ws *websocket.Conn) *BtcdRPCConn {
	conn := &BtcdRPCConn{
		ws:         ws,
		addRequest: make(chan *AddRPCRequest),
		closed:     make(chan struct{}),
	}
	return conn
}

// SendRequest sends an RPC request and returns a channel to read the response's
// result and error.  Part of the RPCConn interface.
func (btcd *BtcdRPCConn) SendRequest(request *ServerRequest) chan RawRPCResponse {
	select {
	case <-btcd.closed:
		// The connection has closed, so instead of adding and sending
		// a request, return a channel that just replies with the
		// error for a disconnected btcd.
		responseChan := make(chan RawRPCResponse, 1)
		responseChan <- RawRPCResponse{Error: &ErrBtcdDisconnectedRaw}
		return responseChan

	default:
		addRequest := &AddRPCRequest{
			Request:      request,
			ResponseChan: make(chan chan RawRPCResponse, 1),
		}
		btcd.addRequest <- addRequest
		return <-addRequest.ResponseChan
	}
}

// Connected returns whether the connection remains established to the RPC
// server.
//
// This function probably should be removed, as any checks for confirming
// the connection are no longer valid after the check and may result in
// races.
func (btcd *BtcdRPCConn) Connected() bool {
	select {
	case <-btcd.closed:
		return false

	default:
		return true
	}
}

// Close forces closing the current btcd connection.
func (btcd *BtcdRPCConn) Close() {
	select {
	case <-btcd.closed:
	default:
		close(btcd.closed)
	}
}

// AddRPCRequest is used to add an RPCRequest to the pool of requests
// being manaaged by a btcd RPC connection.
type AddRPCRequest struct {
	Request      *ServerRequest
	ResponseChan chan chan RawRPCResponse
}

// send performs the actual send of the marshaled request over the btcd
// websocket connection.
func (btcd *BtcdRPCConn) send(rpcrequest *ServerRequest) error {
	// btcjson.Cmds define their own MarshalJSON which returns an error
	// to satisify the json.Marshaler interface, but should never error.
	// If an error does occur, it is due to a struct containing a type
	// that is not marshalable, so panic here rather than silently
	// ignoring it.
	mrequest, err := rpcrequest.request.MarshalJSON()
	if err != nil {
		panic(err)
	}
	return websocket.Message.Send(btcd.ws, mrequest)
}

// Start starts the goroutines required to send RPC requests and listen for
// replies.
func (btcd *BtcdRPCConn) Start() {
	done := btcd.closed
	responses := make(chan RawRPCResponse)

	// Maintain a map of JSON IDs to RPCRequests currently being waited on.
	go func() {
		m := make(map[uint64]*ServerRequest)
		for {
			select {
			case addrequest := <-btcd.addRequest:
				rpcrequest := addrequest.Request
				m[rpcrequest.request.Id().(uint64)] = rpcrequest

				if err := btcd.send(rpcrequest); err != nil {
					// Connection lost.
					log.Infof("Cannot complete btcd websocket send: %v",
						err)
					if err := btcd.ws.Close(); err != nil {
						log.Warnf("Cannot close btcd "+
							"websocket connection: %v", err)
					}
					close(done)
				}

				addrequest.ResponseChan <- rpcrequest.response

			case rawResponse, ok := <-responses:
				if !ok {
					responses = nil
					close(done)
					break
				}
				rpcrequest, ok := m[*rawResponse.Id]
				if !ok {
					log.Warnf("Received unexpected btcd response")
					continue
				}
				delete(m, *rawResponse.Id)

				rpcrequest.response <- rawResponse

			case <-done:
				resp := RawRPCResponse{Error: &ErrBtcdDisconnectedRaw}
				for _, request := range m {
					request.response <- resp
				}
				return
			}
		}
	}()

	// Listen for replies/notifications from btcd, and decide how to handle them.
	go func() {
		// Idea: instead of reading btcd messages from just one websocket
		// connection, maybe use two so the same connection isn't used
		// for both notifications and responses?  Should make handling
		// must faster as unnecessary unmarshal attempts could be avoided.

		for {
			var m string
			if err := websocket.Message.Receive(btcd.ws, &m); err != nil {
				// Log warning if btcd did not disconnect.
				if err != io.EOF {
					log.Infof("Cannot receive btcd websocket message: %v",
						err)
				}
				if err := btcd.ws.Close(); err != nil {
					log.Warnf("Cannot close btcd "+
						"websocket connection: %v", err)
				}
				close(responses)
				return
			}

			// Try notifications (requests with nil ids) first.
			n, err := unmarshalNotification(m)
			if err == nil {
				svrNtfns <- n
				continue
			}

			// Must be a response.
			r, err := unmarshalResponse(m)
			if err == nil {
				responses <- r
				continue
			}

			// Not sure what was received but it isn't correct.
			log.Warnf("Received invalid message from btcd")
		}
	}()
}

// unmarshalResponse attempts to unmarshal a marshaled JSON-RPC response.
func unmarshalResponse(s string) (RawRPCResponse, error) {
	var r RawRPCResponse
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		return r, err
	}

	// Check for a valid ID.
	if r.Id == nil {
		return r, errors.New("id is null")
	}
	return r, nil
}

// unmarshalNotification attempts to unmarshal a marshaled JSON-RPC
// notification (Request with a nil or no ID).
func unmarshalNotification(s string) (btcjson.Cmd, error) {
	req, err := btcjson.ParseMarshaledCmd([]byte(s))
	if err != nil {
		return nil, err
	}

	if req.Id() != nil {
		return nil, errors.New("id is non-nil")
	}

	return req, nil
}

// GetBestBlock gets both the block height and hash of the best block
// in the main chain.
func GetBestBlock(rpc ServerConn) (*btcws.GetBestBlockResult, error) {
	cmd := btcws.NewGetBestBlockCmd(<-NewJSONID)
	response := <-rpc.SendRequest(NewServerRequest(cmd))

	var resultData btcws.GetBestBlockResult
	if _, err := response.FinishUnmarshal(&resultData); err != nil {
		return nil, err
	}
	return &resultData, nil
}

// GetBlock requests details about a block with the given hash.
func GetBlock(rpc ServerConn, blockHash string) (*btcjson.BlockResult, error) {
	// NewGetBlockCmd should never fail with no optargs.  If this does fail,
	// panic now rather than later.
	cmd, err := btcjson.NewGetBlockCmd(<-NewJSONID, blockHash)
	if err != nil {
		panic(err)
	}
	response := <-rpc.SendRequest(NewServerRequest(cmd))

	var resultData btcjson.BlockResult
	if _, err := response.FinishUnmarshal(&resultData); err != nil {
		return nil, err
	}
	return &resultData, nil
}

// GetCurrentNet requests the network a bitcoin RPC server is running on.
func GetCurrentNet(rpc ServerConn) (btcwire.BitcoinNet, error) {
	cmd := btcws.NewGetCurrentNetCmd(<-NewJSONID)
	response := <-rpc.SendRequest(NewServerRequest(cmd))

	var resultData uint32
	if _, err := response.FinishUnmarshal(&resultData); err != nil {
		return 0, err
	}
	return btcwire.BitcoinNet(resultData), nil
}

// NotifyBlocks requests blockconnected and blockdisconnected notifications.
func NotifyBlocks(rpc ServerConn) error {
	cmd := btcws.NewNotifyBlocksCmd(<-NewJSONID)
	response := <-rpc.SendRequest(NewServerRequest(cmd))
	_, err := response.FinishUnmarshal(nil)
	return err
}

// NotifyReceived requests notifications for new transactions that spend
// to any of the addresses in addrs.
func NotifyReceived(rpc ServerConn, addrs []string) error {
	cmd := btcws.NewNotifyReceivedCmd(<-NewJSONID, addrs)
	response := <-rpc.SendRequest(NewServerRequest(cmd))
	_, err := response.FinishUnmarshal(nil)
	return err
}

// NotifySpent requests notifications for when a transaction is processed which
// spends op.
func NotifySpent(rpc ServerConn, outpoints []*btcwire.OutPoint) error {
	ops := make([]btcws.OutPoint, 0, len(outpoints))
	for _, op := range outpoints {
		ops = append(ops, *btcws.NewOutPointFromWire(op))
	}
	cmd := btcws.NewNotifySpentCmd(<-NewJSONID, ops)
	response := <-rpc.SendRequest(NewServerRequest(cmd))
	_, err := response.FinishUnmarshal(nil)
	return err
}

// Rescan requests a blockchain rescan for transactions to any number of
// addresses and notifications to inform wallet about such transactions.
func Rescan(rpc ServerConn, beginBlock int32, addrs []string,
	outpoints []*btcwire.OutPoint) error {

	ops := make([]btcws.OutPoint, 0, len(outpoints))
	for _, op := range outpoints {
		ops = append(ops, *btcws.NewOutPointFromWire(op))
	}
	// NewRescanCmd should never fail with no optargs.  If this does fail,
	// panic now rather than later.
	cmd, err := btcws.NewRescanCmd(<-NewJSONID, beginBlock, addrs, ops)
	if err != nil {
		panic(err)
	}
	response := <-rpc.SendRequest(NewServerRequest(cmd))
	_, err = response.FinishUnmarshal(nil)
	return err
}

// SendRawTransaction sends a hex-encoded transaction for relay.
func SendRawTransaction(rpc ServerConn, hextx string) (txid string, err error) {
	// NewSendRawTransactionCmd should never fail.  In the exceptional case
	// where it does, panic here rather than later.
	cmd, err := btcjson.NewSendRawTransactionCmd(<-NewJSONID, hextx)
	if err != nil {
		panic(err)
	}
	response := <-rpc.SendRequest(NewServerRequest(cmd))

	var resultData string
	_, err = response.FinishUnmarshal(&resultData)
	return resultData, err
}

// GetRawTransaction returns a future representing a pending GetRawTransaction
// command for txsha.. When the result of the request is required it may be
// collected with GetRawTRansactionAsyncResult.
func GetRawTransactionAsync(rpc ServerConn, txsha *btcwire.ShaHash) chan RawRPCResponse {
	// NewGetRawTransactionCmd should never fail with no optargs.  If this
	// does fail, panic now rather than later.
	cmd, err := btcjson.NewGetRawTransactionCmd(<-NewJSONID, txsha.String())
	if err != nil {
		panic(err)
	}
	return rpc.SendRequest(NewServerRequest(cmd))
}

// GetRawTransactionAsyncResult waits for the pending command in request -
// the reqsult of a previous GetRawTransactionAsync() call - and returns either
// the requested transaction, or an error.
func GetRawTransactionAsyncResult(request chan RawRPCResponse) (*btcutil.Tx, error) {
	response := <-request

	var resultData string
	_, err := response.FinishUnmarshal(&resultData)
	if err != nil {
		return nil, err
	}
	serializedTx, err := hex.DecodeString(resultData)
	if err != nil {
		return nil, btcjson.ErrDecodeHexString
	}
	utx, err := btcutil.NewTxFromBytes(serializedTx)
	if err != nil {
		return nil, btcjson.ErrDeserialization
	}
	return utx, nil
}

// GetRawTransaction sends the non-verbose version of a getrawtransaction
// request to receive the serialized transaction referenced by txsha.  If
// successful, the transaction is decoded and returned as a btcutil.Tx.
func GetRawTransaction(rpc ServerConn, txsha *btcwire.ShaHash) (*btcutil.Tx, error) {
	resp := GetRawTransactionAsync(rpc, txsha)
	return GetRawTransactionAsyncResult(resp)
}
