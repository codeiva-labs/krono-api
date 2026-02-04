# Stage 1: Builder
# Use an official Golang image to build the application
FROM golang:1.22-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
# CGO_ENABLED=0 disables cgo to ensure a statically linked executable
# -o app specifies the output executable name
RUN CGO_ENABLED=0 go build -o app .

# Stage 2: Production
# Use a minimal base image for the final production container
FROM alpine:latest

# Set the working directory for the final application
WORKDIR /root/

# Copy the built executable from the builder stage
COPY --from=builder /app/app .

# Expose the port your Go application listens on (default is often 8080 or 3000)
EXPOSE 8080

# Command to run the executable
CMD ["./app"]
