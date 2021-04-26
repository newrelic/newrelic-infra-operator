FROM golang:1.16-alpine3.13 as builder

RUN apk add -U make

WORKDIR /usr/src/nri-k8s-operator

COPY go.mod .

RUN go mod download

COPY . .

RUN make

FROM alpine:3.13

COPY --from=builder /usr/src/nri-k8s-operator/nri-k8s-operator /usr/local/bin/

ENTRYPOINT ["nri-k8s-operator"]
