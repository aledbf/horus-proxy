#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

export DEBIAN_FRONTEND=noninteractive

export BUILD_PATH=/tmp/build

get_src()
{
  hash="$1"
  url="$2"
  f=$(basename "$url")

  echo "Downloading $url"

  curl -fsSL "$url" -o "$f"
  echo "$hash  $f" | sha256sum -c - || exit 10
  tar xzf "$f"
  rm -rf "$f"
}

mkdir --verbose -p "$BUILD_PATH"
cd "$BUILD_PATH"

apk add -U \
  curl \
  make \
  libc-dev \
  gcc

# download, verify and extract the source files
get_src 42f0384f80b6a9b4f42f91ee688baf69165d0573347e6ea84ebed95e928211d7 \
        "https://github.com/openresty/lua-resty-lrucache/archive/v0.09.tar.gz"

get_src 517db9add320250b770f2daac83a49e38e6131611f2daa5ff05c69d5705e9746 \
        "https://github.com/openresty/lua-resty-lock/archive/v0.08rc1.tar.gz"

get_src 3917d506e2d692088f7b4035c589cc32634de4ea66e40fc51259fbae43c9258d \
        "https://github.com/hamishforbes/lua-resty-iputils/archive/v0.3.0.tar.gz"

get_src 095615fe94e64615c4a27f4f4475b91c047cf8d10bc2dbde8d5ba6aa625fc5ab \
        "https://github.com/openresty/lua-resty-string/archive/v0.11.tar.gz"

get_src 89cedd6466801bfef20499689ebb34ecf17a2e60a34cd06e13c0204ea1775588 \
        "https://github.com/openresty/lua-resty-balancer/archive/v0.02rc5.tar.gz"


cd "$BUILD_PATH/lua-resty-lrucache-0.09"
make
make install

cd "$BUILD_PATH/lua-resty-lock-0.08rc1"
make
make install

cd "$BUILD_PATH/lua-resty-iputils-0.3.0"
make
make install

cd "$BUILD_PATH/lua-resty-string-0.11"
make
make install

cd "$BUILD_PATH/lua-resty-balancer-0.02rc5"
make all
make install

apk del \
  make \
  libc-dev \
  gcc

rm -rf /var/cache/apk/*

cd /
rm -rf $BUILD_PATH
