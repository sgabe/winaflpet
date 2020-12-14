FROM golang@sha256:4d8abd16b03209b30b48f69a2e10347aacf7ce65d8f9f685e8c3e20a512234d9 as builder

ARG BUILD_VER
ARG BUILD_REV
ARG BUILD_DATE

ENV BUILD_VER ${BUILD_VER}
ENV BUILD_REV ${BUILD_REV}
ENV BUILD_DATE ${BUILD_DATE}
ENV GO111MODULE=on
ENV USER=winaflpet
ENV UID=10001

LABEL org.label-schema.build-date=$BUILD_DATE \
      org.label-schema.vcs-url="https://github.com/sgabe/winaflpet.git" \
      org.label-schema.vcs-ref=$BUILD_REV \
      org.label-schema.schema-version="1.0.0-rc1"

COPY . /tmp/winaflpet/

RUN apk update && \
    apk add --no-cache git ca-certificates tzdata gnuplot libc-dev gcc && \
    update-ca-certificates && \
    adduser --disabled-password \
            --gecos "" \
            --home "/nonexistent" \
            --shell "/sbin/nologin" \
            --no-create-home \
            --uid "${UID}" "${USER}" && \
    cd /tmp/winaflpet/server && \
    go get -d -v . && \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-X main.BuildVer=$BUILD_VER -X main.BuildRev=$BUILD_REV -w -s -extldflags '-static'" -a \
        -o /tmp/winaflpet/winaflpet .

FROM alpine@sha256:a15790640a6690aa1730c38cf0a440e2aa44aaca9b0e8931a9f2b0d7cc90fd65

RUN apk update && \
    apk add --no-cache curl gnuplot

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

COPY --from=builder --chown=winaflpet:winaflpet /tmp/winaflpet/server/public /opt/winaflpet/public
COPY --from=builder /tmp/winaflpet/server/templates /opt/winaflpet/templates
COPY --from=builder /tmp/winaflpet/winaflpet /opt/winaflpet/

HEALTHCHECK --start-period=1m \
  CMD curl --silent --fail -X POST http://127.0.0.1:4141/ping || exit 1

VOLUME /data

EXPOSE 4141

WORKDIR /opt/winaflpet

USER winaflpet:winaflpet

ENTRYPOINT ["/opt/winaflpet/winaflpet"]
