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

RUN cd / && curl -sSL https://github.com/yannh/kubernetes-json-schema/archive/$SCHEMA_REVISION.zip | unzip - && mv /kubernetes-json-schema-$SCHEMA_REVISION /kubernetes-json-schema

# https://get.helm.sh/helm-v3.7.0-linux-amd64.tar.gz
RUN mkdir /helm && cd /helm && curl -sSL https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz | tar xzf -

FROM scratch

ARG SCHEMA_REVISION=master

COPY --from=builder /usr/local/src/app/helm-kubeconform-action /helm-kubeconform-action

COPY --from=kubeconform /kubeconform /kubeconform

COPY --from=downloader /helm/linux-amd64/helm /helm

COPY --from=downloader /kubernetes-json-schema /kubernetes-json-schema

ENV KUBECONFORM=/kubeconform

ENV HELM=/helm

ENV KUBERNETES_SCHEMA_PATH=/kubernetes-json-schema

ENTRYPOINT ["/helm-kubeconform-action"]
