package chroma_test

import (
	"testing"

	"github.com/monetas/btcnet"
	"github.com/monetas/btcutil"
	"github.com/monetas/btcwallet/chroma"
	"github.com/monetas/gochroma"
)

func TestDeserializeColorOutPointError(t *testing.T) {
	// execute
	_, err := chroma.DeserializeColorOutPoint(nil)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrSerialization)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestSerializeColorOutPointError(t *testing.T) {
	// execute
	_, err := chroma.SerializeColorOutPoint(nil)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrSerialization)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestFetchColorIdErrors(t *testing.T) {
	tests := []struct {
		desc   string
		bucket []byte
	}{
		{
			desc:   "id fetch",
			bucket: chroma.IdBucketName,
		},
		{
			desc:   "cd put",
			bucket: chroma.ColorDefinitionBucketName,
		},
		{
			desc:   "account put",
			bucket: chroma.AccountBucketName,
		},
	}

	for _, test := range tests {
		// setup
		testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
		err := chroma.Initialize(testTx, nil)
		if err != nil {
			t.Fatalf("%v: couldn't initialize tx: %v", test.desc, err)
		}
		cdStr := "EPOBC:00000000000000000000000000000000:0:1"
		cd, err := gochroma.NewColorDefinitionFromStr(cdStr)
		if err != nil {
			t.Fatalf("%v: Definition creation failed: %v", test.desc, err)
		}
		bucket := testTx.Root.Bucket(test.bucket)
		b := bucket.(*chroma.TstBucket)
		b.ErrorAfter = 0

		// execute
		_, err = chroma.FetchColorId(testTx, cd)

		// validate
		if err == nil {
			t.Fatalf("%v: expected error, got nil", test.desc)
		}
		rerr := err.(chroma.ChromaError)
		want := chroma.ErrorCode(chroma.ErrWriteDB)
		if rerr.ErrorCode != want {
			t.Fatalf("%v: different error than expected: want %v got %v", want, err)
		}
	}
}

func TestFetchOutPointId(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}
	outPoint, err := uncoloredCOP.OutPoint()
	if err != nil {
		t.Fatalf("couldn't derive outpoint: %v", err)
	}
	err = chroma.StoreColorOutPoint(testTx, uncoloredCOP)
	if err != nil {
		t.Fatalf("couldn't store color outpoint tx: %v", err)
	}

	// execute
	outPointId := chroma.FetchOutPointId(testTx, outPoint)

	// validate
	if outPointId == nil {
		t.Fatalf("expected a non-nil id")
	}
}

func TestAllColorsError(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}
	b := testTx.RootBucket().Bucket(chroma.ColorDefinitionBucketName)
	err = b.Put([]byte("nonsense"), []byte("blah"))
	if err != nil {
		t.Fatalf("couldn't put data into bucket: %v", err)
	}

	// execute
	_, err = chroma.AllColors(testTx)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrColor)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestFetchKeysError(t *testing.T) {
	tests := []struct {
		desc string
		key  []byte
	}{
		{
			desc: "priv key",
			key:  chroma.PrivKeyName,
		},
		{
			desc: "pub key",
			key:  chroma.PubKeyName,
		},
	}

	for _, test := range tests {
		// setup
		testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
		err := chroma.Initialize(testTx, nil)
		if err != nil {
			t.Fatalf("%v: couldn't initialize tx: %v", test.desc, err)
		}
		b := testTx.RootBucket().Bucket(chroma.KeyBucketName)
		err = b.Put(test.key, []byte("nonsense"))
		if err != nil {
			t.Fatalf("%v: couldn't put into bucket: %v", test.desc, err)
		}

		// execute
		_, _, err = chroma.FetchKeys(testTx)

		// validate
		if err == nil {
			t.Fatalf("%v: expected error, got nil", test.desc)
		}
		rerr := err.(chroma.ChromaError)
		want := chroma.ErrorCode(chroma.ErrHDKey)
		if rerr.ErrorCode != want {
			t.Fatalf("%v: want %v, got %v", test.desc, want, err)
		}
	}
}

func TestInitializeError1(t *testing.T) {
	tests := []struct {
		desc       string
		errorAfter int
	}{
		{
			desc:       "Create Key Bucket",
			errorAfter: 0,
		},
		{
			desc:       "Create ID Bucket",
			errorAfter: 1,
		},
		{
			desc:       "Create Account Bucket",
			errorAfter: 2,
		},
		{
			desc:       "Create Color Definition Bucket",
			errorAfter: 3,
		},
		{
			desc:       "Create Color OutPoint Bucket",
			errorAfter: 4,
		},
		{
			desc:       "Create OutPoint Index Bucket",
			errorAfter: 5,
		},
		{
			desc:       "Create Script To Account Index Bucket",
			errorAfter: 6,
		},
	}

	for _, test := range tests {
		// setup
		testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
		bucket := testTx.RootBucket()
		b := bucket.(*chroma.TstBucket)
		b.ErrorAfter = test.errorAfter

		// execute
		err := chroma.Initialize(testTx, nil)

		// validate
		if err == nil {
			t.Fatalf("%v: expected error, got nil", test.desc)
		}
		rerr := err.(chroma.ChromaError)
		want := chroma.ErrorCode(chroma.ErrCreateBucket)
		if rerr.ErrorCode != want {
			t.Fatalf("%v: unexpected error: want %v, got %v", test.desc, want, err)
		}
	}
}

func TestInitializeError2(t *testing.T) {
	tests := []struct {
		desc       string
		bucket     []byte
		errorAfter int
	}{
		{
			desc:       "Priv Key Put",
			bucket:     chroma.KeyBucketName,
			errorAfter: 0,
		},
		{
			desc:       "Pub Key Put",
			bucket:     chroma.KeyBucketName,
			errorAfter: 1,
		},
		{
			desc:       "Uncolored Acct Put",
			bucket:     chroma.AccountBucketName,
			errorAfter: 0,
		},
		{
			desc:       "Colored Acct Put",
			bucket:     chroma.AccountBucketName,
			errorAfter: 1,
		},
	}

	for _, test := range tests {
		// setup
		testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
		bucket, err := testTx.RootBucket().CreateBucket(test.bucket)
		if err != nil {
			t.Fatalf("%v: can't create bucket %v", test.desc, err)
		}
		b := bucket.(*chroma.TstBucket)
		b.ErrorAfter = test.errorAfter

		// execute
		err = chroma.Initialize(testTx, nil)

		// validate
		if err == nil {
			t.Fatalf("%v: expected error, got nil", test.desc)
		}
		rerr := err.(chroma.ChromaError)
		want := chroma.ErrorCode(chroma.ErrWriteDB)
		if rerr.ErrorCode != want {
			t.Fatalf("%v: unexpected error: want %v, got %v", test.desc, want, err)
		}
	}
}

func TestInitializeError3(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}

	// execute
	err := chroma.Initialize(testTx, []byte("nonsense"))

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrHDKey)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestFetchAcctIndex(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}

	// execute
	_, err = chroma.FetchAcctIndex(testTx, []byte("blah"))

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrAcct)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestStoreScriptIndex1(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}

	// execute
	err = chroma.StoreScriptIndex(testTx, []byte{0}, 0, nil)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrScript)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestStoreScriptIndex2(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}
	bucket, err := testTx.RootBucket().CreateBucket(chroma.ScriptToAccountIndexBucketName)
	if err != nil {
		t.Fatalf("can't create bucket %v", err)
	}
	b := bucket.(*chroma.TstBucket)
	b.ErrorAfter = 0
	addr, err := btcutil.DecodeAddress("1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAMH", &btcnet.MainNetParams)
	if err != nil {
		t.Fatalf("can't make addr %v", err)
	}

	// execute
	err = chroma.StoreScriptIndex(testTx, []byte{0}, 0, addr)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrWriteDB)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestStoreColorOutPointError1(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}

	// execute
	err = chroma.StoreColorOutPoint(testTx, nil)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrSerialization)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestStoreColorOutPointError2(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}
	bucket := testTx.RootBucket().Bucket(chroma.ColorOutPointBucketName)
	b := bucket.(*chroma.TstBucket)
	b.ErrorAfter = 0

	// execute
	err = chroma.StoreColorOutPoint(testTx, uncoloredCOP)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrWriteDB)
	if rerr.ErrorCode != want {
		t.Fatalf("unexpected error: want %v, got %v", want, err)
	}
}

func TestStoreColorOutPointError3(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}

	// execute
	err = chroma.StoreColorOutPoint(testTx, errorCOP)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrShaHash)
	if rerr.ErrorCode != want {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestStoreColorOutPointError4(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}
	bucket := testTx.RootBucket().Bucket(chroma.OutPointIndexBucketName)
	b := bucket.(*chroma.TstBucket)
	b.ErrorAfter = 0

	// execute
	err = chroma.StoreColorOutPoint(testTx, uncoloredCOP)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrWriteDB)
	if rerr.ErrorCode != want {
		t.Fatalf("unexpected error: want %v, got %v", want, err)
	}
}

func TestAllColorOutPointError1(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}
	chroma.NewOutPointId(testTx)

	// execute
	_, err = chroma.AllColorOutPoints(testTx)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrColorOutPoint)
	if rerr.ErrorCode != want {
		t.Fatalf("unexpected error: want %v, got %v", want, err)
	}
}

func TestAllColorOutPointError2(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}
	chroma.NewOutPointId(testTx)
	err = chroma.StoreColorOutPoint(testTx, uncoloredCOP)
	if err != nil {
		t.Fatalf("couldn't store color outpoint: %v", err)
	}
	b := testTx.RootBucket().Bucket(chroma.ColorOutPointBucketName)
	err = b.Put(uncoloredCOP.Id, []byte("blah"))
	if err != nil {
		t.Fatalf("couldn't put data into bucket: %v", err)
	}

	// execute
	_, err = chroma.AllColorOutPoints(testTx)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrSerialization)
	if rerr.ErrorCode != want {
		t.Fatalf("unexpected error: want %v, got %v", want, err)
	}
}

func TestLookupScriptError(t *testing.T) {
	// setup
	testTx := &chroma.TstTx{Root: chroma.NewBucket(-1)}
	err := chroma.Initialize(testTx, nil)
	if err != nil {
		t.Fatalf("couldn't initialize tx: %v", err)
	}

	// execute
	_, _, err = chroma.LookupScript(testTx, []byte("nonsense"))

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	rerr := err.(chroma.ChromaError)
	want := chroma.ErrorCode(chroma.ErrScript)
	if rerr.ErrorCode != want {
		t.Fatalf("unexpected error: want %v, got %v", want, err)
	}
}
