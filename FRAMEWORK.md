# 代码框架说明（简版）

本文档用于帮助裁剪功能与代码，概述当前仓库的总体结构、关键模块职责与依赖关系。

## 1. 项目根目录

- `README.md`：项目介绍与安装说明（wire-pod）
- `readme2.md`：补充说明（如有）
- `setup.sh` / `update.sh` / `compose.yaml` / `dockerfile`：构建与部署脚本/配置
- `chipper/`：核心 Go 代码与资源（主模块）
- `certs/` / `images/`：证书与图片资源

## 2. 核心模块（`chipper/`）

- `go.mod` / `go.sum`：Go 模块与依赖
- `start.sh`：启动入口脚本
  - 读取 `source.sh`（运行时环境配置）
  - 若已存在编译产物 `./chipper`，直接运行
  - 否则执行 `go run cmd/experimental/whisper/main.go`
- `cmd/experimental/whisper/main.go`：当前发现的唯一 Go `main` 入口

## 3. 主要代码分层（`chipper/pkg/`）

### 3.1 Vector SDK 相关（底层 → 高层）

- `vector/`（底层连接与鉴权）
  - 负责建立 gRPC 连接、读取本地配置、注入 Bearer Token
  - 入口：`vector.New(...)`、`vector.NewWP(...)`、`vector.NewEP(...)`、`vector.NewWpExternal(...)`
- `vectorpb/`（协议层，protobuf 生成代码）
  - ExternalInterface 服务及消息类型、枚举常量
  - 作为 `vector/` 与 `sdk-wrapper/` 的基础依赖
- `sdk-wrapper/`（高层封装）
  - 对常用机器人能力做功能级封装（音频、相机、运动、显示、设置、TTS、天气等）
  - 维护 `Robot` 连接、事件流与路径配置
  - 可被上层业务直接调用

> 裁剪建议：仅去掉高层功能可删除 `sdk-wrapper/`；完全不使用 Vector SDK 才考虑连 `vector/` 与 `vectorpb/` 一起裁。

### 3.2 服务与协议

- `servers/chipper/`：核心服务逻辑（意图、知识图谱、连接检查等）
- `servers/jdocs/`：JDOCS 相关服务
- `servers/token/`：token 相关服务
- `wirepod/`：wire-pod 运行支持（预请求、语音、初始化、BLE/SSH、语言包等）
- `vtt/`：意图/知识图谱相关接口

### 3.3 xiaowan 扩展功能

- `xiaowan/tts/`、`tts2/`、`tts3/`、`tts4/`：多种 TTS 实现
- `xiaowan/stt/`：语音识别相关
- `xiaowan/chat/`：聊天/对话功能
- `xiaowan/input_image/`、`structured_outputs/`：图像输入与结构化输出
- `xiaowan/vector/`：与 Vector 交互的扩展能力

### 3.4 工具与基础设施

- `initwirepod/`：初始化与启动辅助
- `scripting/`：脚本支持
- `logger/`：日志封装
- `vars/`：全局变量与配置
- `mdnshandler/`：mDNS 发现
- `oskrpb/`：OSKR protobuf 类型

## 4. Web 相关（`chipper/webserver/`）

- 静态页面：`index.html`、`setup.html`、`initial.html`
- 静态资源：`css/`、`js/`、`assets/`
- 后端：`backend/config-ws/`、`backend/sdkapp/`
- SDK App 前端：`sdkapp/`（控制页、设置页、JS 逻辑）

## 5. 数据与证书

- `intent-data/zh-CN.json`：意图数据
- `session-certs/`、`epod/`：会话/证书相关
- `apiConfig.json`、`weather-map.json`：服务与天气配置

## 6. 裁剪建议（起步）

1. 先列出“必须保留的功能”，再按依赖倒推保留模块。
2. 尽量从高层模块开始裁剪（比如 `sdk-wrapper/`），避免影响底层连接与协议。
3. Web 不需要时可先移除 `chipper/webserver/` 与相关后端引用。

——

如需更详细的“功能到目录”的映射表，告诉我你要保留的功能清单，我可以补充精确的可删目录和调用链说明。
