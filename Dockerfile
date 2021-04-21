FROM golang:1.16-alpine3.13 as builder

RUN apk add -U make

WORKDIR /usr/src/nri-kubernetes-operator

COPY go.mod .

RUN go mod download

COPY . .

RUN make

FROM alpine:3.13

COPY --from=builder /usr/src/nri-kubernetes-operator/nri-kubernetes-operator /usr/local/bin/

ENTRYPOINT ["nri-kubernetes-operator"]
