FROM rust:1.93-bookworm AS indexer

ARG PSP_VERSION=v1.2.0
ARG PALS_JSON_SHA256=a36d35ca1f5c6f4383182bd967f1defb808b472dcc63f441ea91c0b6a0353de6

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git \
    && git config --global http.version HTTP/1.1 \
    && rm -rf /var/lib/apt/lists/*

ENV CARGO_NET_GIT_FETCH_WITH_CLI=true

WORKDIR /build
COPY world-index-rust/Cargo.toml ./Cargo.toml
COPY world-index-rust/Cargo.lock ./Cargo.lock
RUN mkdir src \
    && printf 'fn main() {}\n' > src/main.rs \
    && cargo build --release --locked \
    && rm src/main.rs
COPY world-index-rust/src ./src
RUN touch src/main.rs && cargo build --release --locked
RUN mkdir -p /build/data \
    && curl --fail --location --retry 4 \
      "https://raw.githubusercontent.com/oMaN-Rod/palworld-save-pal/${PSP_VERSION}/data/json/pals.json" \
      --output /build/data/pals.json \
    && echo "${PALS_JSON_SHA256}  /build/data/pals.json" | sha256sum --check -

FROM python:3.12-slim

RUN useradd --create-home --uid 10001 indexer \
    && mkdir -p /opt/save-pal-data/json /app/state \
    && chown indexer:indexer /app/state

COPY --from=indexer /build/target/release/palworld-world-indexer /usr/local/bin/palworld-world-indexer
COPY --from=indexer /build/data/pals.json /opt/save-pal-data/json/pals.json
COPY world-index/save_pal_index.py /app/save_pal_index.py

USER indexer
WORKDIR /app
EXPOSE 16826
ENTRYPOINT ["python", "/app/save_pal_index.py"]
