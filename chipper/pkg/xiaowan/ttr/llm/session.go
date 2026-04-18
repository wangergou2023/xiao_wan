package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
)

const maxRememberedMessages = 16

var rememberedChatsMu sync.Mutex

// GetChat 根据机器人序列号读取记忆中的聊天上下文。
func GetChat(esn string) vars.RememberedChat {
	rememberedChatsMu.Lock()
	defer rememberedChatsMu.Unlock()

	esn = normalizeChatESN(esn)
	if esn == "" {
		return vars.RememberedChat{}
	}

	for _, chat := range vars.RememberedChats {
		if chat.ESN == esn {
			return chat
		}
	}

	chat := loadChatFromDisk(esn)
	if chat.ESN == "" {
		chat.ESN = esn
	}
	vars.RememberedChats = append(vars.RememberedChats, chat)
	return chat
}

// PlaceChat 将最新聊天上下文写回全局缓存。
func PlaceChat(chat vars.RememberedChat) {
	rememberedChatsMu.Lock()
	defer rememberedChatsMu.Unlock()

	chat.ESN = normalizeChatESN(chat.ESN)
	if chat.ESN == "" {
		return
	}
	for i, achat := range vars.RememberedChats {
		if achat.ESN == chat.ESN {
			vars.RememberedChats[i] = chat
			saveChatToDisk(chat)
			return
		}
	}
	vars.RememberedChats = append(vars.RememberedChats, chat)
	saveChatToDisk(chat)
}

// Remember 只保留最近 16 条会话消息，避免上下文无限增长。
func Remember(user, ai openai.ChatCompletionMessage, esn string) {
	RememberMessages([]openai.ChatCompletionMessage{user, ai}, esn)
}

// RememberMessages 允许把 assistant tool call 和 tool 结果一并写入会话历史。
func RememberMessages(messages []openai.ChatCompletionMessage, esn string) {
	if len(messages) == 0 {
		return
	}
	currentChat := GetChat(esn)
	currentChat.ESN = esn
	currentChat.Chats = append(currentChat.Chats, messages...)
	currentChat = trimRememberedChat(currentChat)
	PlaceChat(currentChat)
}

func trimRememberedChat(chat vars.RememberedChat) vars.RememberedChat {
	if len(chat.Chats) <= maxRememberedMessages {
		return chat
	}
	chat.Chats = append([]openai.ChatCompletionMessage(nil), chat.Chats[len(chat.Chats)-maxRememberedMessages:]...)
	return chat
}

func chatHistoryDir() string {
	return filepath.Join(filepath.Dir(vars.ApiConfigPath), "chat_histories")
}

func chatHistoryPath(esn string) string {
	safe := normalizeChatESN(esn)
	safe = strings.ReplaceAll(safe, "/", "_")
	return filepath.Join(chatHistoryDir(), safe+".json")
}

func loadChatFromDisk(esn string) vars.RememberedChat {
	data, err := os.ReadFile(chatHistoryPath(esn))
	if err != nil {
		return vars.RememberedChat{ESN: esn}
	}

	var chat vars.RememberedChat
	if err := json.Unmarshal(data, &chat); err != nil {
		return vars.RememberedChat{ESN: esn}
	}
	chat.ESN = normalizeChatESN(chat.ESN)
	if chat.ESN == "" {
		chat.ESN = normalizeChatESN(esn)
	}
	return trimRememberedChat(chat)
}

func saveChatToDisk(chat vars.RememberedChat) {
	if chat.ESN == "" {
		return
	}
	if err := os.MkdirAll(chatHistoryDir(), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(chat, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(chatHistoryPath(chat.ESN), data, 0o644)
}

func normalizeChatESN(esn string) string {
	return strings.TrimSpace(strings.ToLower(esn))
}
