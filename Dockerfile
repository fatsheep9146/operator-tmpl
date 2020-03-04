FROM reg.docker.alibaba-inc.com/centos:7
#FROM reg.docker.alibaba-inc.com/busybox:v0.1
WORKDIR /app/
COPY operator /app/operator
ENTRYPOINT ["/app/operator"]
