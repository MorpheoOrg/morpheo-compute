FROM debian:stable-slim

RUN apt-get update && \
    apt-get install -y libltdl-dev \
  && rm -rf /var/lib/apt/lists/*

ADD build/target /compute-worker
ENTRYPOINT ["/compute-worker"]
