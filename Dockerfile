# Build pchain in a stock Go builder container
FROM golang:1.12-alpine as builder

RUN apk add --no-cache make gcc musl-dev linux-headers

ADD . /pchain
RUN cd /pchain && make pchain

# Pull Pchain into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /pchain/bin/pchain /usr/local/bin/

EXPOSE 6969 30308 30308/udp
ENTRYPOINT ["pchain"]
