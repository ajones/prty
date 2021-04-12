FROM golang:1.13-alpine as development_base
ARG GITHUB_ACCESS_TOKEN
RUN apk add build-base bash ca-certificates git openssh-client postgresql inotify-tools \
  && git config --global url."https://${GITHUB_ACCESS_TOKEN}:@github.com/".insteadOf "https://github.com/" \
  && echo "Deleting /root/.gitconfig" \
  && rm -rf /root/.gitconfig \
  && echo "Downloading dependencies" \
  && go get -v gopkg.in/urfave/cli.v2 \
  && go get -v github.com/oxequa/realize \
  && go get -u -v github.com/onsi/ginkgo/ginkgo \
  && go get -u -v github.com/onsi/gomega \
  && go get -u -v github.com/modocache/gover \
  && go get -u -v github.com/mattn/goveralls \
  && go get -u -v github.com/pressly/goose/cmd/goose \
  && echo "Finished downloading dependencies" \
  ENV GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPRIVATE=github.com/inburst

ARG GITHUB_ACCESS_TOKEN
ARG SERVICE_NAME
ENV GO111MODULE=on GOOS=linux GOARCH=amd64
WORKDIR /go/src/github.com/inburst/${SERVICE_NAME}
RUN git config --global url."https://${GITHUB_ACCESS_TOKEN}:@github.com/".insteadOf "https://github.com/"
COPY go.* ./
RUN GOPRIVATE=github.com/inburst go mod download
COPY . .
# remove integration folder from build base
# ginko refuses to ignore this folders tests
RUN rm -rf ./integration
RUN rm -f /root/.gitconfig

# Local Development
FROM development_base AS local
ARG SERVICE_NAME
WORKDIR /go/src/github.com/inburst/${SERVICE_NAME}
ENTRYPOINT ["bash", "./entrypoints/local.sh"]

# Integration Testing 
FROM development_base AS testable
ARG SERVICE_NAME
WORKDIR /go/src/github.com/inburst/${SERVICE_NAME}
# ensure integ tests are pulled in before running full suite
COPY ./integration ./integration
ENTRYPOINT ["/go/bin/ssm-env", "bash", "./entrypoints/integ.sh"]

# Deployment Test Guard
FROM development_base AS deployable_base
ARG SERVICE_NAME
WORKDIR /go/src/github.com/inburst/${SERVICE_NAME}
RUN /go/bin/ginkgo -v -r --keepGoing  .
# put actual build as the last step, so we can get faster results
# all the other go testing tools will build the source anyway
RUN go build -a -installsuffix cgo -ldflags="-w -s" -o "/go/bin/${SERVICE_NAME}"

# Master Deployment Artifact
FROM scratch AS deployable
ARG SERVICE_NAME
COPY --from=deployable_base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=deployable_base /go/src/github.com/inburst/${SERVICE_NAME} /go/src/github.com/inburst/${SERVICE_NAME}
COPY --from=deployable_base /go/bin/${SERVICE_NAME} /${SERVICE_NAME}
CMD /$SERVICE_NAME