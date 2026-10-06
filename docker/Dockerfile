# iRODS MCP Server Dockerfile

##############################################
# Build irods-mcp-server for the requested target platform
##############################################
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
# VERSION and GIT_COMMIT are optional. When not given, the Makefile falls back
# to git, which is unavailable in the build context, so VERSION becomes "dev".
ARG VERSION
ARG GIT_COMMIT

RUN apk add make git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOARCH)} make build

##############################################
# irods-mcp-server image
##############################################
FROM alpine:3.20.1
LABEL maintainer="Illyoung Choi <iychoi@email.arizona.edu>"
LABEL description="iRODS MCP Server"

WORKDIR /app

COPY --from=builder /app/bin/irods-mcp-server ./
COPY config.yaml ./

EXPOSE 8080

ENTRYPOINT ["./irods-mcp-server"]
