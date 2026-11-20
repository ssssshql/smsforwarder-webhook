FROM golang:1.22-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /out/sms-code-webhook .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -H -u 1000 app

WORKDIR /app
COPY --from=builder /out/sms-code-webhook /app/sms-code-webhook

RUN mkdir -p /data && chown app:app /data

USER app
ENV DATA_DIR=/data \
    PORT=8080 \
    TZ=Asia/Shanghai

EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["/app/sms-code-webhook"]
