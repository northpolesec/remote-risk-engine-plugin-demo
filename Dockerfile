# Build stage
FROM golang:1.24-alpine AS builder

# Install ca-certificates for HTTPS support
RUN apk add --no-cache ca-certificates git

# Set working directory
WORKDIR /app

# Copy source code and go.mod
COPY . .

# Download dependencies
RUN go mod download

# Build the server binary with specific flags
RUN CGO_ENABLED=0 go build -o server ./cmd/server.go

# Final stage using distroless
FROM gcr.io/distroless/static-debian12

# Copy the binary from builder
COPY --from=builder /app/server /app

# Expose port
EXPOSE 8080

# Run the server
ENTRYPOINT ["/app"]
