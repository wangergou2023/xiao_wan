# XiaoWan

这个目录存放当前基于 wire-pod / chipper 构建的小丸运行时。

如果你想先看完整架构说明，请从这里开始：

- `chipper/pkg/xiaowan/FRAMEWORK.md`

如果你更想先看排障和调试方法，请读：

- `chipper/pkg/xiaowan/TROUBLESHOOTING.md`

## 快速地图

### 运行时入口

- 服务启动入口：`chipper/pkg/initwirepod/startserver.go`
- 兼容层门面：`chipper/pkg/xiaowan/ttr/facade.go`

### 主要包分组

- `preqs`
  - 最早期的语音请求处理
- `stt`
  - 语音转文本集成
- `speechrequest`
  - 请求整形 / 音频辅助
- `ttr`
  - text-to-response 响应流水线
- `workspace`
  - 工作区文档 + todo 状态
- `memory`
  - 持久化长期记忆文档处理
- `vision`
  - 最近一次人脸观察上下文
- `skills`
  - 工作区 skill 加载
- `cron`
  - 定时提醒任务

## 当前方向

当前框架的核心重点是：

- 用工作区文档作为持久上下文
- 使用原生 function tool，而不是假的动作语法
- 用轻量 todo 规划支持多步骤任务
- 真实执行机器人动作：回充、拍照、烟花、后退
- 在正常对话里注入实时人脸上下文

## 最常改的文件

### Prompt / LLM 行为

- `chipper/pkg/xiaowan/ttr/llm/commands.go`
- `chipper/pkg/xiaowan/ttr/llm/stream.go`
- `chipper/pkg/xiaowan/ttr/llm/provider.go`

### 原生工具

- 注册表：`chipper/pkg/xiaowan/ttr/llm/native_tools.go`
- 执行逻辑：`chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`

### Todo 规划

- 状态：`chipper/pkg/xiaowan/workspace/todo.go`
- 自动规划启发式：`chipper/pkg/xiaowan/ttr/llm/todo_planning.go`

### 记忆 / 工作区上下文

- 工作区加载：`chipper/pkg/xiaowan/workspace/loader.go`
- 长期记忆文档处理：`chipper/pkg/xiaowan/memory/profile.go`

### 人脸上下文

- `chipper/pkg/xiaowan/vision/faces.go`

### 机器人直接动作

- 直接工具强制路由：`chipper/pkg/xiaowan/ttr/llm/direct_action.go`
- 机器人执行：`chipper/pkg/xiaowan/ttr/robot/controller.go`

## 建议阅读顺序

如果你第一次看这套代码，建议按这个顺序：

1. `chipper/pkg/xiaowan/FRAMEWORK.md`
2. `chipper/pkg/initwirepod/startserver.go`
3. `chipper/pkg/xiaowan/ttr/llm/stream.go`
4. `chipper/pkg/xiaowan/ttr/llm/native_tools.go`
5. `chipper/pkg/xiaowan/ttr/llm/tool_runtime.go`
6. `chipper/pkg/xiaowan/workspace/loader.go`
7. `chipper/pkg/xiaowan/workspace/todo.go`
