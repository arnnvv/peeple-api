FROM golang:1.23.6-alpine as builder

RUN apk add --no-cache git

# Set the working directory inside the container.
WORKDIR /app

# Cache dependencies by copying go.mod and go.sum first.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code.
COPY . .

# Build the binary statically with optimizations.
# CGO is disabled to ensure the binary is fully static.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o api .

#############################
# Final Stage
#############################
# Distroless
FROM gcr.io/distroless/static:nonroot

# Define build-time arguments.
ARG DATABASE_URL
ARG JWT_SECRET

# Set the environment variables using the build-time arguments.
# These can be overridden at runtime using docker run -e or in your orchestration.
ENV DATABASE_URL=${DATABASE_URL}
ENV JWT_SECRET=${JWT_SECRET}

# Copy the statically compiled binary from the builder stage.
COPY --from=builder /app/api /api

# Expose the port that your API listens on.
EXPOSE 8080

# Use a non-root user for security (distroless image defaults to nonroot).
USER nonroot:nonroot

# Run the binary.
ENTRYPOINT ["/api"]
