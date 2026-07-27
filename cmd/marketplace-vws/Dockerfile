FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum go.mod go.sum ./
COPY apis/go.mod apis/go.sum ./apis/
RUN go mod download
COPY apis/ ./apis/
COPY pkg/ ./pkg/
COPY cmd/ ./cmd/
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /out/marketplace-vws ./cmd/marketplace-vws

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/marketplace-vws /usr/local/bin/marketplace-vws
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/marketplace-vws"]
