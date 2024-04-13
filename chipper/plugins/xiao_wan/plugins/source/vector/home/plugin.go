package main

import (
	"context"
	"encoding/json"
	"fmt"

	sdk_wrapper "github.com/fforchino/vector-go-sdk/pkg/sdk-wrapper"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	config "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/config"
	plugins "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/plugins"
)

// HomeControlPlugin作为plugins.Plugin的实现
var Plugin plugins.Plugin = &HomeControlPlugin{}

// HomeControlPlugin结构体定义
type HomeControlPlugin struct {
	cfg          config.Cfg
	openaiClient *openai.Client
}

// Init方法用于初始化插件
func (h *HomeControlPlugin) Init(cfg config.Cfg, openaiClient *openai.Client) error {
	h.cfg = cfg
	h.openaiClient = openaiClient
	return nil
}

// ID方法返回插件的唯一标识符
func (h HomeControlPlugin) ID() string {
	return "control_home"
}

// Description方法返回插件的描述
func (h HomeControlPlugin) Description() string {
	return "控制机器人回到家（充电站）休息和充电，以及从家出发开始工作。"
}

// FunctionDefinition方法返回OpenAI函数定义
func (h HomeControlPlugin) FunctionDefinition() openai.FunctionDefinition {
	return openai.FunctionDefinition{
		Name:        "control_home",
		Description: "根据指令控制机器人回家休息和充电或从家出发开始工作。",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"action": {
					Type: jsonschema.String,
					Enum: []string{"return", "leave"},
				},
			},
		},
	}
}

// Execute方法执行插件的主要功能，控制机器人的家庭动作
func (h HomeControlPlugin) Execute(jsonInput string) (string, error) {
	// 解析输入
	var input struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal([]byte(jsonInput), &input); err != nil {
		return "", fmt.Errorf("无法解析输入: %v", err)
	}

	ctx := context.Background()
	start := make(chan bool)
	stop := make(chan bool)
	go func() {
		_ = sdk_wrapper.Robot.BehaviorControl(ctx, start, stop)
	}()

	for {
		select {
		case <-start:
			switch input.Action {
			case "return":
				sdk_wrapper.DriveOnCharger()
				stop <- true
				return "欢迎回家！机器人正在充电。", nil
			case "leave":
				sdk_wrapper.DriveOffCharger()
				stop <- true
				return "机器人已准备好开始工作，已离开家。", nil
			default:
				stop <- true
				return "", fmt.Errorf("未知的动作指令: %s", input.Action)
			}
		}
	}
}
