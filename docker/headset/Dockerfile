FROM golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
COPY third_party/discordgo third_party/discordgo
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -ldflags="-s -w" -o faultline-headset ./cmd/faultline-headset

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends alsa-utils ca-certificates python3 python3-pip \
    && rm -rf /var/lib/apt/lists/* \
    && pip3 install --break-system-packages --no-cache-dir openwakeword onnxruntime \
    && python3 -c "from openwakeword.utils import download_models; download_models(['alexa'])"
COPY --from=builder /workspace/faultline-headset /usr/local/bin/faultline-headset
COPY cmd/faultline-headset/wake.py /usr/local/bin/headset-wake.py
# Host ALSA devices are root:audio (gid 29). Stay unprivileged otherwise.
USER 1000:29
ENTRYPOINT ["/usr/local/bin/faultline-headset"]
