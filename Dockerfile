FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o openhub ./cmd/openhub

FROM alpine:latest

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY --from=builder /build/openhub .

RUN mkdir -p /data

EXPOSE 2222 3000

CMD ["./openhub", "server"]
