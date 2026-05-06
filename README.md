# awsctl

TUI for managing AWS Lambda functions and querying DynamoDB tables. Read-only by default; mutations gated behind `--unsafe`.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) + AWS SDK for Go v2.

## Install

```sh
go install github.com/nkane/awsctl/cmd/awsctl@latest
```

A short alias `ac` is installed alongside `awsctl` via goreleaser releases.

## Usage

```sh
awsctl              # read-only mode
awsctl --unsafe     # enables write/destructive operations (with confirm modals)
awsctl --profile dev --region us-east-1
```

Credentials use the default AWS SDK chain (`~/.aws/config`, env vars, IAM role). Switch profile/region inside the TUI with `p`.

### Keys

| Key | Action |
|-----|--------|
| `1` | Lambda mode |
| `2` | DynamoDB mode |
| `p` | Profile/region picker |
| `/` | Filter |
| `j` `k` `g` `G` | Navigate |
| `enter` | Open |
| `esc` | Back |
| `tab` | Next tab |
| `m` | Load more (paginate) |
| `?` | Help |
| `q` | Quit |

## Status

- M0 skeleton: in progress
- M1 Lambda read: pending
- M2 Lambda metrics: pending
- M3 DynamoDB read: pending
- M4 PartiQL + export: pending
- M5 v1 release: pending
- M6 v2 writes (`--unsafe`): pending

## License

MIT — see [LICENSE](LICENSE).

## Development

### Tests

Unit tests (no docker required):

```sh
go test ./...
```

Integration tests run against LocalStack. Bring it up first:

```sh
docker compose -f docker-compose.localstack.yml up -d
AWSCTL_ENDPOINT_URL=http://localhost:4566 \
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1 \
  go test -tags=integration ./...
docker compose -f docker-compose.localstack.yml down -v
```

The `AWSCTL_ENDPOINT_URL` env var also works at runtime — point `awsctl` at LocalStack to develop offline:

```sh
AWSCTL_ENDPOINT_URL=http://localhost:4566 \
AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
  ./awsctl --region us-east-1
```
