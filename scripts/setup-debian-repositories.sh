#!/bin/sh
set -eu

# The official AmneziaWG Debian instructions use the Ubuntu focal PPA.
# Scope its verified signing key to that repository; Debian 13 has no apt-key.
debian_suite=${1:-}
case "$debian_suite" in
    bookworm|trixie) ;;
    *) echo 'Expected Debian bookworm or trixie.' >&2; exit 2 ;;
esac
[ "$(id -u)" -eq 0 ] || { echo 'Repository setup must run as root.' >&2; exit 1; }

for destination in /etc/apt/keyrings /etc/apt/sources.list.d \
    /etc/apt/keyrings/awg-docui-amnezia.asc \
    /etc/apt/sources.list.d/awg-docui-amnezia.sources \
    /etc/apt/sources.list.d/awg-docui-debian-src.sources; do
    [ ! -L "$destination" ] || { echo "Refusing repository symlink: $destination" >&2; exit 1; }
done

repository_tmp=$(mktemp -d /tmp/awg-docui-repository.XXXXXX)
trap 'rm -rf "$repository_tmp"' EXIT
trap 'exit 1' HUP INT TERM
amnezia_fingerprint=75C9DD72C799870E310542E24166F2C257290828
curl --proto '=https' --tlsv1.2 -fsSL \
    "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x$amnezia_fingerprint" \
    -o "$repository_tmp/amnezia.asc"
key_fingerprint=$(gpg --homedir "$repository_tmp" --batch --with-colons \
    --show-keys "$repository_tmp/amnezia.asc" \
    | awk -F: '$1 == "pub" { keys++ } $1 == "fpr" && !fingerprint { fingerprint=$10 }
        END { if (keys == 1) print fingerprint }')
[ "$key_fingerprint" = "$amnezia_fingerprint" ] || {
    echo 'The Amnezia repository signing key fingerprint did not match.' >&2
    exit 1
}

install -d -m 0755 /etc/apt/keyrings /etc/apt/sources.list.d
install -m 0644 "$repository_tmp/amnezia.asc" /etc/apt/keyrings/awg-docui-amnezia.asc
cat > /etc/apt/sources.list.d/awg-docui-amnezia.sources <<'EOF'
Types: deb deb-src
URIs: https://ppa.launchpadcontent.net/amnezia/ppa/ubuntu
Suites: focal
Components: main
Signed-By: /etc/apt/keyrings/awg-docui-amnezia.asc
EOF

# DKMS may fetch the full source tree of the distribution kernel. Add only
# source entries, leaving the host's binary repositories and priorities intact.
# Debian 13 uses .pgp keyrings; using the older .gpg name for the same archive
# conflicts with Signed-By in its existing binary entries.
debian_archive_key=/usr/share/keyrings/debian-archive-keyring.gpg
if [ -f /usr/share/keyrings/debian-archive-keyring.pgp ]; then
    debian_archive_key=/usr/share/keyrings/debian-archive-keyring.pgp
fi
# Existing installations may retain the old spelling after a keyring upgrade.
if [ -r /etc/apt/sources.list.d/debian.sources ]; then
    configured_archive_key=$(awk '/^Signed-By: \/usr\/share\/keyrings\/debian-archive-keyring\.(gpg|pgp)[[:space:]]*$/ { print $2; exit }' \
        /etc/apt/sources.list.d/debian.sources)
    if [ -n "$configured_archive_key" ]; then
        debian_archive_key=$configured_archive_key
    fi
fi
cat > /etc/apt/sources.list.d/awg-docui-debian-src.sources <<EOF
Types: deb-src
URIs: https://deb.debian.org/debian
Suites: $debian_suite $debian_suite-updates
Components: main
Signed-By: $debian_archive_key

Types: deb-src
URIs: https://deb.debian.org/debian-security
Suites: $debian_suite-security
Components: main
Signed-By: $debian_archive_key
EOF
