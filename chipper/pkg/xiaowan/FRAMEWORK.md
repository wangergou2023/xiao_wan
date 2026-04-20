# XiaoWan 框架说明

本文档描述当前仓库里的 XiaoWan 代码框架。

它主要回答三个实际问题：

1. 运行时从哪里启动
2. 一次语音请求如何流经整个系统
3. 哪个包负责记忆、工作区、工具、规划和机器人控制

## 1. 顶层运行时

整个运行时仍然构建在 wire-pod / chipper 服务器栈之上。

- 服务启动入口：`chipper/pkg/initwirepod/startserver.go`
- 那里启动的主要服务层：
  - 配置 web 服务器
  - sdk app web 服务器
  - chipper gRPC + REST 服务
  - STT 请求处理器
  - cron 定时器
  - mDNS / 机器人发现

重要启动顺序如下：

- `BeginWirepodSpecific(...)`
  - 初始化日志
  - 通过 `vars.Init()` 加载配置
  - 通过 `cronpkg.Init()` 初始化 cron
  - 创建 preqs 语音处理器
  - 启动 sdk web 服务器
- `StartChipper()`
  - 加载 TLS 证书
  - 启动 chipper 的 gRPC / HTTP 监听器
  - 把运行时暴露给机器人和初始化客户端

## 2. 包结构

当前 XiaoWan 相关逻辑主要在 `chipper/pkg/xiaowan` 下面。

### 核心包

- `chipper/pkg/xiaowan/preqs`
  - 最早期的请求处理层
  - 接收语音 / STT 侧请求并继续向下传递
- `chipper/pkg/xiaowan/ttr`
  - "text to response"
  - 负责 intent 分发、LLM 编排、机器人执行
- `chipper/pkg/xiaowan/stt`
  - STT 引擎集成
- `chipper/pkg/xiaowan/speechrequest`
  - 请求整形与音频预处理辅助

### 状态与上下文包

- `chipper/pkg/xiaowan/workspace`
  - 工作区文件发现与 prompt 上下文组装
  - TODO 状态持久化
- `chipper/pkg/xiaowan/memory`
  - 长期记忆文档管理
- `chipper/pkg/xiaowan/vision`
  - 实时人脸上下文观察与 prompt 注入
- `chipper/pkg/xiaowan/skills`
  - 工作区 skill 加载
- `chipper/pkg/xiaowan/cron`
  - 定时提醒 / 任务执行

### 响应 / 机器人执行包

- `chipper/pkg/xiaowan/ttr/llm`
  - 构建 prompt
  - 创建 chat 请求
  - 管理原生 tool call
  - 把 LLM 流式输出转成播报
- `chipper/pkg/xiaowan/ttr/robot`
  - 机器人侧直接行为控制
  - TTS、动作、动画、回充、拍照、烟花、中断处理
- `chipper/pkg/xiaowan/ttr/intent`
  - 兼容旧 intent 的处理与参数解析

### 更底层的机器人访问

- `chipper/pkg/vector`
  - Vector 连接 / 会话封装
- `chipper/pkg/sdk-wrapper`
  - 对 SDK 能力做一层薄封装：电机、相机、人脸、设置、语音等

## 3. 语音请求流

当前高层流程：

1. 音频到达
2. STT 转成文本
3. `preqs` 把文本传给响应层
4. `ttr/llm` 构建请求并流式返回答案
5. 当前轮次中可能会执行原生工具
6. 文本通过机器人 TTS 说出来
7. 如果需要，延迟机器人动作会在后面执行
8. remembered chat / todo / workspace 文件被更新

### 重要入口

- 兼容门面：`chipper/pkg/xiaowan/ttr/facade.go`
- 主 LLM 响应入口：`chipper/pkg/xiaowan/ttr/llm/stream.go`
  - `StreamingKGSim(...)`

### 请求创建

`chipper/pkg/xiaowan/ttr/llm/stream.go` 里的 `CreateAIReq(...)` 会组装：

- 基础系统提示词
- 工作区 prompt 上下文
- 当前人脸上下文
- remembered chat 历史
- 可选的直接动作强制 prompt
- 可选的自动 todo 规划 prompt
- 当前用户消息

然后在开启命令支持时启用原生 function tool。

## 4. Prompt 架构

Prompt 构建核心在：

- `chipper/pkg/xiaowan/ttr/llm/commands.go`

当前 prompt 层次：

- 基础 assistant / system prompt
- 运行时语音规则
- 工作区文档上下文
- skill 目录 / 激活 skill
- 机器人运行时规则
- 长期记忆上下文
- 实时人脸上下文

### 工作区 prompt 上下文

由这里负责：

- `chipper/pkg/xiaowan/workspace/loader.go`

它按优先级读取：

- `workspace/AGENTS.md`
- `workspace/IDENTITY.md`
- `workspace/BOOTSTRAP.md`
- `workspace/SOUL.md`
- `workspace/USER.md`
- `workspace/memory/MEMORY.md`
- `workspace/TODO.md`

### 长期记忆

由这里负责：

- `chipper/pkg/xiaowan/memory/profile.go`

重要细节：

- 持久化记忆现在以 `workspace/memory/MEMORY.md` 为唯一事实来源
- `memory.BuildPromptContext(...)` 当前返回空，因为记忆内容已经通过 workspace 文档注入到 prompt 里

### 人脸上下文

由这里负责：

- `chipper/pkg/xiaowan/vision/faces.go`

行为：

- 每个机器人 ESN 启动一个 watcher
- 监听人脸观察事件
- 在内存里保留最近一次观察到的人
- 给 LLM 注入一个很短的“我面前是谁”上下文

这取代了旧的“检测到人脸就主动打招呼”，改成在正常对话里具备身份感知能力。

## 5. 原生工具框架

当前框架是 native-tool-first。

主要文件：

- 工具注册表：`chipper/pkg/xiaowan/ttr/llm/native_tools.go`
- 工具执行逻辑：`chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`

### 当前工具分类

- 环境：
  - `get_current_time`
  - `weather`
- 定时调度：
  - `cron_add`
  - `cron_list`
  - `cron_remove`
- 规划：
  - `todo_read`
  - `todo_write`
  - `todo_update`
  - `todo_clear`
- 机器人直接动作：
  - `go_charge`
  - `take_photo`
  - `celebrate_fireworks`
  - `back_away`
- 工作区 / 文件 / shell 工具：
  - `read_file`
  - `write_file`
  - `edit_file`
  - `list_dir`
  - `system_cmd`

### 工具执行模型

`native_tools.go` 里的 `executeNativeToolCalls(...)` 会：

- 解析工具名和别名
- 校验工作区文档保护规则
- 执行工具
- 收集工具结果，作为 assistant 可见的 tool message
- 如果真实动作需要在播报后执行，则返回延迟机器人动作

### 工作区修改保护

关键工作区文档是受保护的。

某些文件必须先读后改：

- `workspace/AGENTS.md`
- `workspace/IDENTITY.md`
- `workspace/SOUL.md`
- `workspace/USER.md`
- `workspace/memory/MEMORY.md`

这部分逻辑在：

- `native_tools.go` 里的 `guardWorkspaceDocMutation(...)`

## 6. Todo / 规划层

这是新的规划层，给机器人一个轻量“工作记忆”，用于多步骤任务。

主要文件：

- `chipper/pkg/xiaowan/workspace/todo.go`

磁盘文件：

- 机器可读：`workspace/state/todo.json`
- 人类可读：`workspace/TODO.md`

### 状态模型

`TodoState` 包含：

- `active_plan`
- `recent_plans`
- `updated_at`

每个 plan 包含：

- `goal`
- `status`
- 有序 `steps`
- 时间戳

### 行为

- 多步骤请求可以通过 `todo_write` 创建计划
- 进度通过 `todo_update` 记录
- 当所有步骤完成时，active plan 会自动归档
- `todo_clear` 在清空前也会先归档
- 归档计划会保留在 `recent_plans`

### 自动规划启发式

由这里负责：

- `chipper/pkg/xiaowan/ttr/llm/todo_planning.go`

它会在这些情况下给 LLM 增加规划提示：

- 用户请求的是多步骤工作
- 已经存在 active plan，且用户说“继续”

这是提示驱动的，不是额外跑了一个 planner 守护进程。

## 7. 直接动作路由

对于一些非常明确的请求，当前框架不会只依赖模型偏好。

由这里负责：

- `chipper/pkg/xiaowan/ttr/llm/direct_action.go`

当前能强制工具选择的直接动作检测包括：

- 烟花
- 回充
- 拍照
- 后退

这意味着像“放烟花”或“回家充电去吧”这类请求，可以直接路由到特定 native tool，而不是赌模型会不会自己选对。

## 8. 流式响应流水线

主要文件：

- `chipper/pkg/xiaowan/ttr/llm/stream.go`

单轮对话中的核心行为：

- 创建 chat stream
- 累积文本 delta
- 累积 tool call delta
- 清洗输出
- 切分成可播报片段
- 预取后续片段的 TTS 音频
- 在流结束后执行 native tool
