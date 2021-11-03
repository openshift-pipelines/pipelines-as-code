FROM registry.access.redhat.com/ubi8/go-toolset:latest	 AS builder

COPY . /src
WORKDIR /src
RUN go build -mod=vendor -ldflags="-w -s -X github.com/openshift-pipelines/pipelines-as-code/pkg/params/version.Version=`git rev-parse HEAD|grep -qw main && date +'%Y-%m-%d' || git rev-parse --tags`" \
    -v -o /tmp/pipelines-as-code ./cmd/pipelines-as-code 

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.4

LABEL com.redhat.component=pipelines-as-code \ 
    name=openshift-pipelines/pipelines-as-code \ 
    maintainer=pipelines@redhat.com \ 
    summary="This image is to run Pipelines as Code task"

COPY --from=builder /tmp/pipelines-as-code /usr/bin/pipelines-as-code
RUN ln /usr/bin/pipelines-as-code /usr/bin/tkn-pac
CMD ["/usr/bin/pipelines-as-code"]
