sudo STT=whisper ./setup.sh

Which speech-to-text service would you like to use?
1: Coqui (local, no usage collection, less accurate, a little slower)
2: Picovoice Leopard (local, usage collected, accurate, account signup required)
3: VOSK (local, accurate, multilanguage, fast, recommended)
4: Whisper.cpp (local, accurate, multilanguage, a little slower, recommended for more powerful hardware)
5: openai online Whisper 

Enter a number (5): 4

Which Whisper model would you like to use?
Options: tiny, base, small, medium, large-v3, large-v3-q5_0
(tiny is recommended)

Enter preferred model: small

whisper.cpp版本
cd xiao_wan/whisper.cpp
git checkout v1.5.5

流程
xiao_wan/chipper/cmd/experimental/whisper.cpp/main.go
StartFromProgramInit
...
chipper/pkg/servers/chipper/intent_graph.go
StreamingIntentGraph
xiao_wan/chipper/pkg/wirepod/preqs/intent_graph.go
ProcessIntentGraph
xiao_wan/chipper/pkg/wirepod/ttr/kgsim.go
StreamingKGSim
CreateAIReq
xiao_wan/chipper/pkg/wirepod/ttr/kgsim_cmds.go
CreatePrompt

语音转文本
https://github.com/ahmetoner/whisper-asr-webservice

curl -X 'POST' \
  'http://localhost:9000/asr?encode=true&task=transcribe&word_timestamps=false&output=txt' \
  -H 'accept: application/json' \
  -H 'Content-Type: multipart/form-data' \
  -F 'audio_file=@alloy_speech.mp3;type=audio/mpeg'