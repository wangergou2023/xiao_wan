package main

import (
	"context"
	"fmt"

	sdk_wrapper "github.com/fforchino/vector-go-sdk/pkg/sdk-wrapper"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	config "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/config"
	plugins "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/plugins"
)

// CameraPlugin作为plugins.Plugin的实现
var Plugin plugins.Plugin = &CameraPlugin{}

// CameraPlugin结构体定义
type CameraPlugin struct {
	cfg          config.Cfg
	openaiClient *openai.Client
}

// Init方法用于初始化插件
func (c *CameraPlugin) Init(cfg config.Cfg, openaiClient *openai.Client) error {
	c.cfg = cfg
	c.openaiClient = openaiClient
	return nil
}

// ID方法返回插件的唯一标识符
func (c CameraPlugin) ID() string {
	return "take_photo"
}

// Description方法返回插件的描述
func (c CameraPlugin) Description() string {
	return "控制机器人眼睛拍照，并返回图片文件的名称。"
}

// FunctionDefinition方法返回OpenAI函数定义
func (c CameraPlugin) FunctionDefinition() openai.FunctionDefinition {
	return openai.FunctionDefinition{
		Name:        "take_photo",
		Description: "控制机器人的摄像头拍摄一张照片，保存到文件系统，并返回图片文件的名称。",
		Parameters: jsonschema.Definition{
			Type:       jsonschema.Object,
			Properties: map[string]jsonschema.Definition{}, // 此插件不需要参数
		},
	}
}

// Execute方法执行插件的主要功能，控制摄像头拍照并返回文件名称
func (c CameraPlugin) Execute(jsonInput string) (string, error) {

	// 执行控制指令
	ctx := context.Background()
	start := make(chan bool)
	stop := make(chan bool)
	go func() {
		_ = sdk_wrapper.Robot.BehaviorControl(ctx, start, stop)
	}()

	for {
		select {
		case <-start:
			sdk_wrapper.SetLocale("en-US")
			sdk_wrapper.SayText("are you ok ?")
			sdk_wrapper.SaveHiResCameraPicture("camera.jpg")
			fmt.Println("正在拍照")
			stop <- true
			// 返回文件名称
			return fmt.Sprintf("拍照成功，图片名称: %s", "camera.jpg"), nil
		}
	}
}
