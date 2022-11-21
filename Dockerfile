FROM registry.access.redhat.com/ubi9/go-toolset@sha256:9e60bacd07dc4b5f3f559167e6e9a1acd8235d043f0ffc591a2ecc3805446c74 AS builder

ARG BINARY_NAME=pipelines-as-code-controller
COPY . /src
WORKDIR /src
RUN \
    make /tmp/${BINARY_NAME} LDFLAGS="-s -w" OUTPUT_DIR=/tmp

FROM registry.access.redhat.com/ubi9/ubi-minimal@sha256:ecebade89b064d33e6e1405e4ec6e9b904e7c573a52b52d0f38026bb8d1db1f8

ARG BINARY_NAME=pipelines-as-code-controller
LABEL com.redhat.component=${BINARY_NAME} \
    name=openshift-pipelines/${BINARY_NAME} \
    maintainer=pipelines@redhat.com \
    summary="This image is to run Pipelines as Code ${BINARY_NAME} component"

COPY --from=builder /tmp/${BINARY_NAME} /usr/bin/${BINARY_NAME}

USER 1001
ENV RUN_BINARY_NAME=$BINARY_NAME
CMD /usr/bin/${RUN_BINARY_NAME}
