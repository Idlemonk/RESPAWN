FROM golang:1.22-alpine AS builder
WORKDIR /app

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source
COPY . .

# Build the binary — using your structure
RUN go build -o /respawn ./cmd/respawn

# Final lightweight image
FROM alpine:latest
WORKDIR /
COPY --from=builder /respawn /respawn

# Expose your application port here if applicable
EXPOSE 8080

CMD ["/respawn"]
