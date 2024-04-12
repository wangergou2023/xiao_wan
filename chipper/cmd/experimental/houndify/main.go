package main

import (
	"github.com/wangergou2023/xiao_wan/chipper/pkg/initwirepod"
	stt "github.com/wangergou2023/xiao_wan/chipper/pkg/wirepod/stt/houndify"
)

func main() {
	initwirepod.StartFromProgramInit(stt.Init, stt.STT, stt.Name)
}
