package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoClient wraps the SDK DynamoDB client.
type DynamoClient struct {
	api *dynamodb.Client
}

// NewDynamoClient constructs a DynamoClient.
func NewDynamoClient(cfg *Config) *DynamoClient {
	return &DynamoClient{api: dynamodb.NewFromConfig(cfg.AWS)}
}

// ListTables returns all table names in the configured region.
func (c *DynamoClient) ListTables(ctx context.Context) ([]string, error) {
	out := []string{}
	p := dynamodb.NewListTablesPaginator(c.api, &dynamodb.ListTablesInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("dynamodb: list tables: %w", err)
		}
		out = append(out, page.TableNames...)
	}
	return out, nil
}

// DescribeTable returns the full table description.
func (c *DynamoClient) DescribeTable(ctx context.Context, name string) (*ddbtypes.TableDescription, error) {
	resp, err := c.api.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &name})
	if err != nil {
		return nil, fmt.Errorf("dynamodb: describe %q: %w", name, err)
	}
	return resp.Table, nil
}

// ScanInput is a thin shim over the SDK input that the UI can populate
// without importing the SDK package directly.
type ScanInput struct {
	Table             string
	Limit             int32
	FilterExpression  string
	ExpressionValues  map[string]ddbtypes.AttributeValue
	ExclusiveStartKey map[string]ddbtypes.AttributeValue
}

// ScanResult holds one page of scan results.
type ScanResult struct {
	Items            []map[string]ddbtypes.AttributeValue
	LastEvaluatedKey map[string]ddbtypes.AttributeValue
	ScannedCount     int32
}

// Scan runs a single page Scan; caller continues paging by passing
// LastEvaluatedKey back as ExclusiveStartKey.
func (c *DynamoClient) Scan(ctx context.Context, in ScanInput) (*ScanResult, error) {
	req := &dynamodb.ScanInput{TableName: &in.Table}
	if in.Limit > 0 {
		req.Limit = &in.Limit
	}
	if in.FilterExpression != "" {
		req.FilterExpression = &in.FilterExpression
	}
	if len(in.ExpressionValues) > 0 {
		req.ExpressionAttributeValues = in.ExpressionValues
	}
	if len(in.ExclusiveStartKey) > 0 {
		req.ExclusiveStartKey = in.ExclusiveStartKey
	}
	resp, err := c.api.Scan(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("dynamodb: scan %q: %w", in.Table, err)
	}
	return &ScanResult{
		Items:            resp.Items,
		LastEvaluatedKey: resp.LastEvaluatedKey,
		ScannedCount:     resp.ScannedCount,
	}, nil
}

// QueryInput drives a single-page Query.
type QueryInput struct {
	Table                  string
	IndexName              string
	KeyConditionExpression string
	FilterExpression       string
	ExpressionValues       map[string]ddbtypes.AttributeValue
	Limit                  int32
	ExclusiveStartKey      map[string]ddbtypes.AttributeValue
}

// QueryResult is the per-page query output.
type QueryResult struct {
	Items            []map[string]ddbtypes.AttributeValue
	LastEvaluatedKey map[string]ddbtypes.AttributeValue
}

// Query executes a single-page DynamoDB Query.
func (c *DynamoClient) Query(ctx context.Context, in QueryInput) (*QueryResult, error) {
	req := &dynamodb.QueryInput{
		TableName:                 &in.Table,
		KeyConditionExpression:    &in.KeyConditionExpression,
		ExpressionAttributeValues: in.ExpressionValues,
	}
	if in.IndexName != "" {
		req.IndexName = &in.IndexName
	}
	if in.FilterExpression != "" {
		req.FilterExpression = &in.FilterExpression
	}
	if in.Limit > 0 {
		req.Limit = &in.Limit
	}
	if len(in.ExclusiveStartKey) > 0 {
		req.ExclusiveStartKey = in.ExclusiveStartKey
	}
	resp, err := c.api.Query(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("dynamodb: query %q: %w", in.Table, err)
	}
	return &QueryResult{Items: resp.Items, LastEvaluatedKey: resp.LastEvaluatedKey}, nil
}

// PartiQL executes a single ExecuteStatement call. UI handles paging by
// passing NextToken back through subsequent calls if needed (omitted here
// for v1 simplicity).
func (c *DynamoClient) PartiQL(ctx context.Context, statement string) ([]map[string]ddbtypes.AttributeValue, error) {
	resp, err := c.api.ExecuteStatement(ctx, &dynamodb.ExecuteStatementInput{Statement: &statement})
	if err != nil {
		return nil, fmt.Errorf("dynamodb: partiql: %w", err)
	}
	return resp.Items, nil
}

// PutItem is exposed only for tests/seeding; the UI never calls it in v1
// (read-only). v2 will gate any mutating wrappers behind --unsafe.
func (c *DynamoClient) PutItem(ctx context.Context, table string, item map[string]ddbtypes.AttributeValue) error {
	_, err := c.api.PutItem(ctx, &dynamodb.PutItemInput{TableName: &table, Item: item})
	if err != nil {
		return fmt.Errorf("dynamodb: put item %q: %w", table, err)
	}
	return nil
}

// CreateTable is exposed only for tests/seeding (see PutItem note).
func (c *DynamoClient) CreateTable(ctx context.Context, in *dynamodb.CreateTableInput) error {
	_, err := c.api.CreateTable(ctx, in)
	if err != nil {
		return fmt.Errorf("dynamodb: create table: %w", err)
	}
	return nil
}

// WaitTableActive blocks until the named table reaches ACTIVE status or ctx
// is cancelled.
func (c *DynamoClient) WaitTableActive(ctx context.Context, name string) error {
	w := dynamodb.NewTableExistsWaiter(c.api)
	return w.Wait(ctx, &dynamodb.DescribeTableInput{TableName: &name}, 60_000_000_000) // 60s
}
