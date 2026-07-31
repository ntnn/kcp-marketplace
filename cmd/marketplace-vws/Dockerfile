# The builder stays on the native platform and cross-compiles, so a multi-arch
# build never runs the toolchain under emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum go.mod go.sum ./
COPY apis/go.mod apis/go.sum ./apis/
RUN go mod download
COPY apis/ ./apis/
COPY pkg/ ./pkg/
COPY cmd/ ./cmd/
ARG VERSION=dev
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -ldflags="-w -s" -o /out/marketplace-vws ./cmd/marketplace-vws

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/marketplace-vws /usr/local/bin/marketplace-vws
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/marketplace-vws"]
