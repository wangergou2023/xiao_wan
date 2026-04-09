# 语音请求处理链路

```
chipper/cmd/experimental/whisper/main.go
  -> initwirepod.StartFromProgramInit
  -> servers/chipper/intent_graph.go (StreamingIntentGraph 入口)
  -> wirepod/preqs/intent_graph.go (STT -> LLM -> SDK Wrapper)
  -> xiaowan/vector/vector.go (StreamingKGSim)
  -> xiaowan/chat (OpenAIchat 等)
  -> xiaowan/tts4 (LocalTTS)
```

说明：当前已移除“意图匹配/知识图谱”相关逻辑，语音文本直接进入 LLM，再通过 `sdk_wrapper` 播报/动作。
