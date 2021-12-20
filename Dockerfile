FROM golang:1.17-alpine AS builder

RUN mkdir -p /build/src

ADD . /build/src

RUN cd /build/src \
 	&& apk --no-cache add git gcc musl-dev \
	&& go build -o /build/gogrok

FROM alpine:3

COPY --from=builder /build/gogrok /usr/bin/gogrok
ADD scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT "/entrypoint.sh"