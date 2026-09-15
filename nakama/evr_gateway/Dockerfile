FROM golang:1.25-alpine
WORKDIR /build
COPY main.go .
RUN go mod init github.com/briar/evr-gateway && \
    go get github.com/gorilla/websocket && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o evr-gateway main.go

FROM alpine:latest
WORKDIR /app
COPY --from=0 /build/evr-gateway .
EXPOSE 7360
CMD ["./evr-gateway"]
