# Build stage.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/doh-mcp .

# Runtime stage. ca-certificates is required: the DoH resolver is reached over
# TLS, and a scratch image has no trust store.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/doh-mcp /usr/local/bin/doh-mcp
ENTRYPOINT ["doh-mcp"]
