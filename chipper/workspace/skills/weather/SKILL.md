---
name: 天气
description: 通过 weather 工具获取当前天气或简要预报。
---


## 何时使用
当用户询问天气、温度、降雨、预报等。

## 使用步骤
1. 先确认用户给了城市；没给就追问
2. 尝试将城市转换为英文（如 东京 -> Tokyo，北京 -> Beijing）
3. 用 `get_current_time` 获取当前日期，帮助组织回答
4. 当前天气：调用 `weather {"location":"Tokyo"}`
5. 简要预报：调用 `weather {"location":"Tokyo","type":"forecast","days":1}`
6. 将结果压缩成 1-3 行摘要

## 注意事项
- wttr.in 对中文地点支持不稳定，优先使用英文城市名
- 如果用户没给城市，要先追问
- 返回内容可能较长，务必摘要
- 如果天气工具报错，要直接说明当前无法在线查询天气

## 示例
用户："东京今天天气怎么样？"
→ get_current_time
→ weather {"location":"Tokyo"}
→ "东京：多云 8°C，体感 6°C，风速 10 km/h。"

用户："北京未来三天天气如何？"
→ weather {"location":"Beijing","type":"forecast","days":1}
→ "如果需要更详细的未来预报，我可以继续查，但当前先给你简要天气摘要。"
