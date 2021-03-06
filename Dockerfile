FROM golang:1.16.2 AS builder

RUN apt-get update && apt-get install upx -y
WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
COPY main.go main.go
COPY server/ server/
COPY watcher/ watcher/

# Build
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GO111MODULE=on
ENV GOPROXY="https://goproxy.cn"
RUN go mod download && go build -o ingress-controller main.go && upx ingress-controller

FROM alpine:3.9.2
COPY --from=builder /workspace/ingress-controller .
ENTRYPOINT ["/ingress-controller"]
