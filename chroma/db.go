package chroma

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"

	"github.com/monetas/btcscript"
	"github.com/monetas/btcutil"
	"github.com/monetas/btcutil/hdkeychain"
	"github.com/monetas/btcwallet/walletdb"
	"github.com/monetas/btcwire"
	"github.com/monetas/gochroma"
)

var (
	keyBucketName                  = []byte("keys")
	idBucketName                   = []byte("id counter")
	accountBucketName              = []byte("account info")
	colorDefinitionBucketName      = []byte("color definitions")
	colorOutPointBucketName        = []byte("color outpoints")
	outPointIndexBucketName        = []byte("out point index")
	scriptToAccountIndexBucketName = []byte("script account index")

	colorIdKey    = []byte("color id")
	outPointIdKey = []byte("out point id")
	pubKeyName    = []byte("extended pubkey")
	privKeyName   = []byte("encrypted extended privkey")
	netName       = []byte("net")

	uncoloredAcctNum = uint32(0)
	issuingAcctNum   = uint32(1)
)

type ColorId []byte
type OutPointId []byte

func serializeUint32(i uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, i)
	return buf
}

func deserializeUint32(b []byte) uint32 {
	return binary.LittleEndian.Uint32(b)
}

func serializeOutPoint(op *btcwire.OutPoint) []byte {
	return append(op.Hash.Bytes(), serializeUint32(op.Index)...)
}

func serializeColorOutPoint(cop *ColorOutPoint) ([]byte, error) {
	// nil cop will cause a panic, so handle it here
	if cop == nil {
		return nil, MakeError(ErrSerialization, "nil", nil)
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	enc.Encode(cop)
	return buf.Bytes(), nil
}

func deserializeColorOutPoint(b []byte) (*ColorOutPoint, error) {
	var buf bytes.Buffer
	buf.Write(b)
	dec := gob.NewDecoder(&buf)
	var cop ColorOutPoint
	err := dec.Decode(&cop)
	if err != nil {
		str := fmt.Sprintf("%x", b)
		return nil, MakeError(ErrSerialization, str, err)
	}
	return &cop, nil
}

func increment(id []byte) []byte {
	x := deserializeUint32(id) + 1
	return serializeUint32(x)
}

func currentId(tx walletdb.Tx, idKey []byte) []byte {
	bucket := tx.RootBucket().Bucket(idBucketName)
	id := bucket.Get(idKey)
	if len(id) == 0 {
		return serializeUint32(uint32(1))
	}
	return id
}

func newId(tx walletdb.Tx, idKey []byte) ([]byte, error) {
	id := currentId(tx, idKey)
	bucket := tx.RootBucket().Bucket(idBucketName)
	err := bucket.Put(idKey, increment(id))
	if err != nil {
		str := fmt.Sprintf("%v:%x", string(idKey), id)
		return nil, MakeError(ErrWriteDB, str, err)
	}
	return id, nil
}

func newColorId(tx walletdb.Tx) (ColorId, error) {
	return newId(tx, colorIdKey)
}

func newOutPointId(tx walletdb.Tx) (OutPointId, error) {
	return newId(tx, outPointIdKey)
}

func fetchColorId(tx walletdb.Tx, cd *gochroma.ColorDefinition) (ColorId, error) {
	// check to see if it's already in db
	bucket := tx.RootBucket().Bucket(colorDefinitionBucketName)

	var colorId ColorId
	colorId = bucket.Get([]byte(cd.HashString()))
	if len(colorId) != 0 {
		return colorId, nil
	}

	// grab a new color id
	colorId, err := newColorId(tx)
	if err != nil {
		return nil, err
	}
	err = bucket.Put([]byte(cd.HashString()), colorId)
	if err != nil {
		str := fmt.Sprintf("%v:%x", cd.HashString(), colorId)
		return nil, MakeError(ErrWriteDB, str, err)
	}

	// add this color to the proper account
	b2 := tx.RootBucket().Bucket(accountBucketName)
	err = b2.Put(serializeUint32(cd.AccountNumber()), serializeUint32(0))
	if err != nil {
		str := fmt.Sprintf("%v:%x", cd.AccountNumber(), serializeUint32(0))
		return nil, MakeError(ErrWriteDB, str, err)
	}

	return colorId, nil
}

func fetchOutPointId(tx walletdb.Tx, outPoint *btcwire.OutPoint) OutPointId {
	// check to see if it's already in db
	bucket := tx.RootBucket().Bucket(outPointIndexBucketName)
	serializedOutPoint := serializeOutPoint(outPoint)
	outPointId := bucket.Get(serializedOutPoint)
	if len(outPointId) != 0 {
		return outPointId
	}
	return nil
}

func allColors(tx walletdb.Tx) (map[*gochroma.ColorDefinition]ColorId, error) {
	maxColorId := currentId(tx, colorIdKey)
	numColors := deserializeUint32(maxColorId) - 1
	cds := make(map[*gochroma.ColorDefinition]ColorId, int(numColors))

	bucket := tx.RootBucket().Bucket(colorDefinitionBucketName)
	err := bucket.ForEach(
		func(k, v []byte) error {
			cd, err := gochroma.NewColorDefinitionFromStr(string(k) + ":0")
			if err != nil {
				return MakeError(ErrColor, string(k)+":0", err)
			}
			cds[cd] = v
			return nil
		})
	if err != nil {
		return nil, err
	}
	return cds, nil
}

func fetchKeys(tx walletdb.Tx) (*hdkeychain.ExtendedKey, *hdkeychain.ExtendedKey, error) {
	b := tx.RootBucket().Bucket(keyBucketName)
	privStr := string(b.Get(privKeyName))
	priv, err := hdkeychain.NewKeyFromString(privStr)
	if err != nil {
		return nil, nil, MakeError(ErrHDKey, privStr, err)
	}
	pubStr := string(b.Get(pubKeyName))
	pub, err := hdkeychain.NewKeyFromString(pubStr)
	if err != nil {
		return nil, nil, MakeError(ErrHDKey, pubStr, err)
	}
	return priv, pub, nil
}

func initialize(tx walletdb.Tx, seed []byte) error {
	var err error
	if seed == nil {
		seed, err = hdkeychain.GenerateSeed(32)
		if err != nil {
			return MakeError(ErrHDKey, "seed", err)
		}
	}
	// get the hd root
	priv, err := hdkeychain.NewMaster(seed)
	if err != nil {
		return MakeError(ErrHDKey, "master", err)
	}
	pub, err := priv.Neuter()
	if err != nil {
		return MakeError(ErrHDKey, "private", err)
	}

	b, err := tx.RootBucket().CreateBucket(keyBucketName)
	if err != nil {
		return MakeError(ErrCreateBucket, string(keyBucketName), err)
	}
	err = b.Put(privKeyName, []byte(priv.String()))
	if err != nil {
		str := fmt.Sprintf("%v:%x", string(privKeyName), []byte(priv.String()))
		return MakeError(ErrWriteDB, str, err)
	}
	err = b.Put(pubKeyName, []byte(pub.String()))
	if err != nil {
		str := fmt.Sprintf("%v:%x", string(pubKeyName), []byte(pub.String()))
		return MakeError(ErrWriteDB, str, err)
	}
	_, err = tx.RootBucket().CreateBucket(idBucketName)
	if err != nil {
		return MakeError(ErrCreateBucket, string(idBucketName), err)
	}
	b, err = tx.RootBucket().CreateBucket(accountBucketName)
	if err != nil {
		return MakeError(ErrCreateBucket, string(accountBucketName), err)
	}
	err = b.Put(serializeUint32(uncoloredAcctNum), serializeUint32(0))
	if err != nil {
		str := fmt.Sprintf("%v:%x", uncoloredAcctNum, serializeUint32(0))
		return MakeError(ErrWriteDB, str, err)
	}
	err = b.Put(serializeUint32(issuingAcctNum), serializeUint32(0))
	if err != nil {
		str := fmt.Sprintf("%v:%x", issuingAcctNum, serializeUint32(0))
		return MakeError(ErrWriteDB, str, err)
	}
	_, err = tx.RootBucket().CreateBucket(colorDefinitionBucketName)
	if err != nil {
		return MakeError(ErrCreateBucket, string(colorDefinitionBucketName), err)
	}
	_, err = tx.RootBucket().CreateBucket(colorOutPointBucketName)
	if err != nil {
		return MakeError(ErrCreateBucket, string(colorOutPointBucketName), err)
	}
	_, err = tx.RootBucket().CreateBucket(outPointIndexBucketName)
	if err != nil {
		return MakeError(ErrCreateBucket, string(outPointIndexBucketName), err)
	}
	_, err = tx.RootBucket().CreateBucket(scriptToAccountIndexBucketName)
	if err != nil {
		return MakeError(ErrCreateBucket, string(scriptToAccountIndexBucketName), err)
	}
	return nil
}

func fetchAcctIndex(tx walletdb.Tx, acct uint32) (*uint32, error) {
	b := tx.RootBucket().Bucket(accountBucketName)
	raw := b.Get(serializeUint32(acct))
	if len(raw) == 0 {
		str := fmt.Sprintf("%v", acct)
		return nil, MakeError(ErrAcct, str, nil)
	}
	index := deserializeUint32(raw)
	return &index, nil
}

func storeAcctIndex(tx walletdb.Tx, acct, index uint32) error {
	b := tx.RootBucket().Bucket(accountBucketName)
	err := b.Put(serializeUint32(acct), serializeUint32(index))
	if err != nil {
		str := fmt.Sprintf("%v:%x", acct, serializeUint32(index))
		return MakeError(ErrWriteDB, str, err)
	}
	return nil
}

func storeScriptIndex(tx walletdb.Tx, acct, index uint32, addr btcutil.Address) error {
	b := tx.RootBucket().Bucket(scriptToAccountIndexBucketName)
	val := append(serializeUint32(acct), serializeUint32(index)...)
	pkScript, err := btcscript.PayToAddrScript(addr)
	if err != nil {
		str := fmt.Sprintf("%v", addr)
		return MakeError(ErrScript, str, err)
	}
	err = b.Put(pkScript, val)
	if err != nil {
		str := fmt.Sprintf("%x:%x", pkScript, val)
		return MakeError(ErrWriteDB, str, err)
	}
	return nil
}
func storeColorOutPoint(tx walletdb.Tx, cop *ColorOutPoint) error {
	s, err := serializeColorOutPoint(cop)
	if err != nil {
		return err
	}
	b := tx.RootBucket().Bucket(colorOutPointBucketName)
	err = b.Put(cop.Id, s)
	if err != nil {
		str := fmt.Sprintf("%v:%x", string(cop.Id), s)
		return MakeError(ErrWriteDB, str, err)
	}
	b = tx.RootBucket().Bucket(outPointIndexBucketName)
	outPoint, err := cop.OutPoint()
	if err != nil {
		return err
	}
	serializedOutPoint := serializeOutPoint(outPoint)
	err = b.Put(serializedOutPoint, cop.Id)
	if err != nil {
		str := fmt.Sprintf("%x:%x", serializedOutPoint, cop.Id)
		return MakeError(ErrWriteDB, str, err)
	}
	return nil
}

func allColorOutPoints(tx walletdb.Tx) ([]*ColorOutPoint, error) {
	currentOutPointId := currentId(tx, outPointIdKey)
	limit := deserializeUint32(currentOutPointId)
	outPoints := make([]*ColorOutPoint, limit-1)
	b := tx.RootBucket().Bucket(colorOutPointBucketName)
	for i := uint32(1); i < limit; i++ {
		key := serializeUint32(i)
		raw := b.Get(key)
		if len(raw) == 0 {
			str := fmt.Sprintf("%d", i)
			return nil, MakeError(ErrColorOutPoint, str, nil)
		}
		var err error
		outPoints[i-1], err = deserializeColorOutPoint(raw)
		if err != nil {
			return nil, err
		}
	}
	return outPoints, nil
}

func lookupScript(tx walletdb.Tx, pkScript []byte) (*uint32, *uint32, error) {
	b := tx.RootBucket().Bucket(scriptToAccountIndexBucketName)
	raw := b.Get(pkScript)
	if len(raw) == 0 {
		str := fmt.Sprintf("%x", pkScript)
		return nil, nil, MakeError(ErrScript, str, nil)
	}
	acct := deserializeUint32(raw[:4])
	index := deserializeUint32(raw[4:])
	return &acct, &index, nil
}
