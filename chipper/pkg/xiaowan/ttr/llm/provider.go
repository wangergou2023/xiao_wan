package llm

import (
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
)

const (
	bigModelChatEndpoint = "https://open.bigmodel.cn/api/paas/v4"
	defaultBigModelModel = "glm-5.1"
)

func getKnowledgeAPIKey() string {
	if vars.APIConfig.Knowledge.Provider == "bigmodel" && strings.TrimSpace(vars.APIConfig.Knowledge.Key) == "" {
		if strings.TrimSpace(vars.APIConfig.BigModel.Key) != "" {
			return strings.TrimSpace(vars.APIConfig.BigModel.Key)
		}
		return strings.TrimSpace(os.Getenv("BIGMODEL_API_TOKEN"))
	}
	return strings.TrimSpace(vars.APIConfig.Knowledge.Key)
}

// getKnowledgeModel 根据 provider 返回当前对话请求使用的模型名。
func getKnowledgeModel(gpt3tryagain bool) string {
	if gpt3tryagain {
		return openai.GPT3Dot5Turbo
	}

	switch vars.APIConfig.Knowledge.Provider {
	case "openai":
		return openai.GPT4oMini
	case "bigmodel":
		if strings.TrimSpace(vars.APIConfig.Knowledge.Model) == "" {
			if strings.TrimSpace(vars.APIConfig.BigModel.LLMModel) != "" {
				return strings.TrimSpace(vars.APIConfig.BigModel.LLMModel)
			}
			return defaultBigModelModel
		}
		return strings.TrimSpace(vars.APIConfig.Knowledge.Model)
	default:
		return strings.TrimSpace(vars.APIConfig.Knowledge.Model)
	}
}

// newKnowledgeClient 按当前 provider 构建统一的聊天客户端。
func newKnowledgeClient() *openai.Client {
	key := getKnowledgeAPIKey()

	switch vars.APIConfig.Knowledge.Provider {
	case "custom":
		conf := openai.DefaultConfig(key)
		conf.BaseURL = vars.APIConfig.Knowledge.Endpoint
		return openai.NewClientWithConfig(conf)
	case "bigmodel":
		conf := openai.DefaultConfig(key)
		conf.BaseURL = bigModelChatEndpoint
		return openai.NewClientWithConfig(conf)
	default:
		return openai.NewClient(key)
	}
}

func logKnowledgeModel(model string) {
	if strings.TrimSpace(model) == "" {
		logger.Println("Using empty model value")
		return
	}
	logger.Println("Using " + model)
}
