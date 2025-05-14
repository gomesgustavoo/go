########################################
# Stage 1 – imagem base com GStreamer run-time
########################################
FROM nvidia/cuda:12.8.0-runtime-ubuntu22.04 AS gst-base

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        gstreamer1.0-tools \
        gstreamer1.0-plugins-base gstreamer1.0-plugins-good \
        gstreamer1.0-plugins-bad gstreamer1.0-plugins-ugly \
        gstreamer1.0-libav gstreamer1.0-gl \
        librtmp1 ca-certificates && \
    rm -rf /var/lib/apt/lists/*

########################################
# Stage 2 – build do binário Go com cgo
########################################
FROM gst-base AS builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        golang build-essential pkg-config git \
        libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev && \
    rm -rf /var/lib/apt/lists/*

ENV CGO_ENABLED=1 GO111MODULE=on
WORKDIR /src
COPY ingest/*.go .          # ajuste se usar go.mod no nível superior
RUN go mod init ingest 2>/dev/null || true
RUN go build -o ingest .

########################################
# Stage 3 – imagem final enxuta
########################################
FROM gst-base

WORKDIR /app
COPY --from=builder /src/ingest /app/ingest
ENTRYPOINT ["./ingest"]

