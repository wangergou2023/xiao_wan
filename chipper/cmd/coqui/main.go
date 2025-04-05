package main

import (
	"github.com/wangergou2023/wire-pod/chipper/pkg/initwirepod"
	stt "github.com/wangergou2023/wire-pod/chipper/pkg/wirepod/stt/coqui"
)

func main() {
	initwirepod.StartFromProgramInit(stt.Init, stt.STT, stt.Name)
}
