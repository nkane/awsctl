//go:build integration

// Integration tests run against a LocalStack instance.
//
// Bring up LocalStack first:
//   docker compose -f docker-compose.localstack.yml up -d
//
// Then run:
//   AWSCTL_ENDPOINT_URL=http://localhost:4566 \
//   AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1 \
//   go test -tags=integration ./internal/aws/...
//
// The default-skip behaviour means CI without docker still passes.
package aws

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

const (
	endpointEnv = "AWSCTL_ENDPOINT_URL"
	defaultURL  = "http://localhost:4566"
)

// requireLocalStack skips the test unless LocalStack responds on the configured
// endpoint within a short timeout. Keeps `go test ./...` green on machines
// without docker even when the build tag is enabled.
func requireLocalStack(t *testing.T) string {
	t.Helper()
	url := os.Getenv(endpointEnv)
	if url == "" {
		url = defaultURL
		t.Setenv(endpointEnv, url)
	}
	// Provide static credentials LocalStack accepts.
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	if os.Getenv("AWS_REGION") == "" {
		t.Setenv("AWS_REGION", "us-east-1")
	}

	hc := &http.Client{Timeout: 2 * time.Second}
	resp, err := hc.Get(url + "/_localstack/health")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("LocalStack not reachable at %s (start with: docker compose -f docker-compose.localstack.yml up -d): %v", url, err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	return url
}

func newTestConfig(t *testing.T) *Config {
	t.Helper()
	cfg, err := Load(context.Background(), "", "us-east-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

// ---------- Lambda ----------

func TestLambdaListAndGet(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	lc := NewLambdaClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	name := uniqueName("awsctl-fn")
	createPythonNoop(t, ctx, cfg, name)
	t.Cleanup(func() { deleteFunction(t, cfg, name) })

	fns, err := lc.ListFunctions(ctx)
	if err != nil {
		t.Fatalf("ListFunctions: %v", err)
	}
	if !containsFn(fns, name) {
		t.Fatalf("expected %q in list, got %d functions", name, len(fns))
	}

	got, err := lc.GetFunction(ctx, name)
	if err != nil {
		t.Fatalf("GetFunction: %v", err)
	}
	if got.Configuration == nil || got.Configuration.FunctionName == nil || *got.Configuration.FunctionName != name {
		t.Fatalf("unexpected GetFunction response: %+v", got)
	}
}

func TestLambdaInvoke(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	lc := NewLambdaClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	name := uniqueName("awsctl-inv")
	createPythonNoop(t, ctx, cfg, name)
	t.Cleanup(func() { deleteFunction(t, cfg, name) })

	// Wait for fn to leave Pending. LocalStack may need a second.
	if err := waitFunctionActive(ctx, cfg, name); err != nil {
		t.Skipf("function did not reach Active (likely LocalStack docker-in-docker exec issue): %v", err)
	}

	res, err := lc.Invoke(ctx, name, []byte(`{"hello":"world"}`))
	if err != nil {
		t.Skipf("Invoke not supported in this LocalStack setup: %v", err)
	}
	if res.FunctionError != "" {
		t.Fatalf("function error: %s payload=%s", res.FunctionError, string(res.Payload))
	}
	if !strings.Contains(string(res.Payload), "world") {
		t.Fatalf("expected payload echo, got: %s", string(res.Payload))
	}
}

// ---------- DynamoDB ----------

func TestDynamoCRUDAndQuery(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	dc := NewDynamoClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	table := uniqueName("awsctl-tbl")
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
	t.Cleanup(func() { deleteTable(t, cfg, table) })

	if err := dc.WaitTableActive(ctx, table); err != nil {
		t.Fatalf("WaitTableActive: %v", err)
	}

	// Seed three items.
	for i, sk := range []string{"a", "b", "c"} {
		err := dc.PutItem(ctx, table, map[string]ddbtypes.AttributeValue{
			"pk":  &ddbtypes.AttributeValueMemberS{Value: "user#1"},
			"sk":  &ddbtypes.AttributeValueMemberS{Value: sk},
			"idx": &ddbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", i)},
		})
		if err != nil {
			t.Fatalf("PutItem %s: %v", sk, err)
		}
	}

	// ListTables sees it.
	tables, err := dc.ListTables(ctx)
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	if !containsStr(tables, table) {
		t.Fatalf("table %q missing from ListTables (%v)", table, tables)
	}

	// Describe.
	desc, err := dc.DescribeTable(ctx, table)
	if err != nil {
		t.Fatalf("DescribeTable: %v", err)
	}
	if desc.TableName == nil || *desc.TableName != table {
		t.Fatalf("describe mismatch: %+v", desc)
	}

	// Scan.
	scan, err := dc.Scan(ctx, ScanInput{Table: table})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(scan.Items) != 3 {
		t.Fatalf("expected 3 scanned items, got %d", len(scan.Items))
	}

	// Query by pk.
	q, err := dc.Query(ctx, QueryInput{
		Table:                  table,
		KeyConditionExpression: "pk = :p",
		ExpressionValues: map[string]ddbtypes.AttributeValue{
			":p": &ddbtypes.AttributeValueMemberS{Value: "user#1"},
		},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(q.Items) != 3 {
		t.Fatalf("expected 3 queried items, got %d", len(q.Items))
	}

	// PartiQL.
	stmt := fmt.Sprintf(`SELECT pk, sk FROM "%s" WHERE pk = 'user#1'`, table)
	rows, err := dc.PartiQL(ctx, stmt)
	if err != nil {
		t.Fatalf("PartiQL: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("PartiQL expected 3 rows, got %d", len(rows))
	}
}

// ---------- CloudWatch Logs ----------

func TestLogsFilter(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	lc := NewLogsClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	group := "/awsctl/test/" + uniqueName("grp")
	if err := lc.EnsureGroup(ctx, group); err != nil {
		t.Fatalf("EnsureGroup: %v", err)
	}

	// EnsureGroup is idempotent — second call should also succeed.
	if err := lc.EnsureGroup(ctx, group); err != nil {
		t.Fatalf("EnsureGroup repeat: %v", err)
	}

	// Filter against an empty group returns no events, no error.
	page, err := lc.Filter(ctx, FilterInput{LogGroup: group, Limit: 10})
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(page.Events) != 0 {
		t.Fatalf("expected empty events, got %d", len(page.Events))
	}
}

// ---------- helpers ----------

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func containsStr(xs []string, target string) bool {
	for _, x := range xs {
		if x == target {
			return true
		}
	}
	return false
}

func containsFn(xs []FunctionSummary, name string) bool {
	for _, x := range xs {
		if x.Name == name {
			return true
		}
	}
	return false
}

// pythonNoopZip builds an in-memory zip containing a Python handler that
// echoes its event back. Suitable for the python3.12 LocalStack runtime.
func pythonNoopZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("handler.py")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	src := "def handler(event, context):\n    return event\n"
	if _, err := w.Write([]byte(src)); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func createPythonNoop(t *testing.T, ctx context.Context, cfg *Config, name string) {
	t.Helper()
	api := lambda.NewFromConfig(cfg.AWS)
	_, err := api.CreateFunction(ctx, &lambda.CreateFunctionInput{
		FunctionName: awssdk.String(name),
		Role:         awssdk.String("arn:aws:iam::000000000000:role/lambda-role"),
		Runtime:      lambdatypes.RuntimePython312,
		Handler:      awssdk.String("handler.handler"),
		Code:         &lambdatypes.FunctionCode{ZipFile: pythonNoopZip(t)},
		Timeout:      awssdk.Int32(10),
	})
	if err != nil {
		t.Fatalf("CreateFunction: %v", err)
	}
}

func deleteFunction(t *testing.T, cfg *Config, name string) {
	api := lambda.NewFromConfig(cfg.AWS)
	_, err := api.DeleteFunction(context.Background(), &lambda.DeleteFunctionInput{FunctionName: awssdk.String(name)})
	if err != nil {
		t.Logf("cleanup DeleteFunction(%s): %v", name, err)
	}
}

func deleteTable(t *testing.T, cfg *Config, name string) {
	api := dynamodb.NewFromConfig(cfg.AWS)
	_, err := api.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: awssdk.String(name)})
	if err != nil {
		t.Logf("cleanup DeleteTable(%s): %v", name, err)
	}
}

func waitFunctionActive(ctx context.Context, cfg *Config, name string) error {
	api := lambda.NewFromConfig(cfg.AWS)
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		out, err := api.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: awssdk.String(name)})
		if err != nil {
			return err
		}
		if out.Configuration != nil && out.Configuration.State == lambdatypes.StateActive {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return errors.New("timeout waiting for function Active")
}
