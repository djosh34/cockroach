# The patchroaddb release image. The release workflow builds it for linux/amd64
# and linux/arm64 from a context that holds patchroaddb-amd64,
# patchroaddb-arm64, patchroaddb.sh, LICENSE, NOTICE and the licenses folder.
FROM registry.access.redhat.com/ubi9/ubi-minimal
ARG TARGETARCH

# tzdata - for time zone functions; reinstalled to replace the missing
#          files in /usr/share/zoneinfo/
# hostname - used by Kubernetes manifests
# tar - used by kubectl cp
RUN microdnf update -y \
    && rpm --erase --nodeps tzdata \
    && microdnf install tzdata hostname tar gzip xz -y \
    && microdnf clean all \
    && rm -rf /var/cache/yum

# The working directory keeps its upstream name, so the data directory stays
# /cockroach/cockroach-data, as in upstream images.
RUN mkdir /cockroach /licenses /docker-entrypoint-initdb.d
COPY patchroaddb.sh /cockroach/
COPY patchroaddb-${TARGETARCH} /cockroach/patchroaddb
COPY LICENSE NOTICE licenses/ /licenses/

# Set working directory so that relative paths
# are resolved appropriately when passed as args.
WORKDIR /cockroach/

# Include the directory in the path to make it easier to invoke
# commands via Docker
ENV PATH=/cockroach:$PATH

EXPOSE 26257 8080
ENTRYPOINT ["/cockroach/patchroaddb.sh"]
