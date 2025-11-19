FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY server.go .
RUN go build -o test-server server.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/test-server .
EXPOSE 8080
CMD ["./test-server"]
