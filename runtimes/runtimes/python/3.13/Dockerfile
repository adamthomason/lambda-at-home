FROM python:3.13.5-slim-bookworm

ENV PYTHONUNBUFFERED=1

RUN apt update && \
    apt install -y busybox

COPY ./runtimes/python/3.13/ /app/

RUN pip install -r /app/included-libraries.txt && \
    pip install -r /app/requirements.txt && \
    pip install /app/

RUN cat /app/init.sh > /sbin/init && chmod +x /sbin/init

ENTRYPOINT ["/sbin/init"]
