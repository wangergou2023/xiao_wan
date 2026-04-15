//go:build !inbuiltble
// +build !inbuiltble

package botsetup

import "github.com/wangergou2023/xiao_wan/chipper/pkg/logger"

func RegisterBLEAPI() {
	logger.Println("BLE API is unregistered")
}
