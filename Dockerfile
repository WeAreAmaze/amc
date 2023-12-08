# Build
FROM golang:1.19-alpine3.15 AS builder

# RUN apk add --no-cache gcc musl-dev linux-headers git make
RUN apk add --no-cache build-base  linux-headers git bash ca-certificates  libstdc++

WORKDIR /amc
ADD . .
ENV GO111MODULE="on"
RUN go mod tidy && go build  -o ./build/bin/amazechain ./cmd/amc


FROM alpine:3.15
#libstdc++
RUN apk add --no-cache ca-certificates curl tzdata
# copy compiled artifacts from builder
COPY --from=builder /amc/build/bin/* /usr/local/bin/

# Setup user and group
#
# from the perspective of the container, uid=1000, gid=1000 is a sensible choice, but if caller creates a .env
# (example in repo root), these defaults will get overridden when make calls docker-compose
ARG UID=1000
ARG GID=1000
RUN adduser -D -u $UID -g $GID amc


ENV AMCDATA /home/amc/data
# this 777 will be replaced by 700 at runtime (allows semi-arbitrary "--user" values)
RUN mkdir -p "$AMCDATA" && chown -R amc:amc "$AMCDATA" && chmod 777 "$AMCDATA"
VOLUME /home/amc/data

USER amc
WORKDIR /home/amc

RUN echo $UID

EXPOSE 20012 20013 20014 61015/udp 61016  6060
ENTRYPOINT ["amazechain"]
