FROM golang:1.17.2-alpine as go-builder
WORKDIR /src
ENV GO111MODULE=on
RUN apk add --no-cache git make bash
RUN apk add build-base
RUN make build
#RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o formicary ./main.go

FROM alpine:3.13
ENTRYPOINT ["/formicary"]

RUN apk add --no-cache ca-certificates
RUN addgroup -S formicary-user && adduser -S -G formicary-user formicary-user

COPY --from=go-builder /src/out/bin/formicary /formicary
USER formicary-user
