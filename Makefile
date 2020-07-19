build-send-email-function:
	GOOS=linux GOARCH=amd64 go build -o ./build/send-email-handler ./handlers/send-email/main.go
	zip -j ./build/send-email-handler.zip ./build/send-email-handler

build-webhook-function:
	GOOS=linux GOARCH=amd64 go build -o ./build/webhook-handler ./handlers/webhook/main.go
	zip -j ./build/webhook-handler.zip ./build/webhook-handler

build-transcribe-function:
	GOOS=linux GOARCH=amd64 go build -o ./build/invoke-transcribe-handler ./handlers/invoke-transcribe/main.go
	zip -j ./build/invoke-transcribe-handler.zip ./build/invoke-transcribe-handler

build-download-recording-function:
	GOOS=linux GOARCH=amd64 go build -o ./build/download-recording-handler ./handlers/download-recording/main.go
	zip -j ./build/download-recording-handler.zip ./build/download-recording-handler

build-google-speech-function:
	GOOS=linux GOARCH=amd64 go build -o ./build/google-speech-handler ./handlers/google-speech/main.go
	zip -j ./build/google-speech-handler.zip ./build/google-speech-handler

test:
	go test ./...
