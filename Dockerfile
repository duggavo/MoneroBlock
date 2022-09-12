FROM golang:alpine as build
LABEL author="seth@sethforprivacy.com" \
      maintainer="seth@sethforprivacy.com"

# Set MoneroBlock branch or tag to build
ARG MONERO_BLOCK_BRANCH=main

# Set the proper HEAD commit hash for the given branch/tag in MONERO_BLOCK_BRANCH
ARG MONERO_BLOCK_COMMIT_HASH=de08fa10be7706c66f2374baa4d2bafc61fbe49e

# Upgrade base image
RUN set -ex && apk --update --no-cache upgrade

# Install git dependency
RUN set -ex && apk add --update --no-cache git

# Switch to MoneroBlock source directory
WORKDIR /moneroblock

# Build MoneroBlock from given branch and verify HEAD commit
RUN set -ex && git clone --branch ${MONERO_BLOCK_BRANCH} \
    https://github.com/duggavo/MoneroBlock . \
    && test `git rev-parse HEAD` = ${MONERO_BLOCK_COMMIT_HASH} || exit 1 \
    && go get ./... \
    && go build -ldflags="-s -w" ./

# Begin final image build
# Select Alpine 3.x for the base image
FROM alpine:3

# Upgrade base image
RUN set -ex && apk --update --no-cache upgrade

# Install curl for health check
RUN set -ex && apk add --update --no-cache curl

# Add user and setup directories for moneroblock
ARG MONERO_BLOCK_USER="moneroblock"
RUN set -ex && adduser -Ds /bin/bash ${MONERO_BLOCK_USER}

USER "${MONERO_BLOCK_USER}:${MOMONERO_BLOCK_USERNERO_USER}"

# Switch to home directory and install newly built moneroblock binary
WORKDIR /home/${MONERO_BLOCK_USER}
COPY --chown=${MONERO_BLOCK_USER}:${MONERO_BLOCK_USER} --from=build /moneroblock/moneroblock /usr/local/bin/moneroblock

# Expose web port
EXPOSE 31312

# Add HEALTHCHECK against get_info endpoint
HEALTHCHECK --interval=30s --timeout=5s CMD curl --fail http://localhost:31312 || exit 1

# Start moneroblock with default daemon flag, to be overridden by end-users
ENTRYPOINT ["moneroblock", "--bind", "0.0.0.0:31312"]
CMD ["--daemon", "node.sethforprivacy.com:18089"]
