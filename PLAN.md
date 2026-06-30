# awsctl — Build Plan

> Living plan. Kept in sync with the work as tickets land. Source of truth for
> scope, milestone status, and what's next. Doubles as the input plan for the
> `build-with-agent-team` skill (see the Validation section).
>
> Last synced: 2026-06-30 — M6 complete: writes #32–#39/#57–#60 (PR #83) merged; M2 + #40 already in.

## Vision

A k9s-style terminal UI for inspecting and (opt-in) operating AWS resources:
Lambda, DynamoDB, and ECS. Read-only by default; every mutation gated behind
`--unsafe` and a confirm modal. Fast keyboard-driven navigation modeled on k9s:
a per-mode navigation stack, breadcrumbs, and contextual key hints.

## Architecture

- **Language / stack**: Go 1.24, [Bubble Tea](https://github.com/charmbracelet/bubbletea)
  + bubbles (list/viewport/spinner/textinput/help/key), lipgloss, AWS SDK for Go v2.
- **Entry**: `cmd/awsctl` — flags, logger (file sink; TUI owns stdout), `--version` (#24).
- **AWS clients**: `internal/aws/` — one wrapper per service (`lambda.go`,
  `dynamo.go`, `logs.go`, `ecs.go`) returning flat summary structs.
- **UI shell**: `internal/ui/app.go` — per-mode `core.Stack`s, global mode
  switch (`1` Lambda / `2` Dynamo / `3` ECS), profile picker (`p`), help (`?`).
- **Nav core**: `internal/ui/core/` — `Screen` interface, `Stack` (LIFO per
  mode, breadcrumbs), `PushMsg`/`PopMsg`, optional `InputCapturer`/`EscHandler`,
  `Hint` helper. Leaf package (no import cycles).
- **Mode packages**: `internal/ui/{lambda,dynamo,ecs}/` — value models + pointer
  `Screen` adapters; drill-ins exposed as `OpenX(cfg) core.Screen` builders.
- **Key conventions**: shared types live in `core` only; commit style is
  Conventional Commits with **no AI attribution**; one ticket → one PR → CI green
  → squash-merge.

## CI

`.github/workflows/ci.yml` — gates every PR:
1. `test` — build / vet / test matrix (ubuntu + macos, Go 1.24). [#28]
2. `integration` — LocalStack up → seed → `go test -tags=integration ./...`. [#31]
3. `goreleaser` — snapshot build with version ldflags. [#29]

## Milestone roadmap

Legend: ✅ done · 🔲 open · ⚠️ tracker-open but functionally delivered (close out).

### M1 — Lambda (core delivered; enhancements open)
Base list / detail / invoke / logs ship today. Open polish:
- 🔲 Invoke: `$EDITOR` escape (#7), qualifier picker (#8), type toggle (#9), save response (#10)
- 🔲 Detail: code download (#11), layer drill (#12), bg refresh (#13), policy highlight (#14)
- 🔲 List: column view (#15), state badge (#16), multi-region scan (#17)
- 🔲 Logs: time-window picker (#18)

### M2 — Metrics as charts (delivered on feat/m2-metrics, PR #80)
- ✅ CloudWatch `GetMetricData` client (#64)
- ✅ Reusable ntcharts chart component (#65), time-range selector (#66)
- ✅ Lambda detail metrics (#1, #67), Dynamo metrics (#68), ECS metrics (#69)

### M3 — DynamoDB (core done)
- ✅ list + describe (#2), scan + query (#3), item view (#4)

### M4 — PartiQL / export (not started)
- 🔲 PartiQL editor screen (#5)
- 🔲 Export results to CSV / NDJSON (#6)

### M5 — Release / quality (mostly done)
- ✅ `--version` via ldflags (#24) · ✅ CI (#28) · ✅ release workflow (#29) · ✅ LocalStack integration CI (#31)
- 🔲 `awsctl doctor` (#25), `--log-level` flag (#26), teatest snapshot tests (#27), release workflow follow-up (#30)

### M6 — Writes, `--unsafe` (complete)
- ✅ Confirm modal + audit log writer (#40) — gate in place
- ✅ Lambda: env vars (#32), memory/timeout (#33), publish+alias (#34), delete (#35)
- ✅ Dynamo: Put/Update/Delete (#36), BatchWrite (#37), Create/DeleteTable (#38), PartiQL writes (#39)
- ✅ ECS: scale (#57), force deploy (#58), stop task (#59), update revision (#60)

### M7 — ECS (read surface complete)
- ✅ client wrapper (#41⚠️), mode + tab (#42), cluster list (#43⚠️), cluster describe (#44)
- ✅ service list (#45), describe (#46), events (#47)
- ✅ task list (#48), describe (#49), container list (#50)
- ✅ task-def list (#51), describe (#52), revision history + diff (#53)
- ✅ container log tail (#54), ECS Exec (#55), rollout watch (#56)
- ✅ ECS metrics (#69, via M2)
- 🔲 breadcrumb nav polish (#61), multi-cluster/region scan (#62)

### Cross-cutting UI
- ✅ k9s nav-stack refactor (#63), help overlay (#19)
- 🔲 error modal expand (#20), persist profile/region (#21), cancel in-flight (#22), pagination indicator (#23)

## Current status

Read surface across **Lambda, DynamoDB, and ECS** is functional and CI-gated.
ECS drill (cluster → service → task → container) with describes, logs, exec,
rollout watch, events, task-defs, and revision diff is complete. Release
provenance (`--version`) and integration CI are in place.

## Next up (recommended order)

1. **#30 release workflow** finish → cut first tagged release (now that #24 lands).
2. **#43 / #41 cleanup** — close the stale-open ECS tickets already delivered.
3. **UI polish** (#21 persist profile/region, #23 pagination, #20 error modal).
4. **M4 PartiQL / export** (#5 editor, #6 CSV/NDJSON) — self-contained.

> M2 metrics (#64–#69, PR #80) and all of M6 (#40 + writes #32–#39/#57–#60, PRs #81/#83) merged.

## Validation (per layer)

Used by the lead/agents to validate before "done":

```sh
# all layers
go build ./... && go vet ./... && go test ./...

# integration (needs LocalStack)
docker compose -f docker-compose.localstack.yml up -d
./scripts/seed-localstack.sh
AWSCTL_ENDPOINT_URL=http://localhost:4566 AWS_ACCESS_KEY_ID=test \
  AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1 \
  go test -tags=integration ./...

# version / provenance
go build -o /tmp/awsctl ./cmd/awsctl && /tmp/awsctl --version
```

Acceptance for any ticket: build + vet + test green, integration job green on the
PR, behavior matches the issue's scope, no AI attribution in commits/PRs.
