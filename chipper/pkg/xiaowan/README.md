# XiaoWan

This directory contains the current XiaoWan runtime built on top of wire-pod / chipper.

If you want the full architecture write-up, start here:

- `chipper/pkg/xiaowan/FRAMEWORK.md`

If you want practical debugging guidance, read:

- `chipper/pkg/xiaowan/TROUBLESHOOTING.md`

## Quick Map

### Runtime entry

- server bootstrap: `chipper/pkg/initwirepod/startserver.go`
- compatibility facade: `chipper/pkg/xiaowan/ttr/facade.go`

### Main package groups

- `preqs`
  - earliest speech request processing
- `stt`
  - speech-to-text integrations
- `speechrequest`
  - request shaping / audio helpers
- `ttr`
  - text-to-response pipeline
- `workspace`
  - workspace docs + todo state
- `memory`
  - durable long-term memory document handling
- `vision`
  - recent face observation context
- `skills`
  - workspace skill loading
- `cron`
  - scheduled reminder jobs

## Current Direction

The current framework is centered around:

- workspace documents as persistent context
- native function tools instead of fake action syntax
- lightweight todo planning for multi-step tasks
- real robot action execution for charge / photo / fireworks / back-away
- live face context injected into normal conversation

## Most Common Files To Change

### Prompt / LLM behavior

- `chipper/pkg/xiaowan/ttr/llm/commands.go`
- `chipper/pkg/xiaowan/ttr/llm/stream.go`
- `chipper/pkg/xiaowan/ttr/llm/provider.go`

### Native tools

- registry: `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
- execution: `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`

### Todo planning

- state: `chipper/pkg/xiaowan/workspace/todo.go`
- auto-planning heuristics: `chipper/pkg/xiaowan/ttr/llm/todo_planning.go`

### Memory / workspace context

- workspace loading: `chipper/pkg/xiaowan/workspace/loader.go`
- long-term memory doc handling: `chipper/pkg/xiaowan/memory/profile.go`

### Face context

- `chipper/pkg/xiaowan/vision/faces.go`

### Direct robot actions

- direct tool forcing: `chipper/pkg/xiaowan/ttr/llm/direct_action.go`
- robot execution: `chipper/pkg/xiaowan/ttr/robot/controller.go`

## Suggested Reading Order

If you are new to this code, read in this order:

1. `chipper/pkg/xiaowan/FRAMEWORK.md`
2. `chipper/pkg/initwirepod/startserver.go`
3. `chipper/pkg/xiaowan/ttr/llm/stream.go`
4. `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
5. `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
6. `chipper/pkg/xiaowan/workspace/loader.go`
7. `chipper/pkg/xiaowan/workspace/todo.go`
