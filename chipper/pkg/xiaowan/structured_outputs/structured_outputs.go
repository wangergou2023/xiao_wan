package structured_outputs

import (
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// 定义用于存储 API 返回结果的结构体
type Sentence struct {
	Message  string `json:"message"`  // 消息内容
	Emoticon string `json:"emoticon"` // 表情
	Action   string `json:"action"`   // 动作（新增字段）
}

type Result struct {
	OwnName     string     `json:"own_name"`     // 自己的名字
	TargetNames []string   `json:"target_names"` // 打招呼对象的名字列表
	Sentences   []Sentence `json:"sentences"`    // 包含多句话的结构体数组
}

func GetChatCompletionResponseFormat() *openai.ChatCompletionResponseFormat {

	// 生成与 Result 结构体对应的 JSON Schema
	schema, err := jsonschema.GenerateSchemaForType(Result{})
	if err != nil {
		return nil
	}

	return &openai.ChatCompletionResponseFormat{
		Type: openai.ChatCompletionResponseFormatTypeJSONSchema, // 返回 JSON Schema 格式
		JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
			Name:   "responses", // 定义 schema 名称
			Schema: schema,      // 使用之前生成的 JSON Schema
			Strict: true,        // 严格匹配 schema
		},
	}
}
