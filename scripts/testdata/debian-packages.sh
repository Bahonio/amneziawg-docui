#!/bin/sh
set -eu
if [ "${AWG_DOCUI_DEBIAN_PACKAGE_SANDBOX:-}" != yes ] \
   || [ ! -e /.dockerenv ] || [ "$(id -u)" -ne 0 ]; then
    echo 'Use scripts/test-debian-packages.sh' >&2
    exit 1
fi
# shellcheck disable=SC1091
. /etc/os-release
case "$ID:$VERSION_CODENAME" in debian:bookworm|debian:trixie) ;; *) exit 1 ;; esac
test_arch=$(dpkg --print-architecture)
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl gnupg kmod \
    dkms "linux-headers-$test_arch"

# Exercise the very same repository setup included in the release installer.
sh /source/scripts/setup-debian-repositories.sh "$VERSION_CODENAME"
apt-get update
apt-get install -y --no-install-recommends amneziawg
command -v awg
command -v awg-quick

# Loading the module needs a real Debian host. Compilation and module metadata
# can be checked against the Debian kernels inside this unprivileged container.
found_kernel=no
for module_dir in /lib/modules/*; do
    [ -d "$module_dir/build" ] || continue
    kernel_version=${module_dir##*/}
    modinfo -k "$kernel_version" -F filename amneziawg
    modinfo -k "$kernel_version" -F vermagic amneziawg
    dkms status -m amneziawg -k "$kernel_version" | grep -q ': installed'
    found_kernel=yes
done
[ "$found_kernel" = yes ] || { echo 'No Debian kernel module was built.' >&2; exit 1; }
echo "PASS: official AmneziaWG packages and DKMS build on Debian $VERSION_ID/$test_arch"
