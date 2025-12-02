FROM golang:1.25.4-alpine AS backend-builder
WORKDIR /app
# Copy Go module files and download dependencies
COPY src/go.mod src/go.sum ./
RUN go mod download
# Copy application source code
COPY src/*.go ./
COPY src/public public
# CGO_ENABLED=0 ensures a static binary
# -ldflags "-s -w" removes debugging symbols, reducing binary size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o secretnoteapp .

FROM scratch AS final
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
WORKDIR /app
COPY --from=backend-builder /app/secretnoteapp .

EXPOSE 8085
CMD ["./secretnoteapp"]