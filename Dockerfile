FROM registry.access.redhat.com/ubi8/go-toolset:1.15.7	 AS builder

COPY . /src
WORKDIR /src
RUN go build -mod=vendor -v -o /tmp/pipelines-as-code ./cmd/pipelines-as-code

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.4

LABEL com.redhat.component=pipelines-as-code \ 
    name=openshift-pipelines/pipelines-as-code \ 
    maintainer=pipelines@redhat.com \ 
    summary="This image is to run Pipeline as Code task"

LABEL version=0.1.0

COPY --from=builder /tmp/pipelines-as-code /usr/bin/pipelines-as-code
CMD ["/usr/bin/pipelines-as-code"]
