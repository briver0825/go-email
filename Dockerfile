FROM golang:1.24-alpine AS builder

ENV GOPROXY=https://mirrors.aliyun.com/goproxy/,direct

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o email-demo .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/email-demo .
COPY --from=builder /app/web ./web

EXPOSE 8080
CMD ["./email-demo"]
