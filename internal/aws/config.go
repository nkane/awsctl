// Package aws wraps the AWS SDK v2 with thin, TUI-friendly helpers.
package aws

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"gopkg.in/ini.v1"
)

// Config holds the resolved AWS SDK config plus the profile/region
// that produced it. The TUI displays these in the status bar.
type Config struct {
	AWS     awssdk.Config
	Profile string
	Region  string
}

// Load resolves an aws.Config using the default credential chain,
// optionally overriding profile and region. If the AWSCTL_ENDPOINT_URL env
// var is set (e.g. http://localhost:4566 for LocalStack), all SDK clients
// constructed from this config will route to that endpoint.
func Load(ctx context.Context, profile, region string) (*Config, error) {
	opts := []func(*config.LoadOptions) error{}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	if endpoint := os.Getenv("AWSCTL_ENDPOINT_URL"); endpoint != "" {
		opts = append(opts, config.WithBaseEndpoint(endpoint))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws: load config: %w", err)
	}
	resolvedProfile := profile
	if resolvedProfile == "" {
		if v := os.Getenv("AWS_PROFILE"); v != "" {
			resolvedProfile = v
		} else {
			resolvedProfile = "default"
		}
	}
	return &Config{AWS: cfg, Profile: resolvedProfile, Region: cfg.Region}, nil
}

// ListProfiles parses ~/.aws/config and ~/.aws/credentials and returns
// the union of profile names found in either file.
func ListProfiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, path := range []string{
		filepath.Join(home, ".aws", "config"),
		filepath.Join(home, ".aws", "credentials"),
	} {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		f, err := ini.Load(path)
		if err != nil {
			continue
		}
		for _, s := range f.Sections() {
			name := s.Name()
			if name == ini.DefaultSection {
				continue
			}
			// In ~/.aws/config sections look like "profile foo"; "default" is bare.
			name = strings.TrimPrefix(name, "profile ")
			if name == "" {
				continue
			}
			seen[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	if len(out) == 0 {
		out = []string{"default"}
	}
	return out, nil
}

// CommonRegions is a starter list shown in the region picker.
// Users may type a custom region too.
var CommonRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"ca-central-1",
	"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1",
	"ap-south-1", "ap-southeast-1", "ap-southeast-2", "ap-northeast-1", "ap-northeast-2",
	"sa-east-1",
}
