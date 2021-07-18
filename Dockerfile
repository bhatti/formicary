FROM golang:1.16-alpine3.13 as go-builder
WORKDIR /src
ENV GO111MODULE=on
RUN apk add --no-cache git make bash
RUN apk add build-base
RUN make build

FROM alpine:3.13
ENTRYPOINT ["/formicary"]

RUN apk add --no-cache ca-certificates
RUN addgroup -S formicary-user && adduser -S -G formicary-user formicary-user

COPY --from=go-builder /src/out/bin/formicary /formicary
USER formicary-user
