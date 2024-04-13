package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sdk_wrapper "github.com/fforchino/vector-go-sdk/pkg/sdk-wrapper"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	config "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/config"
	plugins "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/plugins"
)

// ArmControlPlugin作为plugins.Plugin的实现
var Plugin plugins.Plugin = &ArmControlPlugin{}

// ArmControlPlugin结构体定义
type ArmControlPlugin struct {
	cfg          config.Cfg
	openaiClient *openai.Client
}

// Init方法用于初始化插件
func (a *ArmControlPlugin) Init(cfg config.Cfg, openaiClient *openai.Client) error {
	a.cfg = cfg
	a.openaiClient = openaiClient
	return nil
}

// ID方法返回插件的唯一标识符
func (a ArmControlPlugin) ID() string {
	return "control_arm"
}

// Description方法返回插件的描述
func (a ArmControlPlugin) Description() string {
	return "控制机器人手臂抬起和放下。"
}

// FunctionDefinition方法返回OpenAI函数定义
func (a ArmControlPlugin) FunctionDefinition() openai.FunctionDefinition {
	return openai.FunctionDefinition{
		Name:        "control_arm",
		Description: "根据指令控制机器人手臂抬起或放下。",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"action": {
					Type: jsonschema.String,
					Enum: []string{"raise", "lower"},
				},
			},
		},
	}
}

// Execute方法执行插件的主要功能，控制手臂动作
func (a ArmControlPlugin) Execute(jsonInput string) (string, error) {
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
	downTimer := time.NewTimer(time.Second * 5) // 设置定时器，例如5秒后自动放下
	downTimer.Stop()                            // 先停止定时器，以备后面根据实际情况启动

	go func() {
		_ = sdk_wrapper.Robot.BehaviorControl(ctx, start, stop)
	}()

	for {
		select {
		case <-start:
			switch input.Action {
			case "raise":
				sdk_wrapper.MoveLift(2.0)
				time.Sleep(time.Second * 1)
				sdk_wrapper.MoveLift(0)
				fmt.Println("正在举起")
				downTimer.Reset(time.Second * 5) // 重新启动定时器
				go func() {
					<-downTimer.C
					// 定时器时间到，自动执行放下操作
					sdk_wrapper.MoveLift(-2.0)
					time.Sleep(time.Second * 1)
					sdk_wrapper.MoveLift(0)
					fmt.Println("自动放下")
					stop <- true
				}()
				return "手臂抬起动作执行完毕。", nil
			case "lower":
				downTimer.Stop() // 收到放下指令，停止定时器
				sdk_wrapper.MoveLift(-2.0)
				time.Sleep(time.Second * 1)
				sdk_wrapper.MoveLift(0)
				fmt.Println("正在放下")
				stop <- true
				return "手臂放下动作执行完毕。", nil
			default:
				return "", fmt.Errorf("未知的动作指令: %s", input.Action)
			}
		}
	}
}
