FROM python:3.12-slim

RUN pip install --no-cache-dir \
        typing-extensions==4.12.2 \
        bottle==0.13.2 \
        proxy-tools==0.1.0 \
    && useradd --create-home --uid 10002 editor \
    && mkdir -p /opt/editor /workspace /app/state
COPY psp-linux-v0.17.4/ /opt/editor/
COPY save_pal_web.py /app/save_pal_web.py
RUN ws_file="$(grep -Rl 'const cee="127.0.0.1:5174/ws",AYe="true"' /opt/editor/ui)" \
    && test -n "$ws_file" \
    && sed -i 's#const cee="127.0.0.1:5174/ws",AYe="true"#const cee=window.location.host+"/editor/ws",AYe="false"#' "$ws_file" \
    && grep -q 'window.location.host+"/editor/ws"' "$ws_file" \
    && sed -i 's#"PUBLIC_WS_URL":"127.0.0.1:5174/ws","PUBLIC_DESKTOP_MODE":"true"#"PUBLIC_WS_URL":"","PUBLIC_DESKTOP_MODE":"false"#' /opt/editor/ui/_app/env.js \
    && rm -f /opt/editor/psp.db \
    && ln -s /opt/editor/data /app/state/data \
    && chown -R editor:editor /opt/editor /app/state

USER editor
WORKDIR /app/state
EXPOSE 16827
ENTRYPOINT ["python", "/app/save_pal_web.py"]
