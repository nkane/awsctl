package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// errorsAs is a thin alias for errors.As so test files in this package
// can satisfy interfaces without re-importing.
func errorsAs(err error, target any) bool { return errors.As(err, target) }

// LambdaClient wraps the SDK Lambda client with TUI-friendly helpers.
type LambdaClient struct {
	api *lambda.Client
}

// NewLambdaClient constructs a LambdaClient from a resolved Config.
func NewLambdaClient(cfg *Config) *LambdaClient {
	return &LambdaClient{api: lambda.NewFromConfig(cfg.AWS)}
}

// FunctionSummary is a thin projection of types.FunctionConfiguration
// holding only the fields the list view needs.
type FunctionSummary struct {
	Name        string
	ARN         string
	Runtime     string
	Memory      int32
	Timeout     int32
	Handler     string
	Description string
	LastUpdated string
}

// ListFunctions returns all Lambda functions in the configured region.
// Pagination is fully drained; callers may add their own UI-level paging.
func (c *LambdaClient) ListFunctions(ctx context.Context) ([]FunctionSummary, error) {
	out := []FunctionSummary{}
	p := lambda.NewListFunctionsPaginator(c.api, &lambda.ListFunctionsInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("lambda: list functions: %w", err)
		}
		for _, fn := range page.Functions {
			out = append(out, summarize(fn))
		}
	}
	return out, nil
}

// GetFunction returns the full configuration plus environment for one fn.
func (c *LambdaClient) GetFunction(ctx context.Context, name string) (*lambda.GetFunctionOutput, error) {
	resp, err := c.api.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: &name})
	if err != nil {
		return nil, fmt.Errorf("lambda: get function %q: %w", name, err)
	}
	return resp, nil
}

// FunctionDetail aggregates everything the detail screen displays for one
// function. Each subfield may be nil/empty if the corresponding API call
// failed or returned nothing — the UI renders "(none)" or skips the tab.
type FunctionDetail struct {
	Function     *lambda.GetFunctionOutput
	Concurrency  *lambda.GetFunctionConcurrencyOutput
	Tags         map[string]string
	Aliases      []types.AliasConfiguration
	Versions     []types.FunctionConfiguration
	EventSources []types.EventSourceMappingConfiguration
	URLConfigs   []types.FunctionUrlConfig
	Policy       string // raw resource policy JSON, "" if none
	CodeSigning  *lambda.GetFunctionCodeSigningConfigOutput
	// Errors collects per-call failures so the UI can show partial data
	// instead of failing the whole detail load.
	Errors map[string]string
}

// GetFunctionDetail fetches everything the detail screen needs in parallel.
// Per-call failures are captured in Detail.Errors instead of aborting.
func (c *LambdaClient) GetFunctionDetail(ctx context.Context, name string) (*FunctionDetail, error) {
	d := &FunctionDetail{Errors: map[string]string{}}

	// Function config + code location is the only call we treat as fatal.
	fn, err := c.api.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: &name})
	if err != nil {
		return nil, fmt.Errorf("lambda: get function %q: %w", name, err)
	}
	d.Function = fn

	if conc, err := c.api.GetFunctionConcurrency(ctx, &lambda.GetFunctionConcurrencyInput{FunctionName: &name}); err != nil {
		d.Errors["concurrency"] = err.Error()
	} else {
		d.Concurrency = conc
	}

	// Tags live on the function ARN.
	if fn.Configuration != nil && fn.Configuration.FunctionArn != nil {
		if tagsOut, err := c.api.ListTags(ctx, &lambda.ListTagsInput{Resource: fn.Configuration.FunctionArn}); err != nil {
			d.Errors["tags"] = err.Error()
		} else {
			d.Tags = tagsOut.Tags
		}
	}

	if al, err := c.api.ListAliases(ctx, &lambda.ListAliasesInput{FunctionName: &name}); err != nil {
		d.Errors["aliases"] = err.Error()
	} else {
		d.Aliases = al.Aliases
	}

	if v, err := c.api.ListVersionsByFunction(ctx, &lambda.ListVersionsByFunctionInput{FunctionName: &name}); err != nil {
		d.Errors["versions"] = err.Error()
	} else {
		d.Versions = v.Versions
	}

	if es, err := c.api.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{FunctionName: &name}); err != nil {
		d.Errors["event_sources"] = err.Error()
	} else {
		d.EventSources = es.EventSourceMappings
	}

	if u, err := c.api.ListFunctionUrlConfigs(ctx, &lambda.ListFunctionUrlConfigsInput{FunctionName: &name}); err != nil {
		d.Errors["url_configs"] = err.Error()
	} else {
		d.URLConfigs = u.FunctionUrlConfigs
	}

	if pol, err := c.api.GetPolicy(ctx, &lambda.GetPolicyInput{FunctionName: &name}); err != nil {
		// ResourceNotFoundException = no policy, not an error.
		if !isNotFound(err) {
			d.Errors["policy"] = err.Error()
		}
	} else if pol.Policy != nil {
		d.Policy = *pol.Policy
	}

	if cs, err := c.api.GetFunctionCodeSigningConfig(ctx, &lambda.GetFunctionCodeSigningConfigInput{FunctionName: &name}); err != nil {
		if !isNotFound(err) {
			d.Errors["code_signing"] = err.Error()
		}
	} else {
		d.CodeSigning = cs
	}

	return d, nil
}

// isNotFound reports whether an SDK error wraps a ResourceNotFoundException
// from any of the AWS services we call. Saves a typed-error import in callers.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var nf *types.ResourceNotFoundException
	if errorsAs(err, &nf) {
		return true
	}
	return false
}

// InvokeResult is the decoded outcome of a synchronous invocation.
type InvokeResult struct {
	StatusCode      int32
	FunctionError   string
	LogResult       string // base64-encoded; UI decodes when displaying
	Payload         []byte
	ExecutedVersion string
}

// Invoke runs a synchronous RequestResponse invocation with the provided
// JSON payload (may be nil for empty event).
func (c *LambdaClient) Invoke(ctx context.Context, name string, payload []byte) (*InvokeResult, error) {
	in := &lambda.InvokeInput{
		FunctionName:   &name,
		InvocationType: types.InvocationTypeRequestResponse,
		LogType:        types.LogTypeTail,
		Payload:        payload,
	}
	resp, err := c.api.Invoke(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("lambda: invoke %q: %w", name, err)
	}
	r := &InvokeResult{
		StatusCode: resp.StatusCode,
		Payload:    resp.Payload,
	}
	if resp.FunctionError != nil {
		r.FunctionError = *resp.FunctionError
	}
	if resp.LogResult != nil {
		r.LogResult = *resp.LogResult
	}
	if resp.ExecutedVersion != nil {
		r.ExecutedVersion = *resp.ExecutedVersion
	}
	return r, nil
}

func summarize(fn types.FunctionConfiguration) FunctionSummary {
	s := FunctionSummary{}
	if fn.FunctionName != nil {
		s.Name = *fn.FunctionName
	}
	if fn.FunctionArn != nil {
		s.ARN = *fn.FunctionArn
	}
	s.Runtime = string(fn.Runtime)
	if fn.MemorySize != nil {
		s.Memory = *fn.MemorySize
	}
	if fn.Timeout != nil {
		s.Timeout = *fn.Timeout
	}
	if fn.Handler != nil {
		s.Handler = *fn.Handler
	}
	if fn.Description != nil {
		s.Description = *fn.Description
	}
	if fn.LastModified != nil {
		s.LastUpdated = *fn.LastModified
	}
	return s
}
