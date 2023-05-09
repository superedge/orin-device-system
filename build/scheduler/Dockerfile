FROM golang:1.17 as builder

ARG TARGETOS
ARG TARGETARCH
ARG GO_LDFLAGS

# config
WORKDIR /go/src/orin-device-system
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -v ${GO_LDFLAGS} -o /go/bin/orin-scheduler cmd/scheduler/scheduler.go
# runtime image
FROM debian:bullseye-slim

COPY --from=builder /go/bin/orin-scheduler /usr/bin/orin-scheduler
