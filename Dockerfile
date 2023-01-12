# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:latest
#MAINTAINER maochengfei@transsion.com

COPY deploy/bin/manager /

EXPOSE 9090

ENTRYPOINT ["/manager"]