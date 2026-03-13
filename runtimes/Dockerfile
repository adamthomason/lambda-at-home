FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y e2fsprogs sudo && \
    apt-get clean

WORKDIR /workspace

COPY generate-ext4.sh /usr/local/bin/generate-ext4.sh

RUN chmod +x /usr/local/bin/generate-ext4.sh
RUN echo "running"

ENTRYPOINT ["/usr/local/bin/generate-ext4.sh"]
