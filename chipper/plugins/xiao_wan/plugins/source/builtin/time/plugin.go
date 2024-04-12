package main

import (
	"fmt"
	"time"

	config "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/config"
	plugins "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/plugins"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// 声明TimePlugin作为plugins.Plugin的实现
var Plugin plugins.Plugin = &TimePlugin{}

// TimePlugin结构体定义
type TimePlugin struct {
	cfg          config.Cfg
	openaiClient *openai.Client
}

// Init方法用于初始化插件
func (t *TimePlugin) Init(cfg config.Cfg, openaiClient *openai.Client) error {
	t.cfg = cfg
	t.openaiClient = openaiClient
	// 通常这里会有更多初始化代码
	return nil
}

// ID方法返回插件的唯一标识符
func (t TimePlugin) ID() string {
	return "time"
}

// Description方法返回插件的描述
func (t TimePlugin) Description() string {
	return "获取当前时间。"
}

// FunctionDefinition方法返回OpenAI函数定义
func (t TimePlugin) FunctionDefinition() openai.FunctionDefinition {
	return openai.FunctionDefinition{
		Name:        "time",
		Description: "返回系统的当前日期和时间。",
		Parameters: jsonschema.Definition{
			Type:       jsonschema.Object,
			Properties: map[string]jsonschema.Definition{}, // 此插件不需要参数
		},
	}
}

// Execute方法执行插件的主要功能，返回当前时间
func (t TimePlugin) Execute(jsonInput string) (string, error) {
	// 使用time包获取当前时间
	currentTime := time.Now()
	// 格式化当前时间并返回
	return fmt.Sprintf("当前时间是: %s", currentTime.Format("2006-01-02 15:04:05")), nil
}
