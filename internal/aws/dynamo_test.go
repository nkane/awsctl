package aws

import (
	"context"
	"testing"
)

// TestBatchWriteItemsEmpty verifies the no-op path: with no puts and no
// deletes, BatchWriteItems must not touch the SDK client (here nil) and must
// return nil.
func TestBatchWriteItemsEmpty(t *testing.T) {
	c := &DynamoClient{} // api is nil; the empty path must not dereference it
	if err := c.BatchWriteItems(context.Background(), "tbl", nil, nil); err != nil {
		t.Fatalf("BatchWriteItems(empty): want nil, got %v", err)
	}
}
