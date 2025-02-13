FROM golang:1.22.0-alpine AS backend-builder
WORKDIR /app
COPY src/go.mod src/go.sum ./
RUN go mod download
COPY src/*.go .
COPY src/public public
RUN CGO_ENABLED=0 GOOS=linux go build -o secretnoteapp .

FROM alpine:latest AS certificates
RUN apk --no-cache add ca-certificates

FROM scratch
WORKDIR /app
COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=backend-builder /app/secretnoteapp .
EXPOSE 8085
CMD ["./secretnoteapp"]