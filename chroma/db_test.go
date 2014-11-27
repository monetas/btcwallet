package chroma_test

import (
	"testing"

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
}

func TestSerializeColorOutPointError(t *testing.T) {
	// execute
	_, err := chroma.SerializeColorOutPoint(nil)

	// validate
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestFetchColorIdErrors(t *testing.T) {
	tests := []struct {
		desc       string
		bucket     []byte
		errorAfter int
	}{
		{
			desc:       "id fetch",
			bucket:     chroma.IdBucketName,
			errorAfter: 0,
		},
		{
			desc:       "cd put",
			bucket:     chroma.ColorDefinitionBucketName,
			errorAfter: 0,
		},
		{
			desc:       "account put",
			bucket:     chroma.AccountBucketName,
			errorAfter: 0,
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
		b.ErrorAfter = test.errorAfter

		// execute
		_, err = chroma.FetchColorId(testTx, cd)

		// validate
		if err == nil {
			t.Fatalf("%v: expected error, got nil", test.desc)
		}
		if err != chroma.TestError {
			t.Fatalf("%v: different error than expected: want %v got %v", chroma.TestError, err)
		}
	}
}
