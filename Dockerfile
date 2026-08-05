# Stage 1: Build the Go executable
FROM golang:1.26.3-bookworm AS builder

# Install build dependencies for CGO (sqlite3 package requires GCC)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy all files including the vendor directory
COPY . .

# Build the executable with CGO enabled for sqlite3
ENV CGO_ENABLED=1
RUN go build -tags "sqlite_fts5" -mod=vendor -o qmd ./cmd/qmd

# Stage 2: Create a minimal final image using glibc-based Debian bookworm-slim
FROM debian:bookworm-slim

# Copy the compiled executable from the build stage
COPY --from=builder /app/qmd /usr/local/bin/qmd

# Set the default entrypoint
ENTRYPOINT ["/usr/local/bin/qmd"]
