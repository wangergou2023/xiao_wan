package memory

import "strings"

// UpdateProfileFromConversation 用轻量规则从用户话术里抽取稳定事实。
// 这里先做最有价值的一层：名字、主人关系、打招呼偏好。
func UpdateProfileFromConversation(esn, userText, aiText string) {
	profile := LoadProfile(esn)
	profile.ESN = esn

	userText = strings.TrimSpace(userText)
	if userText == "" {
		return
	}

	if name := extractName(userText); name != "" {
		profile.UserName = name
	}
	if strings.Contains(userText, "我是你的主人") || strings.Contains(userText, "我是你主人") {
		if profile.OwnerName == "" && profile.UserName != "" {
			profile.OwnerName = profile.UserName
		}
		addFact(&profile, "the user says they are your owner")
	}
	if ownerName := extractOwnerName(userText); ownerName != "" {
		profile.OwnerName = ownerName
	}
	if greeting := extractGreetingPreference(userText); greeting != "" {
		profile.PreferredGreeting = greeting
		addFact(&profile, greeting)
	}
	if language := extractPreferredLanguage(userText); language != "" {
		profile.PreferredLanguage = language
	}
	if nickname := extractNickname(userText); nickname != "" {
		profile.Nickname = nickname
	}
	for _, topic := range extractFavoriteTopics(userText) {
		addUnique(&profile.FavoriteTopics, topic)
	}
	for _, topic := range extractForbiddenTopics(userText) {
		addUnique(&profile.ForbiddenTopics, topic)
	}
	for _, fact := range extractStableFacts(userText) {
		addFact(&profile, fact)
	}

	// 如果 AI 已经确认记住了，就把名字和身份关系固化下来。
	if strings.Contains(aiText, "记住") && profile.OwnerName == "" && profile.UserName != "" &&
		(strings.Contains(userText, "主人") || strings.Contains(userText, "王")) {
		profile.OwnerName = profile.UserName
	}

	SaveProfile(profile)
}

func extractName(text string) string {
	for _, marker := range []string{"我叫", "我是"} {
		if idx := strings.Index(text, marker); idx >= 0 {
			name := strings.TrimSpace(text[idx+len(marker):])
			name = trimTrailingClause(name)
			if plausibleName(name) {
				return name
			}
		}
	}
	return ""
}

func extractOwnerName(text string) string {
	if idx := strings.Index(text, "主人是"); idx >= 0 {
		name := strings.TrimSpace(text[idx+len("主人是"):])
		name = trimTrailingClause(name)
		if plausibleName(name) {
			return name
		}
	}
	return ""
}

func extractGreetingPreference(text string) string {
	switch {
	case strings.Contains(text, "打招呼要用日语说主人"):
		return "greet the user in Japanese and call them master"
	case strings.Contains(text, "打招呼") && strings.Contains(text, "主人"):
		return "call the user master when greeting"
	default:
		return ""
	}
}

func extractPreferredLanguage(text string) string {
	switch {
	case strings.Contains(text, "说中文"), strings.Contains(text, "用中文"):
		return "Chinese"
	case strings.Contains(text, "说日语"), strings.Contains(text, "用日语"):
		return "Japanese"
	case strings.Contains(text, "说英语"), strings.Contains(text, "用英语"):
		return "English"
	default:
		return ""
	}
}

func extractNickname(text string) string {
	for _, marker := range []string{"叫我", "你要叫我", "以后叫我"} {
		if idx := strings.Index(text, marker); idx >= 0 {
			name := strings.TrimSpace(text[idx+len(marker):])
			name = trimTrailingClause(name)
			if plausibleName(name) {
				return name
			}
		}
	}
	return ""
}

func extractFavoriteTopics(text string) []string {
	var topics []string
	switch {
	case strings.Contains(text, "我喜欢讲笑话"), strings.Contains(text, "我喜欢笑话"):
		topics = append(topics, "jokes")
	case strings.Contains(text, "我喜欢做饭"), strings.Contains(text, "我喜欢烹饪"):
		topics = append(topics, "cooking")
	case strings.Contains(text, "我喜欢机器人"):
		topics = append(topics, "robots")
	}
	return topics
}

func extractForbiddenTopics(text string) []string {
	var topics []string
	switch {
	case strings.Contains(text, "别聊政治"), strings.Contains(text, "不要聊政治"):
		topics = append(topics, "politics")
	case strings.Contains(text, "别开黄腔"), strings.Contains(text, "不要开黄腔"):
		topics = append(topics, "sexual jokes")
	}
	return topics
}

// extractStableFacts 提取适合长期保存的稳定事实。
// 这里只抓两类：
// 1. 用户明确要求“记住/记一下”的信息
// 2. 明显稳定的个人背景信息，如居住地 / 籍贯
func extractStableFacts(text string) []string {
	var facts []string
	if fact := extractExplicitMemoryFact(text); fact != "" {
		facts = append(facts, fact)
	}
	if fact := extractHomeFact(text); fact != "" {
		facts = append(facts, fact)
	}
	return facts
}

func extractExplicitMemoryFact(text string) string {
	for _, marker := range []string{"记住", "记一下", "你要记住", "给我记住"} {
		if idx := strings.Index(text, marker); idx >= 0 {
			fact := strings.TrimSpace(text[idx+len(marker):])
			fact = normalizeMemoryFact(fact)
			if plausibleFact(fact) {
				return fact
			}
		}
	}
	return ""
}

func extractHomeFact(text string) string {
	switch {
	case strings.Contains(text, "我家住"):
		if value := extractTailValue(text, "我家住"); value != "" {
			return "用户家住" + value
		}
	case strings.Contains(text, "我住在"):
		if value := extractTailValue(text, "我住在"); value != "" {
			return "用户住在" + value
		}
	case strings.Contains(text, "我是") && strings.Contains(text, "人"):
		if value := extractRegionIdentity(text); value != "" {
			return "用户是" + value
		}
	}
	return ""
}

func extractTailValue(text, marker string) string {
	idx := strings.Index(text, marker)
	if idx < 0 {
		return ""
	}
	value := strings.TrimSpace(text[idx+len(marker):])
	value = trimTrailingClause(value)
	if !plausibleFact(value) {
		return ""
	}
	return value
}

func extractRegionIdentity(text string) string {
	idx := strings.Index(text, "我是")
	if idx < 0 {
		return ""
	}
	value := strings.TrimSpace(text[idx+len("我是"):])
	value = trimTrailingClause(value)
	if strings.HasSuffix(value, "人") && plausibleFact(value) {
		return value
	}
	return ""
}

func normalizeMemoryFact(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimLeft(text, "，,。：:;； ")
	for _, prefix := range []string{"我家住", "我住在", "我是", "我"} {
		if strings.HasPrefix(text, prefix) {
			switch prefix {
			case "我家住":
				if value := strings.TrimSpace(strings.TrimPrefix(text, prefix)); plausibleFact(value) {
					return "用户家住" + trimTrailingClause(value)
				}
			case "我住在":
				if value := strings.TrimSpace(strings.TrimPrefix(text, prefix)); plausibleFact(value) {
					return "用户住在" + trimTrailingClause(value)
				}
			case "我是":
				if value := strings.TrimSpace(strings.TrimPrefix(text, prefix)); plausibleFact(value) {
					return "用户是" + trimTrailingClause(value)
				}
			case "我":
				if value := strings.TrimSpace(strings.TrimPrefix(text, prefix)); plausibleFact(value) {
					return "用户" + trimTrailingClause(value)
				}
			}
		}
	}
	return trimTrailingClause(text)
}

func trimTrailingClause(text string) string {
	for _, sep := range []string{"，", ",", "。", "！", "?", "？", " ", "你", "我", "以后"} {
		if idx := strings.Index(text, sep); idx >= 0 {
			text = text[:idx]
			break
		}
	}
	return strings.TrimSpace(text)
}

func plausibleName(name string) bool {
	runes := []rune(strings.TrimSpace(name))
	return len(runes) >= 2 && len(runes) <= 8
}

func plausibleFact(fact string) bool {
	runes := []rune(strings.TrimSpace(fact))
	return len(runes) >= 2 && len(runes) <= 32
}

func addFact(profile *UserProfile, fact string) {
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return
	}
	for _, existing := range profile.Facts {
		if existing == fact {
			return
		}
	}
	profile.Facts = append(profile.Facts, fact)
}

func addUnique(target *[]string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	for _, existing := range *target {
		if existing == value {
			return
		}
	}
	*target = append(*target, value)
}
