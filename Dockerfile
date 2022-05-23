FROM registry.access.redhat.com/ubi9/go-toolset AS builder

ARG COMPONENT=controller
COPY . /src
WORKDIR /src
RUN \
    make /tmp/tkn-pac /tmp/pipelines-as-code-${COMPONENT} LDFLAGS="-s -w" OUTPUT_DIR=/tmp

FROM registry.access.redhat.com/ubi9/ubi-minimal

ARG COMPONENT=controller
LABEL com.redhat.component=pipelines-as-code-${COMPONENT} \
    name=openshift-pipelines/pipelines-as-code-${COMPONENT} \
    maintainer=pipelines@redhat.com \
    summary="This image is to run Pipelines as Code ${COMPONENT} component"

COPY --from=builder /tmp/pipelines-as-code-${COMPONENT} /usr/bin/pipelines-as-code-${COMPONENT}
COPY --from=builder /tmp/tkn-pac /usr/bin/tkn-pac

ENV RUN_COMPONENT=$COMPONENT
CMD /usr/bin/pipelines-as-code-${RUN_COMPONENT}
