.PHONY: build

build-send-email-function:
	GOOS=linux GOARCH=amd64 go build -o ./build/send-email-handler ./handlers/send-email/handler.go
	zip -j ./build/send-email-handler.zip ./build/send-email-handler

build-webhook-function:
	GOOS=linux GOARCH=amd64 go build -o ./build/webhook-handler ./handlers/webhook/handler.go
	zip -j ./build/webhook-handler.zip ./build/webhook-handler

build-transcribe-function:
	GOOS=linux GOARCH=amd64 go build -o ./build/invoke-transcribe-handler ./handlers/invoke-transcribe/handler.go
	zip -j ./build/invoke-transcribe-handler.zip ./build/invoke-transcribe-handler

build: build-send-email-function build-webhook-function build-transcribe-function
