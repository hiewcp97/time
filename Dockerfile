# Stage 1: Build the Go application
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy module definitions
COPY go.mod ./

# Copy all source files
COPY . .

# Ensure go modules are tidy and download dependencies
RUN go mod tidy

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server main.go

# Stage 2: Create small deployment image
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from build stage
COPY --from=builder /app/server .

# Copy static assets and DB bootstrap folders
COPY --from=builder /app/static ./static
COPY --from=builder /app/db ./db

EXPOSE 8080

CMD ["./server"]
