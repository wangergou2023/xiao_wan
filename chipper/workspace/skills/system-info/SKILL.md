---
name: 系统信息
description: 获取内核、内存、负载、磁盘等系统信息（通过 system_cmd 工具）。
---


## 何时使用
当用户要求查看 dmesg、内核版本、内存占用、系统负载、磁盘使用情况等。

## 使用步骤
1. 直接调用 `system_cmd` 执行系统命令。
2. 调用方式：system_cmd {"command":"<shell 命令>"}（兼容 cmd 字段）
3. 输出时做简短摘要，避免整屏刷屏。

## 示例
用户：“看一下内核信息”
→ system_cmd {"command":"uname -a"}
→ “内核版本：... 架构：...”
