#!/usr/bin/env sh
# Regenerates the static APT repository that the gh-pages branch serves
# (issue #66). Given a gh-pages checkout, a freshly built .deb, and the
# signing key material, it copies the .deb into pool/, re-indexes dists/
# with apt-ftparchive, signs the Release file, and writes public.key —
# leaving the working tree ready for the release workflow to commit and push.
#
# Not for interactive use: called from .github/workflows/release.yml.
#
# Required env (set by the workflow):
#   APT_REPO_DIR        path to the gh-pages branch checkout
#   APT_DEB             path to the system-monitor .deb to publish
#   APT_GPG_KEY         path to the armored signing key (repo secret APT_GPG_KEY)
#   APT_GPG_PASSPHRASE  passphrase for that key (repo secret APT_GPG_PASSPHRASE)
set -eu

repo_url="https://josephheinz.github.io/system-monitor"
suite="stable"
component="main"
arch="amd64"

: "${APT_REPO_DIR:?APT_REPO_DIR is not set}"
: "${APT_DEB:?APT_DEB is not set}"
: "${APT_GPG_KEY:?APT_GPG_KEY is not set — add the APT_GPG_KEY and APT_GPG_PASSPHRASE repository secrets}"
APT_GPG_PASSPHRASE="${APT_GPG_PASSPHRASE:-}"

if [ ! -s "$APT_DEB" ]; then
  echo "error: APT_DEB is not a non-empty file: $APT_DEB" >&2
  exit 1
fi
if [ ! -s "$APT_GPG_KEY" ]; then
  echo "error: APT_GPG_KEY is not a non-empty file: $APT_GPG_KEY" >&2
  echo "       configure the APT_GPG_KEY / APT_GPG_PASSPHRASE repository secrets" >&2
  exit 1
fi

# Sign with a throwaway keyring so the runner's own keys stay untouched.
GNUPGHOME=$(mktemp -d)
trap 'rm -rf "$GNUPGHOME"' EXIT
chmod 700 "$GNUPGHOME"
export GNUPGHOME

# Publish under the standard pool layout (pool/<component>/<letter>/<name>);
# earlier versions stay indexed.
pool_dir="$APT_REPO_DIR/pool/$component/s/system-monitor"
mkdir -p "$pool_dir"
cp "$APT_DEB" "$pool_dir/"

# apt-ftparchive writes Filename fields relative to the CWD, so run it from
# the archive root and pass the pool path relative to that root.
cd "$APT_REPO_DIR"
bin_dir="dists/$suite/$component/binary-$arch"
mkdir -p "$bin_dir"
apt-ftparchive packages pool > "$bin_dir/Packages"
gzip -9 -kf "$bin_dir/Packages"
if [ ! -s "$bin_dir/Packages" ]; then
  echo "error: no packages found in pool/ — empty Packages index" >&2
  exit 1
fi

apt-ftparchive \
  -o "APT::FTPArchive::Release::Origin=$repo_url" \
  -o "APT::FTPArchive::Release::Label=system-monitor" \
  -o "APT::FTPArchive::Release::Suite=$suite" \
  -o "APT::FTPArchive::Release::Codename=$suite" \
  -o "APT::FTPArchive::Release::Architectures=$arch" \
  -o "APT::FTPArchive::Release::Components=$component" \
  release "dists/$suite" > "dists/$suite/Release"

gpg --batch --import "$APT_GPG_KEY"
gpg --batch --armor --export > public.key
gpg --batch --yes --pinentry-mode loopback \
  --passphrase "$APT_GPG_PASSPHRASE" \
  --clearsign -o "dists/$suite/InRelease" "dists/$suite/Release"
gpg --batch --yes --pinentry-mode loopback \
  --passphrase "$APT_GPG_PASSPHRASE" \
  --detach-sign --armor -o "dists/$suite/Release.gpg" "dists/$suite/Release"
