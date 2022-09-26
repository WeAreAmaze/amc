# Build
FROM docker.io/library/golang:1.19-alpine3.15 AS builder


# RUN apk add --no-cache gcc musl-dev linux-headers git make
RUN apk add --no-cache build-base  linux-headers git bash ca-certificates  libstdc++

WORKDIR /amc
ADD . .

RUN go mod tidy && go build  -o ./build/bin/AmazeChain-linux-amd64 ./cmd/amc


FROM docker.io/library/alpine:3.15

RUN apk add --no-cache ca-certificates curl libstdc++ tzdata
# copy compiled artifacts from builder
COPY --from=builder /amc/build/bin/* /usr/local/bin/

EXPOSE 20012
WORKDIR /amc