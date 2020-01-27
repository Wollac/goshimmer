# we need to use alpine to build since cgo is required
FROM golang:1.13 AS build

# Set the current Working Directory inside the container
RUN mkdir /heartbeat
WORKDIR /heartbeat

# Download dependencies
COPY go.mod . 
COPY go.sum .
RUN go mod download

# Copy everything from the current directory to the PWD(Present Working Directory) inside the container
COPY . .

# Build
WORKDIR /heartbeat/tools/heartbeat
RUN CGO_ENABLED=0 GOOS=linux go build -o /go/bin/heartbeat

FROM alpine:latest  

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the Pre-built binary file from the previous stage
COPY --from=build /go/bin/heartbeat .

ENTRYPOINT ["./heartbeat"]
