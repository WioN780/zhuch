# Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o server ./cmd/server

# Run stage
FROM alpine:3.21
RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=builder /app/server .
# Bot brains: the spawner loads models/<name>.json relative to CWD and falls
# back to the scripted bot (with only a log warning) when missing — without
# this line production bots silently lose the evolved genome.
COPY --from=builder /app/models ./models
USER app
EXPOSE 8080
CMD ["./server"]
