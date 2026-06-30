//go:build integration

// LocalStack integration tests for the M6 Dynamo write methods (DeleteItem,
// BatchWriteItems, DeleteTable). Shares helpers (requireLocalStack,
// newTestConfig, uniqueName, containsStr, deleteTable) with integration_test.go.
package aws

import (
	"context"
	"fmt"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// createWriteTestTable creates a pk/sk PAY_PER_REQUEST table and waits for it
// to go ACTIVE, registering cleanup.
func createWriteTestTable(t *testing.T, ctx context.Context, dc *DynamoClient, cfg *Config) string {
	t.Helper()
	table := uniqueName("awsctl-wtbl")
	if err := dc.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   awssdk.String(table),
		BillingMode: ddbtypes.BillingModePayPerRequest,
		AttributeDefinitions: []ddbtypes.AttributeDefinition{
			{AttributeName: awssdk.String("pk"), AttributeType: ddbtypes.ScalarAttributeTypeS},
			{AttributeName: awssdk.String("sk"), AttributeType: ddbtypes.ScalarAttributeTypeS},
		},
		KeySchema: []ddbtypes.KeySchemaElement{
			{AttributeName: awssdk.String("pk"), KeyType: ddbtypes.KeyTypeHash},
			{AttributeName: awssdk.String("sk"), KeyType: ddbtypes.KeyTypeRange},
		},
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if err := dc.WaitTableActive(ctx, table); err != nil {
		t.Fatalf("WaitTableActive: %v", err)
	}
	return table
}

func key(pk, sk string) map[string]ddbtypes.AttributeValue {
	return map[string]ddbtypes.AttributeValue{
		"pk": &ddbtypes.AttributeValueMemberS{Value: pk},
		"sk": &ddbtypes.AttributeValueMemberS{Value: sk},
	}
}

func TestDynamoDeleteItem(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	dc := NewDynamoClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	table := createWriteTestTable(t, ctx, dc, cfg)
	t.Cleanup(func() { deleteTable(t, cfg, table) })

	if err := dc.PutItem(ctx, table, key("user#1", "a")); err != nil {
		t.Fatalf("PutItem: %v", err)
	}

	// Sanity: it exists.
	got, err := dc.GetItem(ctx, table, key("user#1", "a"))
	if err != nil || got == nil {
		t.Fatalf("GetItem before delete: got=%v err=%v", got, err)
	}

	if err := dc.DeleteItem(ctx, table, key("user#1", "a")); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}

	miss, err := dc.GetItem(ctx, table, key("user#1", "a"))
	if err != nil {
		t.Fatalf("GetItem after delete: %v", err)
	}
	if miss != nil {
		t.Fatalf("expected nil after delete, got %+v", miss)
	}

	// DeleteItem of an absent key is a no-op (no error).
	if err := dc.DeleteItem(ctx, table, key("nope", "nope")); err != nil {
		t.Fatalf("DeleteItem(absent): %v", err)
	}
}

func TestDynamoBatchWriteItems(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	dc := NewDynamoClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	table := createWriteTestTable(t, ctx, dc, cfg)
	t.Cleanup(func() { deleteTable(t, cfg, table) })

	// Batch-put 30 items (forces >1 chunk past the 25-item limit).
	puts := make([]map[string]ddbtypes.AttributeValue, 0, 30)
	for i := 0; i < 30; i++ {
		puts = append(puts, key("user#1", fmt.Sprintf("s%02d", i)))
	}
	if err := dc.BatchWriteItems(ctx, table, puts, nil); err != nil {
		t.Fatalf("BatchWriteItems(puts): %v", err)
	}

	scan, err := dc.Scan(ctx, ScanInput{Table: table})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(scan.Items) != 30 {
		t.Fatalf("after batch put: want 30 items, got %d", len(scan.Items))
	}

	// Batch-delete the first 10.
	dels := make([]map[string]ddbtypes.AttributeValue, 0, 10)
	for i := 0; i < 10; i++ {
		dels = append(dels, key("user#1", fmt.Sprintf("s%02d", i)))
	}
	if err := dc.BatchWriteItems(ctx, table, nil, dels); err != nil {
		t.Fatalf("BatchWriteItems(deletes): %v", err)
	}

	scan2, err := dc.Scan(ctx, ScanInput{Table: table})
	if err != nil {
		t.Fatalf("Scan after delete: %v", err)
	}
	if len(scan2.Items) != 20 {
		t.Fatalf("after batch delete: want 20 items, got %d", len(scan2.Items))
	}
}

func TestDynamoDeleteTable(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	dc := NewDynamoClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	table := createWriteTestTable(t, ctx, dc, cfg)

	if err := dc.DeleteTable(ctx, table); err != nil {
		t.Fatalf("DeleteTable: %v", err)
	}

	// The table leaves the listing once DELETING completes.
	deadline := time.Now().Add(30 * time.Second)
	for {
		tables, err := dc.ListTables(ctx)
		if err != nil {
			t.Fatalf("ListTables: %v", err)
		}
		if !containsStr(tables, table) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("table %q still present 30s after DeleteTable", table)
		}
		time.Sleep(1 * time.Second)
	}
}
