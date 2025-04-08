FROM golang:1.24.2-alpine3.21 as builder

RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build -o api

FROM gcr.io/distroless/static:nonroot
COPY --from=builder --chmod=0755 /app/api /api
USER nonroot:nonroot
ENTRYPOINT ["/api"]
