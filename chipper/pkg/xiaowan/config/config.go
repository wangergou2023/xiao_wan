// 定义配置包
package config

// 定义主配置结构体
type Cfg struct {
	openAiAPIKey  string // OpenAI API的密钥
	openAibaseURL string // OpenAI 中转地址
}

// New函数用于创建并初始化Cfg配置实例
func New() Cfg {
	// 初始化主配置
	cfg := Cfg{
		openAiAPIKey:  "your",    // OpenAI API的密钥
		openAibaseURL: "your/v1", // 中转地址
	}

	return cfg // 返回配置实例
}

// OpenAiAPIKey方法返回OpenAI API的密钥
func (c Cfg) OpenAiAPIKey() string {
	return c.openAiAPIKey
}

func (c Cfg) SetOpenAiAPIKey(openAiAPIKey string) Cfg {
	c.openAiAPIKey = openAiAPIKey
	return c
}

func (c Cfg) OpenAibaseURL() string {
	return c.openAibaseURL
}

func (c Cfg) SetOpenAibaseURL(openAibaseURL string) Cfg {
	c.openAibaseURL = openAibaseURL
	return c
}
