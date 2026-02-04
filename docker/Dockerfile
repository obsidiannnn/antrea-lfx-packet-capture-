FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y tcpdump ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /

COPY agent /agent

ENTRYPOINT ["/agent"]

