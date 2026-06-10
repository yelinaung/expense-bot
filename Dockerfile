# Build stage
# golang:1.26.3-alpine
FROM golang@sha256:68cb6d68bed024785b69195b89af7ac7a444f27791435f98647edff595aa0479 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY internal ./internal
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -o expense-bot .

# Run stage
# alpine:3
FROM alpine@sha256:33154315cf4402e697f065e6ec2156e292187e633908ccfede9c66279b6fa956

# hadolint ignore=DL3018
RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S appgroup \
    && adduser -S appuser -G appgroup

WORKDIR /app
COPY --from=builder /app/expense-bot .

USER appuser
CMD ["./expense-bot"]
