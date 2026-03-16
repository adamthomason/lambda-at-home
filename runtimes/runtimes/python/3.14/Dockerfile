FROM python:3.14-alpine

ENV PYTHONUNBUFFERED=1

# Copy shared Python runtime code, then version-specific files on top
COPY ./common/python/ /app/
COPY ./runtimes/python/3.14/ /app/

RUN pip install --no-cache-dir -r /app/included-libraries.txt && \
    pip install --no-cache-dir -r /app/requirements.txt && \
    pip install --no-cache-dir /app/ && \
    find /usr/local/lib -name '__pycache__' -type d -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib -name 'tests' -type d -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib -name 'test' -type d -exec rm -rf {} + 2>/dev/null || true && \
    find /usr/local/lib -name '*.pyc' -delete && \
    rm -rf /root/.cache

RUN rm -f /sbin/init && cp /app/init.sh /sbin/init && chmod +x /sbin/init

ENTRYPOINT ["/sbin/init"]
