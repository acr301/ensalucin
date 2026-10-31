FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /asmlab ./cmd/asmlab

FROM debian:trixie-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends qemu-user llvm lld \
 && rm -rf /var/lib/apt/lists/* \
 && for t in llvm-mc llvm-objdump llvm-objcopy ld.lld; do \
      ln -sf "$(ls /usr/lib/llvm-*/bin/$t | tail -n1)" /usr/local/bin/$t; done
COPY --from=build /asmlab /usr/local/bin/asmlab
COPY examples /examples
WORKDIR /work
ENTRYPOINT ["asmlab"]
