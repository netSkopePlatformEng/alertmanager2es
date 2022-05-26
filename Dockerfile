FROM golang:1.18 as build

WORKDIR /go/src/github.com/webdevops/alertmanager2es

# Get deps (cached)
COPY ./go.mod /go/src/github.com/webdevops/alertmanager2es
COPY ./go.sum /go/src/github.com/webdevops/alertmanager2es
COPY ./Makefile /go/src/github.com/webdevops/alertmanager2es
RUN make dependencies

# Compile
COPY ./ /go/src/github.com/webdevops/alertmanager2es
RUN make test
RUN make lint
RUN make build
RUN ./alertmanager2es --help

#############################################
# FINAL IMAGE
#############################################
FROM artifactory-gcp.netskope.io/pe-docker/ns-ubuntu-2004-fips:latest
ENV LOG_JSON=1
COPY --from=build /go/src/github.com/webdevops/alertmanager2es/alertmanager2es /
USER 1000
ENTRYPOINT ["/alertmanager2es"]
