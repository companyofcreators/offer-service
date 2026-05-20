FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/offer-service ./cmd/api/main.go

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=UTC

WORKDIR /app

COPY --from=builder /app/bin/offer-service .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8084

ENTRYPOINT ["./offer-service"]
