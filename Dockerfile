FROM registry.access.redhat.com/ubi8/go-toolset:latest	 AS builder


COPY . /src
WORKDIR /src
RUN make /tmp/tkn-pac /tmp/pipelines-as-code-controller LDFLAGS="-s -w" OUTPUT_DIR=/tmp

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.5

LABEL com.redhat.component=pipelines-as-code \
    name=openshift-pipelines/pipelines-as-code \
    maintainer=pipelines@redhat.com \
    summary="This image is to run Pipelines as Code task"

COPY --from=builder /tmp/pipelines-as-code-controller /usr/bin/pipelines-as-code-controller
COPY --from=builder /tmp/tkn-pac /usr/bin/tkn-pac
CMD ["/usr/bin/pipelines-as-code"]
