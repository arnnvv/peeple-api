FROM golang:1.24-alpine as builder

RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o api

FROM gcr.io/distroless/static:nonroot
COPY --from=builder --chmod=0755 /app/api /api
USER nonroot:nonroot
ENTRYPOINT ["/api"]
