# Usa uma imagem the GoCV pre-compilada
FROM ghcr.io/hybridgroup/opencv:4.10.0 AS builder

ENV CGO_ENABLED=1
ENV GO111MODULE=on

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Compilar
COPY . .
RUN go build -o qr-scanner main.go
RUN go build -o baseline ./cmd/baseline/main.go
RUN go build -o fuzz-runner ./cmd/fuzz/main.go

# Minimal runner
FROM ghcr.io/hybridgroup/opencv:4.10.0
WORKDIR /app

COPY --from=builder /app/qr-scanner .
COPY --from=builder /app/baseline .
COPY --from=builder /app/fuzz-runner .
COPY --from=builder /app/models ./models

ENTRYPOINT ["./qr-scanner"]
