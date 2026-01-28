# Build stage
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build both binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /key-bringer ./cmd/key-bringer
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /key-seeker ./cmd/key-seeker

# Runtime stage for key-bringer (Cloud Run)
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /key-bringer /key-bringer

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/key-bringer"]
