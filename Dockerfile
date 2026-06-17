# hotreload Dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install hotreload
RUN go install github.com/shravansumanthanan/hot-reload-engine-go@latest

# Ensure /go/bin is in PATH (it is by default in the golang image, but let's be explicit)
ENV PATH="/go/bin:${PATH}"

# We don't copy the user project code here. 
# This Dockerfile is meant to be used as a base image for development.
# Instead, users will mount their code into the container.
WORKDIR /src
ENTRYPOINT ["hotreload"]
