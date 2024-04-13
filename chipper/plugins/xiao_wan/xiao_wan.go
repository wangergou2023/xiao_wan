package xiao_wan

// 导入所需的包
import (
	"context" // 用于控制请求、超时和取消
	"fmt"     // 用于格式化输出

	"regexp"  // 用于正则表达式
	"strconv" // 用于字符串和其他类型的转换

	// 用于控制屏幕输出
	openai "github.com/sashabaranov/go-openai" // OpenAI GPT的Go客户端
	// 聊天界面
	config "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/config"   // 配置
	plugins "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/plugins" // 插件系统
)

// 定义助手结构体，包括配置、OpenAI客户端、函数定义和聊天界面
type Xiao_wan struct {
	cfg                 config.Cfg
	Client              *openai.Client
	functionDefinitions []openai.FunctionDefinition
}

// 定义系统提示信息，指导如何使用AI助手
var SystemPrompt = `
你是一个名为小丸的多才多艺的AI助手。你启动时的首要任务是“激活”你的记忆，即立即回忆并熟悉与用户及其偏好最相关的数据。这有助于个性化并增强用户互动。

利用可用的插件套件提供最佳解决方案。你可以：
- 对于简单任务，单独使用插件。
- 对于复杂任务，串联多个插件。

例如：如果被告知“明天，我需要做X”，结合日期时间插件确定日期和记忆插件来保存任务。

存储和检索信息是你角色的关键。凭借你的能力，确保用户相关数据的保存和检索。优先捕获重要和次要的细节，增强你的记忆深度。保存任何细节时，总是包含其上下文。例如，如果用户提到他们喜欢咖啡，记得当时表达的情景或情感。这样的上下文在后续交互中是无价的。

在接收到用户输入之前，你必须做的第一件事就是使用记忆插件来激活你的记忆。这将使你能够为用户提供最好的可能体验。

`

// 定义全局变量conversation，用于存储对话历史
var conversation []openai.ChatCompletionMessage

// appendMessage函数用于向对话中添加消息
func appendMessage(role string, message string, name string) {
	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    role,
		Content: message,
		Name:    name,
	})
}

// resetConversation函数用于清空对话历史
func resetConversation() {
	conversation = []openai.ChatCompletionMessage{}
}

// restartConversation函数用于重置并重新开始对话
func (xiao_wan Xiao_wan) restartConversation() {
	resetConversation() // 重置对话

	appendMessage(openai.ChatMessageRoleSystem, SystemPrompt, "") // 添加系统提示到对话

	response, err := xiao_wan.sendMessage() // 发送系统提示到OpenAI并获取回复

	if err != nil {
		fmt.Printf("Error sending system prompt to OpenAI: %v\n", err)
	}

	appendMessage(openai.ChatMessageRoleAssistant, response, "") // 添加助手回复到对话
}

// Message函数用于处理用户消息
func (xiao_wan Xiao_wan) Message(message string) (string, error) {

	appendMessage(openai.ChatMessageRoleUser, message, "") // 添加用户消息到对话

	response, err := xiao_wan.sendMessage() // 发送消息到OpenAI并获取回复

	if err != nil {
		return "", err
	}

	appendMessage(openai.ChatMessageRoleAssistant, response, "") // 添加助手回复到对话
	fmt.Printf("xiao wan:%s\r\n", response)

	return response, nil
}

// sendMessage函数用于向OpenAI发送请求并获取回复
func (xiao_wan Xiao_wan) sendMessage() (string, error) {
	resp, err := xiao_wan.sendRequestToOpenAI() // 发送请求到OpenAI

	if err != nil {
		return "", err
	}

	if resp.Choices[0].FinishReason == openai.FinishReasonFunctionCall {
		responseContent, err := xiao_wan.handleFunctionCall(resp) // 处理函数调用
		if err != nil {
			return "", err
		}
		return responseContent, nil
	}

	return resp.Choices[0].Message.Content, nil
}

// handleFunctionCall函数用于处理OpenAI回复中的函数调用
func (xiao_wan Xiao_wan) handleFunctionCall(resp *openai.ChatCompletionResponse) (string, error) {

	funcName := resp.Choices[0].Message.FunctionCall.Name // 获取函数名称
	fmt.Println("获取函数名称", funcName)
	ok := plugins.IsPluginLoaded(funcName) // 检查是否加载了相应插件
	fmt.Println("检查是否加载了相应插件", ok)
	if !ok {
		return "", fmt.Errorf("no plugin loaded with name %v", funcName)
	}

	jsonResponse, err := plugins.CallPlugin(resp.Choices[0].Message.FunctionCall.Name, resp.Choices[0].Message.FunctionCall.Arguments) // 调用插件

	if err != nil {
		return "", err
	}
	appendMessage(openai.ChatMessageRoleFunction, resp.Choices[0].Message.Content, funcName)
	appendMessage(openai.ChatMessageRoleFunction, jsonResponse, "functionName")

	resp, err = xiao_wan.sendRequestToOpenAI() // 发送请求到OpenAI
	if err != nil {
		return "", err
	}

	if resp.Choices[0].FinishReason == openai.FinishReasonFunctionCall {
		return xiao_wan.handleFunctionCall(resp) // 递归处理函数调用
	}

	return resp.Choices[0].Message.Content, nil
}

// sendRequestToOpenAI函数用于向OpenAI发送请求
func (xiao_wan Xiao_wan) sendRequestToOpenAI() (*openai.ChatCompletionResponse, error) {
	resp, err := xiao_wan.Client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:        openai.GPT4Turbo,
			Messages:     conversation,
			Functions:    xiao_wan.functionDefinitions,
			FunctionCall: "auto",
		},
	)

	if err != nil {
		xiao_wan.openaiError(err) // 处理OpenAI错误
		fmt.Println("Error: ", err)
	}
	return &resp, err
}

// Start函数用于启动助手
func Start(cfg config.Cfg, openaiClient *openai.Client) Xiao_wan {
	if err := plugins.LoadPlugins(cfg, openaiClient); err != nil {
		fmt.Printf("Error loading plugins: %v", err)
	}
	fmt.Println("Plugins loaded successfully")
	xiao_wan := Xiao_wan{
		cfg:                 cfg,
		Client:              openaiClient,
		functionDefinitions: plugins.GenerateOpenAIFunctionsDefinition(),
	}

	xiao_wan.restartConversation()

	fmt.Println("xiao wan is ready!")
	return xiao_wan

}

// OpenAIError结构体用于封装OpenAI错误
type OpenAIError struct {
	StatusCode int
}

// parseOpenAIError函数用于解析OpenAI错误
func parseOpenAIError(err error) *OpenAIError {
	var statusCode int

	reStatusCode := regexp.MustCompile(`status code: (\d+)`)

	if match := reStatusCode.FindStringSubmatch(err.Error()); match != nil {
		statusCode, _ = strconv.Atoi(match[1]) // 将字符串转换为整数
	}

	return &OpenAIError{
		StatusCode: statusCode,
	}
}

// openaiError函数用于处理OpenAI错误
func (xiao_wan Xiao_wan) openaiError(err error) {
	parsedError := parseOpenAIError(err)

	switch parsedError.StatusCode {
	case 401:
		fmt.Println("Invalid OpenAI API key. Please enter a valid key.")
		fmt.Println("You can find your API key at https://beta.openai.com/account/api-keys")
		fmt.Println("You can also set your API key as an environment variable named OPENAI_API_KEY")
	default:
		// Handle other errors
		fmt.Println("Unknown error: ", parsedError.StatusCode)
	}
}
