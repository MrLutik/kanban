FROM debian:bookworm

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    devscripts \
    debhelper \
    dh-golang \
    golang-go \
    git \
    lintian \
    fakeroot \
    && rm -rf /var/lib/apt/lists/*

# Set up build directory
WORKDIR /build

# Copy source with debian directory
COPY . /build/kanban-cli-1.0.0/
COPY packaging/debian/debian /build/kanban-cli-1.0.0/debian/

WORKDIR /build/kanban-cli-1.0.0

# Build the package
CMD ["bash", "-c", "\
    VERSION=${VERSION:-1.0.0} && \
    sed -i \"s/kanban-cli (.*)/kanban-cli (${VERSION}-1)/\" debian/changelog && \
    dpkg-buildpackage -us -uc -b && \
    lintian ../*.deb 2>&1 || true && \
    cp ../*.deb /output/ \
"]
