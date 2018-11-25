FROM golang:1.11.2 as build
WORKDIR /src
ADD . /src
RUN go mod tidy && \
    CGO_ENABLED=0 go build -v

FROM alpine
COPY --from=build /src/rancher-pipeline-deploy /bin
WORKDIR /root
CMD [ "/bin/rancher-pipeline-deploy" ]
