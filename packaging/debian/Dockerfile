FROM debian:bookworm

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    devscripts \
    debhelper \
    dh-golang \
    git \
    lintian \
    fakeroot \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.23 from official tarball (Debian Bookworm only has 1.19)
RUN wget -q https://go.dev/dl/go1.23.4.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz && \
    rm go1.23.4.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

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
    dpkg-buildpackage -us -uc -b -d && \
    lintian ../*.deb 2>&1 || true && \
    cp ../*.deb /output/ \
"]
