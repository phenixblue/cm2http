FROM alpine:latest

LABEL org.opencontainers.image.authors="Joe Searcy <joe@twr.io"
LABEL org.opencontainers.image.source="https://github.com/phenixblue/cm2http"

RUN addgroup -g 1900 cm2http
RUN adduser -u 1900 -G cm2http --disabled-password cm2http

ENV PORT 5555
EXPOSE $PORT

COPY bin/cm2http /
RUN chown cm2http:cm2http /cm2http
USER cm2http

CMD ["/cm2http"]