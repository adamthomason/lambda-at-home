FROM python:3.13.5-slim-bookworm

ENV PYTHONUNBUFFERED=1

RUN apt-get update && \
    apt-get install -y --no-install-recommends busybox && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY ./runtimes/python/3.13/ /app/

RUN pip install --no-cache-dir -r /app/included-libraries.txt && \
    pip install --no-cache-dir -r /app/requirements.txt && \
    pip install --no-cache-dir /app/ && \
    find /usr/local/lib/python3.13 -type d -name '__pycache__' -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib/python3.13 -type d -name 'tests' -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib/python3.13 -type d -name 'test' -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib/python3.13 -name '*.pyc' -delete && \
    rm -rf /root/.cache

RUN cat /app/init.sh > /sbin/init && chmod +x /sbin/init

ENTRYPOINT ["/sbin/init"]
