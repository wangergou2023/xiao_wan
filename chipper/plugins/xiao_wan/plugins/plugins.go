package plugins

// 导入必要的包
import (
	"encoding/json" // 用于JSON处理
	"fmt"           // 用于格式化输出
	"os"            // 提供操作系统函数，用于文件路径操作等
	"path/filepath" // 用于文件路径操作
	"plugin"        // 支持从共享库动态加载代码
	"runtime"

	config "github.com/wangergou2023/xiao_wan/chipper/plugins/xiao_wan/config" // 配置包
	"github.com/sashabaranov/go-openai"                                        // OpenAI GPT库
)

// 已加载插件的映射，键为插件ID，值为插件实例
var loadedPlugins = make(map[string]Plugin)

// Plugin接口定义了所有插件必须实现的方法
type Plugin interface {
	Init(cfg config.Cfg, openaiClient *openai.Client) error // 初始化插件
	ID() string                                             // 获取插件ID
	Description() string                                    // 获取插件描述
	FunctionDefinition() openai.FunctionDefinition          // 获取函数定义，用于OpenAI
	Execute(string) (string, error)                         // 执行插件逻辑
}

// PluginResponse结构体用于封装插件执行的响应
type PluginResponse struct {
	Error  string `json:"error,omitempty"`  // 错误信息，如果有的话
	Result string `json:"result,omitempty"` // 成功执行的结果
}

// LoadPlugins函数加载指定目录下的所有插件
func LoadPlugins(cfg config.Cfg, openaiClient *openai.Client) error {
	loadedPlugins = make(map[string]Plugin) // 重新初始化插件映射

	// 获取当前函数的执行文件路径
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Println("Error: Cannot get current file path")
		return fmt.Errorf("cannot get current file path")
	}

	// 打印当前文件所在目录
	fmt.Println("Current file path:", filename)
	fmt.Println("Current directory:", filepath.Dir(filename))

	// 从"compiled"目录读取插件文件
	files, err := os.ReadDir(filepath.Dir(filename) + "/compiled")
	if err != nil {
		return err
	}

	// 遍历文件，加载.so文件作为插件
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".so" {
			fmt.Println("Loading plugin: ", file.Name())
			err := loadSinglePlugin(filepath.Dir(filename)+"/compiled/"+file.Name(), cfg, openaiClient)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// loadSinglePlugin函数加载单个插件
func loadSinglePlugin(path string, cfg config.Cfg, openaiClient *openai.Client) error {
	plugin, err := plugin.Open(path) // 打开插件文件
	if err != nil {
		return err
	}

	symbol, err := plugin.Lookup("Plugin") // 查找插件中的"Plugin"符号
	if err != nil {
		return err
	}

	// 类型断言确认找到的符号类型正确
	p, ok := symbol.(*Plugin)
	if !ok {
		return fmt.Errorf("unexpected type from module symbol: %s", path)
	}
	err = (*p).Init(cfg, openaiClient) // 初始化插件
	if err != nil {
		return err
	}
	loadedPlugins[(*p).ID()] = *p // 将插件加入映射
	return nil
}

// CallPlugin函数通过ID查找插件并执行
func CallPlugin(id string, jsonInput string) (string, error) {
	response := PluginResponse{}

	plugin, exists := GetPluginByID(id) // 查找插件
	if !exists {
		response.Error = fmt.Sprintf("plugin with ID %s not found", id)
		jsonResponse, err := json.Marshal(response)
		return string(jsonResponse), err
	}

	// 执行插件
	result, err := plugin.Execute(jsonInput)
	if err != nil {
		response.Error = err.Error()
	} else {
		response.Result = result
	}

	// 将执行结果转换为JSON
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("error marshaling response to JSON: %v", err)
	}

	return string(jsonResponse), nil
}

// IsPluginLoaded函数检查指定ID的插件是否已加载
func IsPluginLoaded(id string) bool {
	_, exists := loadedPlugins[id]
	return exists
}

// GetPluginByID函数通过ID获取插件
func GetPluginByID(id string) (Plugin, bool) {
	p, exists := loadedPlugins[id]
	return p, exists
}

// GetAllPlugins函数返回所有已加载的插件
func GetAllPlugins() map[string]Plugin {
	return loadedPlugins
}

// GenerateOpenAIFunctionsDefinition函数生成所有插件的OpenAI函数定义
func GenerateOpenAIFunctionsDefinition() []openai.FunctionDefinition {
	var definitions []openai.FunctionDefinition

	// 遍历已加载的插件，收集它们的函数定义
	for _, plugin := range loadedPlugins {
		def := plugin.FunctionDefinition()
		definitions = append(definitions, def)
	}

	return definitions
}
