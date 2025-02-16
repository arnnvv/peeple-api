FROM golang:1.23.6-alpine as builder

RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o api .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/api /api
ENTRYPOINT ["/api"]
