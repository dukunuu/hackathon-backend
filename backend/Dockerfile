# ./backend/Dockerfile

# Stage 1: Builder
FROM golang:1.24.2-alpine AS builder

# Install git (might be needed by go install for some packages or if your go.mod has private repos)
RUN apk add --no-cache git

ENV CGO_ENABLED=0 GOOS=linux
WORKDIR /app

# Install golang-migrate CLI using 'go install'.
# This installs it to $GOPATH/bin or $GOBIN, which is typically /go/bin in the golang image.
RUN echo "Installing golang-migrate CLI in builder..." && \
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest && \
    echo "golang-migrate CLI installed to /go/bin/migrate in builder"

# Copy migration files.
# Assuming your schema files are in './db/schema' and you want them in './migrations' inside the container.
COPY ./db/schema ./migrations

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN echo "Downloading Go modules..." && \
    go mod download && \
    echo "Verifying Go modules..." && \
    go mod verify

# Copy the rest of the application source code
COPY . .

# Build the application
RUN echo "Building the Go application..." && \
    go build -ldflags="-w -s" -o /app/main ./cmd/main.go && \
    echo "Go application built successfully."

# --- Final Stage ---
FROM alpine:3.21.3

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the built application from the builder stage
COPY --from=builder /app/main .

# Copy the migrate CLI from the builder stage to the final image
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
# Copy migration files from the builder stage to the final image
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

# The default entrypoint for the application
ENTRYPOINT ["/app/main"]

