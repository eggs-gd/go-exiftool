#!/bin/bash

set -eo pipefail
shopt -s extglob

cd $(dirname "${BASH_SOURCE[0]}")
cd ..

# Single source of truth for the ExifTool version (also checked by dist.yml)
version=$(tr -d '[:space:]' < EXIFTOOL_VERSION)
# exiftool.org no longer hosts tarballs. SourceForge keeps the latest few
# releases; CPAN keeps production releases forever.
exiftool_urls=(
  "https://sourceforge.net/projects/exiftool/files/Image-ExifTool-${version}.tar.gz/download"
  "https://cpan.metacpan.org/authors/id/E/EX/EXIFTOOL/Image-ExifTool-${version}.tar.gz"
)

# Setup
rm -rf tmp/
mkdir -p tmp/

# Download Exiftool
for url in "${exiftool_urls[@]}"; do
  curl -fL# "$url" --output tmp/exiftool.tar.gz && break
done
[ -s tmp/exiftool.tar.gz ] || { echo "ExifTool ${version} not found at any source" >&2; exit 1; }
tar xzf tmp/exiftool.tar.gz -C tmp/ && rm tmp/exiftool.tar.gz
mv tmp/* tmp/exiftool

# Cleanup and test
pushd tmp/exiftool
rm -rf !(exiftool|lib|t|README)
find lib -name '*.pod' -delete
prove -l lib -b t 
rm -rf t
./exiftool -ver -v
popd

# Move to destination
rm -rf ${1:-dist}
mv tmp/exiftool ${1:-dist}

# Cleanup
rm -rf tmp/
