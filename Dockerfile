FROM alpine:3.21.2

ARG VERSION
ARG REVISION
ARG BUILDDATE

WORKDIR /usr/local/bin

COPY gitlab-ci-semver-labels .

ENTRYPOINT ["gitlab-ci-semver-labels"]

LABEL \
  maintainer="Piotr Roszatycki <piotr.roszatycki@gmail.com>" \
  org.opencontainers.image.created=${BUILDDATE} \
  org.opencontainers.image.description="Download release from Gitlab project" \
  org.opencontainers.image.licenses="MIT" \
  org.opencontainers.image.revision=${REVISION} \
  org.opencontainers.image.source=https://github.com/dex4er/gitlab-ci-semver-labels \
  org.opencontainers.image.title=gitlab-ci-semver-labels \
  org.opencontainers.image.url=https://github.com/dex4er/gitlab-ci-semver-labels \
  org.opencontainers.image.vendor=dex4er \
  org.opencontainers.image.version=v${VERSION}
