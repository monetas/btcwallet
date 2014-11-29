package chroma

import (
	"github.com/monetas/btcutil"
	"github.com/monetas/btcutil/hdkeychain"
	"github.com/monetas/btcwallet/walletdb"
	"github.com/monetas/btcwire"
	"github.com/monetas/gochroma"
)

var (
	KeyBucketName                  = keyBucketName
	IdBucketName                   = idBucketName
	AccountBucketName              = accountBucketName
	ColorDefinitionBucketName      = colorDefinitionBucketName
	ColorOutPointBucketName        = colorOutPointBucketName
	OutPointIndexBucketName        = outPointIndexBucketName
	ScriptToAccountIndexBucketName = scriptToAccountIndexBucketName
	PubKeyName                     = pubKeyName
	PrivKeyName                    = privKeyName
)

func SerializeUint32(i uint32) []byte {
	return serializeUint32(i)
}

func DeserializeColorOutPoint(b []byte) (*ColorOutPoint, error) {
	return deserializeColorOutPoint(b)
}

func SerializeColorOutPoint(cop *ColorOutPoint) ([]byte, error) {
	return serializeColorOutPoint(cop)
}

func FetchColorId(tx walletdb.Tx, cd *gochroma.ColorDefinition) (ColorId, error) {
	return fetchColorId(tx, cd)
}

func Initialize(tx walletdb.Tx, seed []byte) error {
	return initialize(tx, seed)
}

func FetchOutPointId(tx walletdb.Tx, outPoint *btcwire.OutPoint) OutPointId {
	return fetchOutPointId(tx, outPoint)
}

func StoreColorOutPoint(tx walletdb.Tx, cop *ColorOutPoint) error {
	return storeColorOutPoint(tx, cop)
}

func AllColors(tx walletdb.Tx) (map[*gochroma.ColorDefinition]ColorId, error) {
	return allColors(tx)
}

func FetchKeys(tx walletdb.Tx) (*hdkeychain.ExtendedKey, *hdkeychain.ExtendedKey, error) {
	return fetchKeys(tx)
}

func FetchAcctIndex(tx walletdb.Tx, acct uint32) (*uint32, error) {
	return fetchAcctIndex(tx, acct)
}

func StoreScriptIndex(tx walletdb.Tx, acct, index uint32, addr btcutil.Address) error {
	return storeScriptIndex(tx, acct, index, addr)
}

func NewOutPointId(tx walletdb.Tx) (OutPointId, error) {
	return newOutPointId(tx)
}

func AllColorOutPoints(tx walletdb.Tx) ([]*ColorOutPoint, error) {
	return allColorOutPoints(tx)
}

func LookupScript(tx walletdb.Tx, pkScript []byte) (*uint32, *uint32, error) {
	return lookupScript(tx, pkScript)
}
