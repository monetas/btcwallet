package chroma

import (
	"github.com/monetas/btcwallet/walletdb"
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
)

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
