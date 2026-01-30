
# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o csy-helper-bot .

# Run stage
FROM alpine:3

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/expense-bot .

CMD ["./expense-bot"]
