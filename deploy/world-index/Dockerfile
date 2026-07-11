FROM python:3.12-slim

RUN pip install --no-cache-dir typing-extensions==4.12.2 \
    && useradd --create-home --uid 10001 indexer \
    && mkdir -p /opt/save-pal /app/state \
    && chown indexer:indexer /app/state
COPY psp-linux-v0.17.4/ /opt/save-pal/
COPY save_pal_index.py /app/save_pal_index.py

USER indexer
WORKDIR /app
EXPOSE 16826
ENTRYPOINT ["python", "/app/save_pal_index.py"]
