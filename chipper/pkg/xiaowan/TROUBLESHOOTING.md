# XiaoWan Troubleshooting

This document is for runtime debugging, not architecture study.

If you want the overall design first, read:

- `chipper/pkg/xiaowan/FRAMEWORK.md`

If you want the short package map, read:

- `chipper/pkg/xiaowan/README.md`

## 1. First Checks

Before chasing a specific bug, verify these basics:

- the server actually started from `chipper/pkg/initwirepod/startserver.go`
- the robot is discovered and connected
- the configured LLM/STT provider is reachable
- `workspace/` files exist where the runtime expects them
- the request is really reaching the LLM path and not being handled by some older intent path

Good startup signs in logs:

- `wire-pod started successfully!`
- `Vector discovered on network`
- `Making LLM request for device ...`
- `Using ...` for the active model

## 2. Symptom: The Robot Speaks, But No Tool Action Happens

Examples:

- user says "放烟花" but no fireworks
- user says "回家充电" but it only replies in speech

### What to check

- direct action routing:
  - `chipper/pkg/xiaowan/ttr/llm/direct_action.go`
- request assembly:
  - `chipper/pkg/xiaowan/ttr/llm/stream.go`
- native tool registry:
  - `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
- tool execution:
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- robot action execution:
  - `chipper/pkg/xiaowan/ttr/robot/controller.go`

### Useful log lines

- `LLM native tool calls: ...`
- `LLM action executing: goCharge ...`
- `LLM action executing: celebrateFireworks ...`
- `Native tool call request failed, retrying without native tools: ...`

### Likely causes

- the request did not match the direct-action heuristic
- the model ignored `auto` tool choice on a non-forced request
- the tool call happened but the robot-side RPC failed
- behavior control could not be acquired or was interrupted

### Current expected behavior

For obvious requests, these should now prefer forced native tools:

- fireworks
- charge
- photo
- back away

## 3. Symptom: The Robot Says Weird Fake Action Text

Examples:

- `celebrateFireworks||now}}`
- leftover fake action syntax leaking into speech

### What to check

- output cleanup:
  - `chipper/pkg/xiaowan/ttr/llm/sanitize.go`
- streaming accumulation:
  - `chipper/pkg/xiaowan/ttr/llm/stream.go`
- speech chunk splitting:
  - `chipper/pkg/xiaowan/ttr/llm/sentence_split.go`

### Useful log lines

- `LLM delta: ...`
- `LLM final raw: ...`
- `LLM final slices: ...`

### Likely causes

- the model emitted stale old-style action text instead of a tool call
- streamed deltas split the garbage output across chunks
- sanitizing removed only part of the text before the recent fixes

### Current expectation

The system should now strip:

- complete fake action blocks
- orphan tails like `...||now}}`
- bare fake action strings like `celebrateFireworks||now`

## 4. Symptom: Tool Call Happened, But Follow-Up Speech Is Bad or Missing

Examples:

- tool runs, but the robot says nothing after
- tool follow-up becomes stiff or falls back unexpectedly

### What to check

- tool follow-up generation:
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- main stream completion logic:
  - `chipper/pkg/xiaowan/ttr/llm/stream.go`

### Useful log lines

- `LLM tool follow-up failed: ...`
- `Using local tool follow-up fallback`
- `LLM stream finished`

### Likely causes

- provider-side follow-up request failed
- rate limit or 429 during the follow-up request
- fallback summarizer had only limited tool payload to work with

## 5. Symptom: Memory Was "Remembered" in Speech, But Not Actually Saved

Examples:

- robot says it remembers a preference
- later it cannot answer from memory

### What to check

- workspace memory source of truth:
  - `chipper/workspace/memory/MEMORY.md`
- memory doc builder:
  - `chipper/pkg/xiaowan/memory/profile.go`
- workspace prompt loading:
  - `chipper/pkg/xiaowan/workspace/loader.go`
- file tool execution:
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- workspace guards:
  - `chipper/pkg/xiaowan/ttr/llm/native_tools.go`

### Useful log lines

- `LLM native tool calls: ... read_file ... edit_file ...`
- `before editing a key workspace document, call read_file on the same path first`
- `Using remembered chats, length of ...`

### Likely causes

- the model talked about remembering but never used file tools
- it tried to edit `MEMORY.md` without first reading it
- the workspace path was wrong or missing earlier
- chat history was remembered, but durable memory was not written

### Current design rule

Durable memory lives in:

- `workspace/memory/MEMORY.md`

Short chat memory lives elsewhere and is not the same thing.

## 6. Symptom: Workspace File Edit Was Rejected

### What to check

- guard logic:
  - `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
- file tool implementation:
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`

### Common rejection

- `before editing a key workspace document, call read_file on the same path first`

### Why it happens

Key docs are protected:

- `workspace/AGENTS.md`
- `workspace/IDENTITY.md`
- `workspace/SOUL.md`
- `workspace/USER.md`
- `workspace/memory/MEMORY.md`

The model must first inspect them before replacing content.

## 7. Symptom: Todo Plan Did Not Update

Examples:

- multi-step task started but no todo file changed
- progress did not move to completed
- plan stayed active forever

### What to check

- todo state:
  - `chipper/workspace/state/todo.json`
- readable todo:
  - `chipper/workspace/TODO.md`
- runtime state logic:
  - `chipper/pkg/xiaowan/workspace/todo.go`
- tool implementation:
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- auto-planning heuristics:
  - `chipper/pkg/xiaowan/ttr/llm/todo_planning.go`

### Useful log / payload hints

- `todo_write`
- `todo_update`
- `todo_clear`
- `plan_status`
- `notice`

### Likely causes

- the request was too simple, so no todo plan was encouraged
- the model handled the task in one shot without planning
- `todo_update` used the wrong `step_id`
- the active plan never reached a fully completed or cancelled state

### Current behavior

- completed plans should auto-archive
- `todo_clear` should archive before clearing
- archived plans live in `recent_plans`

## 8. Symptom: "Continue" Does Not Resume the Previous Task Well

### What to check

- resume heuristics:
  - `chipper/pkg/xiaowan/ttr/llm/todo_planning.go`
- active plan on disk:
  - `chipper/workspace/state/todo.json`

### Likely causes

- there was no active plan to resume
- the old task had already auto-finished and moved to `recent_plans`
- the phrase did not match current resume heuristics strongly enough

## 9. Symptom: Face Recognition Context Is Missing

Examples:

- the robot sees a person but the LLM does not know who is in front of it

### What to check

- face watcher:
  - `chipper/pkg/xiaowan/vision/faces.go`
- sdk face support:
  - `chipper/pkg/sdk-wrapper/sdk-wrapper-faces.go`

### Useful log lines

- `Face watcher for ... restarting after vision mode reset`
- `Face watcher for ... stopped, retrying: ...`

### Likely causes

- face context feature disabled in config
- no recent face event within TTL
- watcher restarted due to vision mode reset
- the face had no saved name

### Current design

The system now uses face context as prompt context during normal conversation, not proactive face-triggered greeting.

## 10. Symptom: The Robot Is Always "Busy"

Examples:

- face-triggered or passive actions get delayed
- foreground interaction cooldown seems to block follow-up behavior

### What to check

- foreground activity tracking:
  - `chipper/pkg/xiaowan/ttr/robot/state.go`
- LLM stream lifecycle:
  - `chipper/pkg/xiaowan/ttr/llm/stream.go`

### Useful log lines

- `Foreground activity begin for ...`
- `Foreground activity end for ...`
- `Foreground activity stale auto-release ...`

### Likely causes

- the system still thinks an interaction is in progress
- a stale stream or TTS session was not released promptly
- robot-side behavior control remained occupied

## 11. Symptom: TTS Is Slow, Broken, or Sounds Wrong

### What to check

- TTS prefetch:
  - `chipper/pkg/xiaowan/ttr/llm/tts_prefetch.go`
- big-model TTS path:
  - `chipper/pkg/xiaowan/ttr/llm/tts_bigmodel.go`
- audio playback / conversion:
  - `chipper/pkg/xiaowan/ttr/llm/tts_audio.go`
  - `chipper/pkg/xiaowan/ttr/robot/audio_convert.go`
  - `chipper/pkg/xiaowan/ttr/robot/voice.go`
- text cleanup:
  - `chipper/pkg/xiaowan/ttr/llm/sanitize.go`

### Useful log lines

- `BigModel TTS prefetch failed ...`
- `BigModel TTS failed, falling back to SDK voice: ...`

### Likely causes

- big-model TTS request failed
- prefetch cache miss or error
- text cleanup still produced awkward speech text
- SDK voice fallback handled the text differently

## 12. Symptom: LLM Request Works Sometimes, Then Falls Back or Errors

### What to check

- provider setup:
  - `chipper/pkg/xiaowan/ttr/llm/provider.go`
- request creation:
  - `chipper/pkg/xiaowan/ttr/llm/stream.go`

### Useful log lines

- `Using ...`
- `Error creating chat completion stream: ...`
- `Native tool call request failed, retrying without native tools: ...`

### Likely causes

- provider/model mismatch
- bad API key or endpoint
- tool-capable request failed so the runtime retried without tools
- rate limiting during main request or tool follow-up

## 13. Symptom: Fireworks Tool Called, But Robot Still Did Not Move

### What to check

- fireworks execution:
  - `chipper/pkg/xiaowan/ttr/robot/controller.go`
- LLM-side deferred execution:
  - `chipper/pkg/xiaowan/ttr/llm/commands.go`

### Useful log lines

- `CelebrateFireworks: requesting behavior control ...`
- `CelebrateFireworks: control granted ...`
- `CelebrateFireworks: play animation failed ...`
- `CelebrateFireworks: control release failed ...`

### Likely causes

- behavior control RPC timeout
- robot-side connection reset mid-animation
- execution was scheduled but the SDK call failed

## 14. Best Debugging Order

When a bug appears, debug in this order:

1. Did the request reach the LLM path?
2. Did the prompt include the right context?
3. Did the model emit a tool call?
4. If not, was the request supposed to force one?
5. If yes, did tool execution succeed?
6. If yes, did robot-side RPC execution succeed?
7. If yes, did post-tool speech or state update fail?

That order usually narrows the problem fast.

## 15. Most Useful Files During Debugging

- request assembly:
  - `chipper/pkg/xiaowan/ttr/llm/stream.go`
- prompt rules:
  - `chipper/pkg/xiaowan/ttr/llm/commands.go`
- provider/model choice:
  - `chipper/pkg/xiaowan/ttr/llm/provider.go`
- tool registry:
  - `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
- tool execution:
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- direct action forcing:
  - `chipper/pkg/xiaowan/ttr/llm/direct_action.go`
- todo state:
  - `chipper/pkg/xiaowan/workspace/todo.go`
- workspace prompt loading:
  - `chipper/pkg/xiaowan/workspace/loader.go`
- long-term memory:
  - `chipper/pkg/xiaowan/memory/profile.go`
- face context:
  - `chipper/pkg/xiaowan/vision/faces.go`
- robot execution:
  - `chipper/pkg/xiaowan/ttr/robot/controller.go`
- foreground activity / busy tracking:
  - `chipper/pkg/xiaowan/ttr/robot/state.go`
