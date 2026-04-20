# XiaoWan 排障手册

本文档用于运行时调试，不是架构学习文档。

如果你想先看整体设计，请读：

- `chipper/pkg/xiaowan/FRAMEWORK.md`

如果你想先看简短包地图，请读：

- `chipper/pkg/xiaowan/README.md`

## 1. 第一轮检查

在追具体 bug 之前，先确认这些基础点：

- 服务确实是从 `chipper/pkg/initwirepod/startserver.go` 启动的
- 机器人已经被发现并连接
- 当前配置的 LLM / STT 提供商可以访问
- 运行时预期位置的 `workspace/` 文件存在
- 请求真的走进了 LLM 路径，而不是还被旧 intent 路径吃掉了

日志中比较好的启动信号：

- `wire-pod started successfully!`
- `Vector discovered on network`
- `Making LLM request for device ...`
- 当前模型的 `Using ...`

## 2. 现象：机器人会说话，但没有执行工具动作

例如：

- 用户说“放烟花”，但是没放烟花
- 用户说“回家充电”，但它只口头回答

### 要检查什么

- 直接动作路由：
  - `chipper/pkg/xiaowan/ttr/llm/direct_action.go`
- 请求组装：
  - `chipper/pkg/xiaowan/ttr/llm/stream.go`
- 原生工具注册表：
  - `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
- 工具执行：
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- 机器人动作执行：
  - `chipper/pkg/xiaowan/ttr/robot/controller.go`

### 有用日志

- `LLM native tool calls: ...`
- `LLM action executing: goCharge ...`
- `LLM action executing: celebrateFireworks ...`
- `Native tool call request failed, retrying without native tools: ...`

### 常见原因

- 请求没有匹配到直接动作启发式
- 在非强制场景下，模型忽略了 `auto` 工具选择
- 工具调用发生了，但机器人侧 RPC 失败
- 没拿到 behavior control，或者被中断了

### 当前预期行为

对于明显请求，现在这些应该优先强制用 native tool：

- 烟花
- 回充
- 拍照
- 后退

## 3. 现象：机器人说出奇怪的假动作文本

例如：

- `celebrateFireworks||now}}`
- 旧的假动作语法残留进播报里

### 要检查什么

- 输出清洗：
  - `chipper/pkg/xiaowan/ttr/llm/sanitize.go`
- 流式累积：
  - `chipper/pkg/xiaowan/ttr/llm/stream.go`
- 句子切分：
  - `chipper/pkg/xiaowan/ttr/llm/sentence_split.go`

### 有用日志

- `LLM delta: ...`
- `LLM final raw: ...`
- `LLM final slices: ...`

### 常见原因

- 模型输出了过时的旧动作文本，而不是 tool call
- 流式 delta 把垃圾文本切碎后分散到多个块里
- 早期清洗逻辑只清掉了一部分

### 当前预期

系统现在应该能清理：

- 完整的假动作块
- 残缺尾巴，例如 `...||now}}`
- 裸露的假动作字符串，例如 `celebrateFireworks||now`

## 4. 现象：工具调用发生了，但后续播报很差或者丢了

例如：

- 工具执行了，但机器人后面没说话
- 工具 follow-up 话术僵硬，或者意外回退

### 要检查什么

- 工具 follow-up 生成：
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- 主流结束逻辑：
  - `chipper/pkg/xiaowan/ttr/llm/stream.go`

### 有用日志

- `LLM tool follow-up failed: ...`
- `Using local tool follow-up fallback`
- `LLM stream finished`

### 常见原因

- 提供商侧 follow-up 请求失败
- follow-up 请求触发 429 或限流
- fallback 总结器只拿到了很有限的工具结果

## 5. 现象：嘴上说“记住了”，但实际上没写进记忆

例如：

- 机器人说记住了某个偏好
- 之后再问却答不出来

### 要检查什么

- 工作区长期记忆事实来源：
  - `chipper/workspace/memory/MEMORY.md`
- 记忆文档构建：
  - `chipper/pkg/xiaowan/memory/profile.go`
- 工作区 prompt 加载：
  - `chipper/pkg/xiaowan/workspace/loader.go`
- 文件工具执行：
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- 工作区保护规则：
  - `chipper/pkg/xiaowan/ttr/llm/native_tools.go`

### 有用日志

- `LLM native tool calls: ... read_file ... edit_file ...`
- `before editing a key workspace document, call read_file on the same path first`
- `Using remembered chats, length of ...`

### 常见原因

- 模型只是嘴上说“我记住了”，但根本没调文件工具
- 它试图修改 `MEMORY.md`，但没先读文件
- 早期 workspace 路径错误或缺失
- 只记住了短期聊天历史，没有写入持久记忆

### 当前设计规则

持久长期记忆放在：

- `workspace/memory/MEMORY.md`

短期聊天记忆在别处，它和长期记忆不是同一回事。

## 6. 现象：工作区文件编辑被拒绝

### 要检查什么

- 保护逻辑：
  - `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
- 文件工具实现：
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`

### 常见报错

- `before editing a key workspace document, call read_file on the same path first`

### 为什么会这样

这些关键文档受保护：

- `workspace/AGENTS.md`
- `workspace/IDENTITY.md`
- `workspace/SOUL.md`
- `workspace/USER.md`
- `workspace/memory/MEMORY.md`

模型必须先读一遍，再允许替换内容。

## 7. 现象：Todo 计划没有更新

例如：

- 多步骤任务开始了，但 todo 文件没变
- 进度没有推进到 completed
- 计划一直卡在 active

### 要检查什么

- todo 状态：
  - `chipper/workspace/state/todo.json`
- 人类可读 todo：
  - `chipper/workspace/TODO.md`
- 运行时状态逻辑：
  - `chipper/pkg/xiaowan/workspace/todo.go`
- 工具实现：
  - `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
- 自动规划启发式：
  - `chipper/pkg/xiaowan/ttr/llm/todo_planning.go`

### 有用日志 / payload 提示

- `todo_write`
- `todo_update`
- `todo_clear`
- `plan_status`
- `notice`

### 常见原因

- 请求太简单，没有触发 todo 规划提示
- 模型一口气做完，没有走 planning
- `todo_update` 用错了 `step_id`
- active plan 一直没到 completed 或 cancelled 状态

### 当前行为

- 已完成计划应该自动归档
- `todo_clear` 应该先归档再清空
- 归档计划存放在 `recent_plans`

## 8. 现象：“继续”没有很好地续上之前的任务

### 要检查什么

- 恢复启发式：
  - `chipper/pkg/xiaowan/ttr/llm/todo_planning.go`
- 磁盘上的 active plan：
  - `chipper/workspace/state/todo.json`

### 常见原因

- 根本没有 active plan 可恢复
- 旧任务已经自动完成并移动到 `recent_plans`
- 当前“继续”这句话没有足够强地匹配恢复启发式

## 9. 现象：人脸识别上下文缺失

例如：

- 机器人看到了人，但 LLM 不知道当前面前是谁

### 要检查什么

- 人脸 watcher：
  - `chipper/pkg/xiaowan/vision/faces.go`
- sdk 人脸支持：
  - `chipper/pkg/sdk-wrapper/sdk-wrapper-faces.go`

### 有用日志

- `Face watcher for ... restarting after vision mode reset`
- `Face watcher for ... stopped, retrying: ...`

### 常见原因

- 配置里关掉了 face context
- 最近一次人脸事件超出了 TTL
- watcher 因 vision mode reset 重启
- 那张脸没有已保存的名字

### 当前设计

现在系统把人脸上下文作为正常对话里的 prompt context，而不是检测到人脸就主动打招呼。

## 10. 现象：机器人一直处于“busy”状态

例如：

- 人脸触发或被动动作一直延迟
- foreground interaction cooldown 一直挡住后续行为

### 要检查什么

- foreground activity 跟踪：
  - `chipper/pkg/xiaowan/ttr/robot/state.go`
- LLM 流生命周期：
  - `chipper/pkg/xiaowan/ttr/llm/stream.go`

### 有用日志

- `Foreground activity begin for ...`
- `Foreground activity end for ...`
- `Foreground activity stale auto-release ...`

### 常见原因

- 系统仍然认为有交互在进行中
- 某个陈旧 stream 或 TTS 会话没及时释放
- 机器人侧 behavior control 还被占着

## 11. 现象：TTS 慢、坏、或者听起来不对

### 要检查什么

- TTS 预取：
  - `chipper/pkg/xiaowan/ttr/llm/tts_prefetch.go`
- big-model TTS 路径：
  - `chipper/pkg/xiaowan/ttr/llm/tts_bigmodel.go`
