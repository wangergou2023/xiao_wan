---
name: 技能创建器
description: 为 MimiClaw 创建新技能。
---


## 何时使用
当用户要求创建新技能、教会新的能力或扩展功能时。

## 如何创建技能
1. 选择简短、清晰的名称（小写，可用连字符）
2. SKILL.md 须包含固定的 YAML front matter（必须有 name 和 description）：
   - `---`
   - `name: <技能名>`
   - `description: <一句话描述>`
   - `---`
3. front matter 之后按以下结构编写：
   - `# 标题` —— 清晰的名称
   - 简短描述段落
   - `## 何时使用` —— 触发条件
   - `## 使用步骤` —— 操作说明
   - `## 示例` —— 具体示例（可选但推荐）
4. 使用 `write_file` 保存到 `workspace/skills/<name>/SKILL.md`
5. 下一次对话开始后技能会自动生效

## 最佳实践
- 技能要简洁，避免过长（上下文有限）
- 重点写“做什么”，而不是“怎么做”
- 明确指出需要调用的工具
- 通过提问测试新技能是否生效

## 示例
创建 "translate" 技能：
write_file {"path":"workspace/skills/translate/SKILL.md","content":"---\nname: 翻译\ndescription: 在语言之间翻译文本。\n---\n\n# 翻译\n\n在语言之间翻译文本。\n\n## 何时使用\n当用户要求翻译文本时。\n\n## 使用步骤\n1. 识别源语言与目标语言\n2. 直接翻译\n"}
