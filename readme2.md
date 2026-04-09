命令
sudo ./setup.sh
sudo ./chipper/start.sh

语音转文本
https://github.com/ahmetoner/whisper-asr-webservice

curl -X 'POST' \
  'http://localhost:9000/asr?encode=true&task=transcribe&word_timestamps=false&output=txt' \
  -H 'accept: application/json' \
  -H 'Content-Type: multipart/form-data' \
  -F 'audio_file=@alloy_speech.mp3;type=audio/mpeg'
