# Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server/main.go

# Run stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .
# Bot brains: the spawner loads models/<name>.json relative to CWD and falls
# back to the scripted bot (with only a log warning) when missing — without
# this line production bots silently lose the evolved genome.
COPY --from=builder /app/models ./models
EXPOSE 8080
CMD ["./server"]
