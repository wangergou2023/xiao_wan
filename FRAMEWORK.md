# 代码框架说明（当前目录结构）

本文档基于当前仓库目录整理，方便裁剪与定位模块。

## 1. 项目根目录

- `FRAMEWORK.md`：本说明
- `LICENSE`：许可证
- `setup.sh` / `update.sh`：安装与更新脚本
- `readme2.md`：语音处理链路说明
- `chipper/`：核心 Go 服务与资源

## 2. 核心模块（`chipper/`）

- `go.mod` / `go.sum`：Go 模块与依赖
- `start.sh`：启动入口（支持 `-p/--web-port` 指定端口）
- `apiConfig.json` / `weather-map.json`：服务与天气配置
- `source.sh`：运行时环境变量（由 setup 生成）
- `session-certs/`、`epod/`：会话/证书相关
- `webserver/`：Web UI 与后端

入口代码：
- `cmd/experimental/whisper/main.go`

## 3. 主要代码分层（`chipper/pkg/`）

### 3.1 Vector SDK 分层

- `vector/`：底层连接与鉴权
- `vectorpb/`：protobuf/gRPC 类型定义
- `sdk-wrapper/`：高层 SDK 封装（音频、相机、运动、显示、设置、TTS 等）

### 3.2 服务与协议

- `servers/chipper/`：核心 gRPC 服务入口（当前保留 IntentGraph 流入口与连接检查）
- `servers/jdocs/`：JDOCS 服务
- `servers/token/`：token 服务
- `vtt/`：语音流请求/响应结构（仅保留 IntentGraph）

### 3.3 wire-pod 运行支持

- `initwirepod/`：启动与服务装配
- `wirepod/preqs/`：语音请求处理（STT → LLM → SDK Wrapper）
- `wirepod/speechrequest/`：语音流封装与解码
- `wirepod/setup/`：BLE/SSH/证书等初始化
- `wirepod/localization/`：模型下载与语言设置

### 3.4 xiaowan 业务扩展

- `xiaowan/stt/`：语音转文本
- `xiaowan/chat/`：LLM 请求
- `xiaowan/tts*`：多种 TTS 实现
- `xiaowan/vector/`：拍照/播报/动作整合流程
- `xiaowan/structured_outputs/`：结构化输出解析
- `xiaowan/input_image/`：图像输入
- `xiaowan/config/`：自定义配置

### 3.5 基础设施

- `logger/`：日志
- `vars/`：全局配置与运行时变量
- `scripting/`：脚本支持
- `mdnshandler/`：mDNS 发现

## 4. Web 相关（`chipper/webserver/`）

- 静态页面：`index.html`、`setup.html`、`initial.html`
- 静态资源：`css/`、`js/`、`assets/`
- 后端：`backend/config-ws/`、`backend/sdkapp/`
- SDK App 前端：`sdkapp/`

## 5. 当前语音处理主链路

```
cmd/experimental/whisper/main.go
  -> initwirepod.StartFromProgramInit
  -> servers/chipper/intent_graph.go (StreamingIntentGraph)
  -> wirepod/preqs/intent_graph.go (STT -> LLM -> SDK Wrapper)
  -> xiaowan/vector/vector.go (StreamingKGSim)
```
