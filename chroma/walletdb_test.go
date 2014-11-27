// Make a very simple in-memory walletdb
// TODO: Add hooks for intentional errors

package chroma

import (
	"errors"
	"fmt"
	"io"

	"github.com/monetas/btcwallet/walletdb"
)

var TestError = errors.New("Test Error")

type TstBucket struct {
	lookup     map[string]string
	buckets    map[string]*TstBucket
	count      int
	ErrorAfter int
}

var _ walletdb.Bucket = (*TstBucket)(nil)

func NewBucket(errorAfter int) *TstBucket {
	return &TstBucket{
		lookup:     make(map[string]string, 10),
		buckets:    make(map[string]*TstBucket, 10),
		ErrorAfter: errorAfter,
	}
}

func (b *TstBucket) Bucket(key []byte) walletdb.Bucket {
	bucket, ok := b.buckets[string(key)]
	if !ok {
		return nil
	}
	return bucket
}

func (b *TstBucket) CreateBucket(key []byte) (walletdb.Bucket, error) {
	if b.ErrorAfter != -1 && b.ErrorAfter <= b.count {
		return nil, TestError
	}
	b.count++
	newBucket := TstBucket{
		lookup:     make(map[string]string, 10),
		buckets:    make(map[string]*TstBucket, 10),
		ErrorAfter: -1,
	}
	b.buckets[string(key)] = &newBucket
	return &newBucket, nil
}

func (b *TstBucket) CreateBucketIfNotExists(key []byte) (walletdb.Bucket, error) {
	if b.ErrorAfter != -1 && b.ErrorAfter <= b.count {
		return nil, TestError
	}
	b.count++
	bucket := b.Bucket(key)
	if bucket != nil {
		return bucket, nil
	}
	return b.CreateBucket(key)
}

func (b *TstBucket) DeleteBucket(key []byte) error {
	if b.ErrorAfter != -1 && b.ErrorAfter <= b.count {
		return TestError
	}
	b.count++
	delete(b.buckets, string(key))
	return nil
}

func (b *TstBucket) ForEach(fn func(k, v []byte) error) error {
	if b.ErrorAfter != -1 && b.ErrorAfter <= b.count {
		return TestError
	}
	b.count++
	for k, v := range b.lookup {
		err := fn([]byte(k), []byte(v))
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *TstBucket) Writable() bool {
	return true
}

func (b *TstBucket) Put(key, value []byte) error {
	if b.ErrorAfter != -1 && b.ErrorAfter <= b.count {
		return TestError
	}
	b.count++
	b.lookup[string(key)] = string(value)
	return nil
}

func (b *TstBucket) Get(key []byte) []byte {
	val, ok := b.lookup[string(key)]
	if !ok {
		return nil
	}
	return []byte(val)
}

func (b *TstBucket) Delete(key []byte) error {
	if b.ErrorAfter != -1 && b.ErrorAfter <= b.count {
		return TestError
	}
	b.count++
	delete(b.lookup, string(key))
	return nil
}

type TstTx struct {
	Root *TstBucket
}

var _ walletdb.Tx = (*TstTx)(nil)

func (tx *TstTx) RootBucket() walletdb.Bucket {
	return tx.Root
}
func (tx *TstTx) Commit() error {
	return nil
}
func (tx *TstTx) Rollback() error {
	return nil
}

type TstNamespace struct {
	tx  *TstTx
	key []byte
}

var _ walletdb.Namespace = (*TstNamespace)(nil)

func (ns *TstNamespace) Begin(writable bool) (walletdb.Tx, error) {
	return ns.tx, nil
}
func (ns *TstNamespace) View(fn func(walletdb.Tx) error) error {
	return fn(ns.tx)
}
func (ns *TstNamespace) Update(fn func(walletdb.Tx) error) error {
	return fn(ns.tx)
}

type TstDb struct {
	namespaces map[string]*TstNamespace
}

// Enforce db implements the walletdb.Db interface.
var _ walletdb.DB = (*TstDb)(nil)

func (db *TstDb) Namespace(key []byte) (walletdb.Namespace, error) {
	namespace, ok := db.namespaces[string(key)]
	if !ok {
		tx := &TstTx{Root: NewBucket(-1)}
		db.namespaces[string(key)] = &TstNamespace{tx: tx, key: key}
		return db.namespaces[string(key)], nil
	}
	return namespace, nil
}
func (db *TstDb) DeleteNamespace(key []byte) error {
	delete(db.namespaces, string(key))
	return nil
}
func (db *TstDb) Copy(w io.Writer) error {
	return nil
}
func (db *TstDb) Close() error {
	return nil
}

func init() {
	// Register the driver.
	dbType := "test"
	driver := walletdb.Driver{
		DbType: dbType,
		Create: dBDriver,
		Open:   dBDriver,
	}
	if err := walletdb.RegisterDriver(driver); err != nil {
		panic(fmt.Sprintf("Failed to regiser database driver '%s': %v",
			dbType, err))
	}
}

func dBDriver(args ...interface{}) (walletdb.DB, error) {
	newDb := &TstDb{namespaces: make(map[string]*TstNamespace, 10)}
	return newDb, nil
}
