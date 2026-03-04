FROM golang:1.25-alpine AS builder
ARG REL_VER=dev
ARG TARGETARCH
WORKDIR /

# Install git and certificates
RUN apk --no-cache add tzdata ca-certificates git
COPY go.* ./
RUN go mod download
COPY ./cmd ./cmd/
COPY ./internal ./internal/
RUN --mount=type=cache,target=/root/.cache/go-build env GOOS=linux GOARCH=${TARGETARCH} go build -v -tags=goexperiment.jsonv2 -ldflags "-s -w -X main.Version=${REL_VER}" -o out/did-resolver ./cmd/did-resolver

FROM scratch
WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/did-resolver /app/did-resolver

EXPOSE 8080
ENTRYPOINT ["/app/did-resolver"]
