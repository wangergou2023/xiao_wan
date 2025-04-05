package main

import (
	"github.com/wangergou2023/wire-pod/chipper/pkg/initwirepod"
	stt "github.com/wangergou2023/wire-pod/chipper/pkg/wirepod/stt/houndify"
)

func main() {
	initwirepod.StartFromProgramInit(stt.Init, stt.STT, stt.Name)
}
