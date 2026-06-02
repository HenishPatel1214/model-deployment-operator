FROM golang:1.26 AS builder
WORKDIR /workspace
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager ./cmd/manager
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o benchmark ./cmd/benchmark

FROM gcr.io/distroless/static:nonroot AS manager
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532
ENTRYPOINT ["/manager"]

FROM gcr.io/distroless/static:nonroot AS benchmark
WORKDIR /
COPY --from=builder /workspace/benchmark .
USER 65532:65532
ENTRYPOINT ["/benchmark"]
