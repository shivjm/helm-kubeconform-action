ARG GO_VERSION=1.16.5

ARG KUBECONFORM_VERSION=v0.4.11

FROM golang:$GO_VERSION-alpine AS builder

COPY go.mod go.sum main.go /usr/local/src/app/

RUN cd /usr/local/src/app/ && CGO_ENABLED=0 GOOS=linux go build -tags netgo -ldflags '-w' .

FROM ghcr.io/yannh/kubeconform:$KUBECONFORM_VERSION-alpine AS kubeconform

# no need to parametrize the version of Alpine Linux as it’s only used
# for curl & unzip
FROM alpine:3.14 AS downloader

ARG HELM_VERSION=v3.7.0

ARG SCHEMA_REVISION=master

RUN apk add -q --no-cache curl

# https://get.helm.sh/helm-v3.7.0-linux-amd64.tar.gz
RUN mkdir /helm && cd /helm && curl -sSL https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz | tar xzf -

FROM gcr.io/distroless/static@sha256:912bd2c2b9704ead25ba91b631e3849d940f9d533f0c15cf4fc625099ad145b1

ARG SCHEMA_REVISION=master

COPY --from=builder /usr/local/src/app/helm-kubeconform-action /helm-kubeconform-action

COPY --from=kubeconform /kubeconform /kubeconform

COPY --from=downloader /helm/linux-amd64/helm /helm

ENV KUBECONFORM=/kubeconform

ENV HELM=/helm

ENTRYPOINT ["/helm-kubeconform-action"]
