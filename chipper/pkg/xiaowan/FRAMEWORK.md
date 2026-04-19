# XiaoWan Framework

This document describes the current XiaoWan code framework as it exists in this repo today.

It is meant to answer three practical questions:

1. Where the runtime starts
2. How a voice request flows through the system
3. Which package owns memory, workspace, tools, planning, and robot control

## 1. Top-Level Runtime

The runtime is still built on top of the wire-pod / chipper server stack.

- Server bootstrap: `chipper/pkg/initwirepod/startserver.go`
- Main service layers started there:
  - config web server
  - sdk app web server
  - chipper gRPC + REST service
  - STT request processor
  - cron scheduler
  - mDNS / robot discovery

The important entry sequence is:

- `BeginWirepodSpecific(...)`
  - initializes logging
  - loads config via `vars.Init()`
  - initializes cron via `cronpkg.Init()`
  - creates the preqs voice processor
  - starts the sdk web server
- `StartChipper()`
  - loads TLS certs
  - starts the chipper gRPC / HTTP listeners
  - exposes the runtime to the robot and setup clients

## 2. Package Layout

The current XiaoWan-specific logic lives mainly under `chipper/pkg/xiaowan`.

### Core packages

- `chipper/pkg/xiaowan/preqs`
  - earliest request-processing layer
  - receives speech/STT-side requests and passes them downstream
- `chipper/pkg/xiaowan/ttr`
  - "text to response"
  - owns intent dispatch, LLM orchestration, robot execution
- `chipper/pkg/xiaowan/stt`
  - STT engine integration
- `chipper/pkg/xiaowan/speechrequest`
  - request shaping and audio preparation helpers

### State and context packages

- `chipper/pkg/xiaowan/workspace`
  - workspace file discovery and prompt context assembly
  - TODO state persistence
- `chipper/pkg/xiaowan/memory`
  - long-term memory document management
- `chipper/pkg/xiaowan/vision`
  - live face context observation and prompt injection
- `chipper/pkg/xiaowan/skills`
  - workspace skill loading
- `chipper/pkg/xiaowan/cron`
  - scheduled reminder/job execution

### Response / robot execution packages

- `chipper/pkg/xiaowan/ttr/llm`
  - builds prompts
  - creates chat requests
  - manages native tool calls
  - streams LLM output into speech
- `chipper/pkg/xiaowan/ttr/robot`
  - direct robot-side behavior control
  - TTS, motion, animation, charger, photo, fireworks, interrupt handling
- `chipper/pkg/xiaowan/ttr/intent`
  - compatibility intent handling and parameter parsing

### Lower-level robot access

- `chipper/pkg/vector`
  - Vector connection/session wrapper
- `chipper/pkg/sdk-wrapper`
  - thin wrappers around SDK capabilities: motors, camera, faces, settings, voice, etc.

## 3. Voice Request Flow

Current high-level flow:

1. Audio arrives
2. STT converts it to text
3. `preqs` passes text into the response layer
4. `ttr/llm` builds the request and streams the answer
5. native tools may run during the turn
6. text is spoken through robot TTS
7. deferred robot actions run if needed
8. remembered chat / todo / workspace files are updated

### Important entry point

- Compatibility facade: `chipper/pkg/xiaowan/ttr/facade.go`
- Main LLM response entry: `chipper/pkg/xiaowan/ttr/llm/stream.go`
  - `StreamingKGSim(...)`

### Request creation

`CreateAIReq(...)` in `chipper/pkg/xiaowan/ttr/llm/stream.go` assembles:

- base system prompt
- workspace prompt context
- current face context
- remembered chat history
- optional direct-action forcing prompt
- optional auto-todo planning prompt
- current user message

Then it enables native function tools when command support is on.

## 4. Prompt Architecture

Prompt building is centered in:

- `chipper/pkg/xiaowan/ttr/llm/commands.go`

Current prompt layers:

- base assistant/system prompt
- runtime voice rules
- workspace document context
- skill catalog / active skills
- robot runtime rules
- long-term memory context
- live face context

### Workspace prompt context

Owned by:

- `chipper/pkg/xiaowan/workspace/loader.go`

It reads, in priority order:

- `workspace/AGENTS.md`
- `workspace/IDENTITY.md`
- `workspace/BOOTSTRAP.md`
- `workspace/SOUL.md`
- `workspace/USER.md`
- `workspace/memory/MEMORY.md`
- `workspace/TODO.md`

### Long-term memory

Owned by:

- `chipper/pkg/xiaowan/memory/profile.go`

Important detail:

- durable memory now uses `workspace/memory/MEMORY.md` as the source of truth
- `memory.BuildPromptContext(...)` currently returns empty because the memory content is already injected through workspace docs

### Face context

Owned by:

- `chipper/pkg/xiaowan/vision/faces.go`

Behavior:

- starts one watcher per robot ESN
- listens for face-observation events
- keeps the most recent observed person in memory
- injects a short "who is in front of me" prompt for the LLM

This replaces the old "auto greet on face detect" approach with contextual identity awareness during normal conversation.

## 5. Native Tool Framework

The current framework is native-tool-first.

Main files:

- tool registry: `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
- tool execution logic: `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`

### Current tool categories

- environment:
  - `get_current_time`
  - `weather`
- scheduler:
  - `cron_add`
  - `cron_list`
  - `cron_remove`
- planning:
  - `todo_read`
  - `todo_write`
  - `todo_update`
  - `todo_clear`
- direct robot actions:
  - `go_charge`
  - `take_photo`
  - `celebrate_fireworks`
  - `back_away`
- workspace / file / shell tools:
  - `read_file`
  - `write_file`
  - `edit_file`
  - `list_dir`
  - `system_cmd`

### Tool execution model

`executeNativeToolCalls(...)` in `native_tools.go`:

- resolves tool names and aliases
- validates workspace document guards
- executes tools
- collects tool results as assistant-visible tool messages
- returns deferred robot actions when a real action should happen after speech

### Workspace mutation guards

Key workspace docs are protected.

For some files, the model must read first before editing:

- `workspace/AGENTS.md`
- `workspace/IDENTITY.md`
- `workspace/SOUL.md`
- `workspace/USER.md`
- `workspace/memory/MEMORY.md`

That logic lives in:

- `guardWorkspaceDocMutation(...)` in `native_tools.go`

## 6. Todo / Planning Layer

This is the new planning layer that gives the robot a lightweight "working memory" for multi-step tasks.

Main file:

- `chipper/pkg/xiaowan/workspace/todo.go`

Files on disk:

- machine-readable: `workspace/state/todo.json`
- human-readable: `workspace/TODO.md`

### State model

`TodoState` contains:

- `active_plan`
- `recent_plans`
- `updated_at`

Each plan contains:

- `goal`
- `status`
- ordered `steps`
- timestamps

### Behavior

- multi-step requests can create a plan with `todo_write`
- progress is recorded with `todo_update`
- when all steps finish, the active plan is auto-archived
- `todo_clear` also archives before clearing
- archived plans are kept in `recent_plans`

### Auto-planning heuristics

Owned by:

- `chipper/pkg/xiaowan/ttr/llm/todo_planning.go`

It adds planning guidance when:

- the user asks for multi-step work
- there is already an active plan and the user says "continue"

This is guidance-driven, not a separate planner daemon.

## 7. Direct Action Routing

For some very clear requests, the current framework does not rely only on model preference.

Owned by:

- `chipper/pkg/xiaowan/ttr/llm/direct_action.go`

Current direct-action detection can force tool choice for:

- fireworks
- charging
- photo taking
- back-away movement

This means requests like "放烟花" or "回家充电去吧" can be routed to a specific native tool instead of hoping the model chooses correctly.

## 8. Streaming Response Pipeline

Main file:

- `chipper/pkg/xiaowan/ttr/llm/stream.go`

Core behavior during a turn:

- create chat stream
- accumulate text deltas
- accumulate tool call deltas
- sanitize output
- split output into speakable chunks
- prefetch TTS audio for later chunks
- execute native tools after stream completion
- optionally generate a post-tool follow-up answer
- remember conversation history

### Supporting pieces

- `sanitize.go`
  - cleans problematic output
  - strips old fake action text
- `sentence_split.go`
  - splits stream text into speech chunks
- `tts_prefetch.go`
  - preloads future TTS segments
- `session.go`
  - saves remembered chats to disk
- `provider.go`
  - chooses provider/model/client

## 9. Speech and Robot Execution

Robot-side execution is split between LLM orchestration and robot helpers.

### LLM action playback

- `chipper/pkg/xiaowan/ttr/llm/commands.go`

This file still owns:

- speech text execution
- lightweight animation and motor gesture helpers
- image capture action path
- deferred high-level action execution

Even though legacy prompt/action syntax is gone, this file still serves as the action dispatcher for internal `RobotAction` values.

### Robot control

- `chipper/pkg/xiaowan/ttr/robot/controller.go`
- `chipper/pkg/xiaowan/ttr/robot/voice.go`
- `chipper/pkg/xiaowan/ttr/robot/behavior.go`
- `chipper/pkg/xiaowan/ttr/robot/interrupt.go`

This layer owns:

- SDK speech
- big-model TTS playback fallback path
- behavior control acquisition/release
- charger/photo/fireworks/back-away execution
- touch / wake-word interrupt handling

## 10. Chat Memory

Conversation history is separate from long-term memory.

Owned by:

- `chipper/pkg/xiaowan/ttr/llm/session.go`

Behavior:

- remembers recent assistant/user/tool messages
- stores them under the chat history directory beside `ApiConfigPath`
- trims history to a bounded number of messages

So there are now three different state layers:

- chat history -> short conversational context
- workspace memory -> durable user facts
- todo state -> active task plan

## 11. What Is Removed / No Longer Central

The current framework no longer centers around:

- custom web intent definitions as the main control surface
- face-triggered proactive greeting behavior
- old brace-style fake action syntax in prompts
- legacy action catalogs injected into prompts

The current direction is:

- prompt + workspace context
- native tools
- todo planning
- direct robot actions through actual execution paths

## 12. Recommended Mental Model

If you are modifying the system, think in this order:

1. Is this durable user knowledge?
   - put it in workspace memory
2. Is this only for the current task?
   - put it in todo state
3. Is this only for the current conversation?
   - keep it in remembered chat
4. Is this a real robot-world action?
   - expose or use a native tool
5. Is this just prompt shaping?
   - change workspace docs or LLM prompt-building code

## 13. File Map For Common Work

### Add a new native function tool

- registry: `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
- execution: `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- prompt rule if needed: `chipper/pkg/xiaowan/ttr/llm/commands.go`
- tests: `chipper/pkg/xiaowan/ttr/llm/native_tools_test.go`

### Change task planning behavior

- runtime state: `chipper/pkg/xiaowan/workspace/todo.go`
- planning heuristics: `chipper/pkg/xiaowan/ttr/llm/todo_planning.go`
- tests: `chipper/pkg/xiaowan/ttr/llm/todo_planning_test.go`

### Change long-term memory behavior

- memory doc builder: `chipper/pkg/xiaowan/memory/profile.go`
- workspace loading: `chipper/pkg/xiaowan/workspace/loader.go`

### Change face identity context

- `chipper/pkg/xiaowan/vision/faces.go`

### Change direct robot actions

- LLM-side routing: `chipper/pkg/xiaowan/ttr/llm/direct_action.go`
- robot execution: `chipper/pkg/xiaowan/ttr/robot/controller.go`

## 13.5 Developer Quick Reference

If you want to change a specific behavior quickly, use this map:

- "Why did the model say this?"
  - `chipper/pkg/xiaowan/ttr/llm/commands.go`
  - `chipper/pkg/xiaowan/ttr/llm/stream.go`
- "Why did it call or not call a tool?"
  - `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
  - `chipper/pkg/xiaowan/ttr/llm/direct_action.go`
  - `chipper/pkg/xiaowan/ttr/llm/todo_planning.go`
- "Why did a file edit get blocked?"
  - `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- "Why did memory not show up in the prompt?"
  - `chipper/pkg/xiaowan/workspace/loader.go`
  - `chipper/pkg/xiaowan/memory/profile.go`
- "Why did todo not update or auto-finish?"
  - `chipper/pkg/xiaowan/workspace/todo.go`
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- "Why did the robot not physically move?"
  - `chipper/pkg/xiaowan/ttr/robot/controller.go`
  - `chipper/pkg/xiaowan/ttr/robot/behavior.go`
- "Why did the robot not know who was in front of it?"
  - `chipper/pkg/xiaowan/vision/faces.go`
- "Why did TTS or speech chunking behave strangely?"
  - `chipper/pkg/xiaowan/ttr/llm/sanitize.go`
  - `chipper/pkg/xiaowan/ttr/llm/sentence_split.go`
  - `chipper/pkg/xiaowan/ttr/llm/tts_prefetch.go`

## 14. Current Architecture Summary

In one sentence:

The current XiaoWan framework is a wire-pod-based voice runtime where workspace documents, remembered chat, live face context, native function tools, and a lightweight todo planner are combined into a streaming LLM loop that drives real Vector robot actions.
