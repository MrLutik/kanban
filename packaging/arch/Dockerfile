FROM archlinux:base-devel

# Update system and install dependencies
RUN pacman -Syu --noconfirm && \
    pacman -S --noconfirm go git namcap fakeroot sudo && \
    pacman -Scc --noconfirm

# Create build user (makepkg refuses to run as root)
RUN useradd -m builder && \
    echo "builder ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers && \
    mkdir -p /build /output && \
    chown -R builder:builder /build /output

USER builder
WORKDIR /build

# Copy PKGBUILD and install file
COPY --chown=builder:builder packaging/arch/PKGBUILD /build/
COPY --chown=builder:builder packaging/arch/kanban.install /build/

# Create source tarball from the repo
COPY --chown=builder:builder . /build/kanban-src/

# Build the package
CMD ["bash", "-c", "\
    VERSION=${VERSION:-1.0.0} && \
    sed -i \"s/pkgver=.*/pkgver=${VERSION}/\" PKGBUILD && \
    cd /build/kanban-src && \
    tar czf /build/kanban-cli-${VERSION}.tar.gz --transform \"s,^,kanban-${VERSION}/,\" \
        --exclude='.git' --exclude='bin' --exclude='dist' --exclude='*.db' \
        . && \
    cd /build && \
    sed -i 's|source=(.*)|source=(\"kanban-cli-${VERSION}.tar.gz\")|' PKGBUILD && \
    makepkg -sf --noconfirm && \
    namcap *.pkg.tar.zst 2>&1 || true && \
    cp *.pkg.tar.zst /output/ \
"]
