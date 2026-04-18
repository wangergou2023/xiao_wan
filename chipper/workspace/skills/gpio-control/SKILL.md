---
name: GPIO 控制
description: 通过命令行控制 Linux GPIO 引脚。
---

## 何时使用
当用户需要控制 GPIO 引脚时，如：
- 控制 LED 开关
- 读取传感器状态
- 设置引脚输入/输出模式
- 配置引脚高低电平

## 使用步骤

### 1. 导出 GPIO 引脚
```bash
echo <gpio_num> > /sys/class/gpio/export
```

### 2. 设置方向（输入/输出）
```bash
echo out > /sys/class/gpio/gpio<gpio_num>/direction  # 输出
echo in > /sys/class/gpio/gpio<gpio_num>/direction   # 输入
```

### 3. 写入值（输出模式）
```bash
echo 1 > /sys/class/gpio/gpio<gpio_num>/value  # 高电平
echo 0 > /sys/class/gpio/gpio<gpio_num>/value  # 低电平
```

### 4. 读取值（输入模式）
```bash
cat /sys/class/gpio/gpio<gpio_num>/value
```

### 5. 取消导出
```bash
echo <gpio_num> > /sys/class/gpio/unexport
```

## 工具使用
使用 `system_cmd` 工具执行上述命令。
执行前先提醒用户这会直接操作系统 GPIO，通常需要 root 权限。

## 示例

**点亮 LED（GPIO 17）：**
```
system_cmd {"command":"echo 17 > /sys/class/gpio/export"}
system_cmd {"command":"echo out > /sys/class/gpio/gpio17/direction"}
system_cmd {"command":"echo 1 > /sys/class/gpio/gpio17/value"}
```

**读取按钮状态（GPIO 18）：**
```
system_cmd {"command":"echo 18 > /sys/class/gpio/export"}
system_cmd {"command":"echo in > /sys/class/gpio/gpio18/direction"}
system_cmd {"command":"cat /sys/class/gpio/gpio18/value"}
```

## 注意事项
- GPIO 编号使用 BCM 编码（不是物理引脚号）
- 需要 root 权限
- 某些系统使用 libgpiod 工具（gpioset/gpioget）
- 这是直接的系统级操作；除非用户明确要求，否则不要擅自改动硬件状态
