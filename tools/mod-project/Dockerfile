FROM alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
ARG TARGETARCH
ARG RELEASE_VERSION
ARG SOURCE_REVISION
LABEL org.opencontainers.image.title="0xMetaLabs Fleet research fork" \
      org.opencontainers.image.source="https://github.com/inno-fusion/fleet" \
      org.opencontainers.image.version="${RELEASE_VERSION}" \
      org.opencontainers.image.revision="${SOURCE_REVISION}" \
      org.opencontainers.image.licenses="MIT AND LicenseRef-Fleet-EE"
RUN apk --no-cache add ca-certificates && apk --no-cache upgrade openssl libcrypto3 libssl3 \
    && addgroup -S fleet && adduser -S fleet -G fleet
COPY binaries/linux-${TARGETARCH}/fleet binaries/linux-${TARGETARCH}/fleetctl /usr/bin/
COPY licenses/ /usr/share/licenses/fleet/
ENV FLEET_LICENSE_LOCAL_RESEARCH=true
USER fleet
EXPOSE 8080
CMD ["fleet", "serve"]
