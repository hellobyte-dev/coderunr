#!/bin/bash

PREFIX=$(realpath $(dirname $0))

mkdir -p build

cd build

curl "https://www.python.org/ftp/python/3.12.0/Python-3.12.0.tgz" -o python.tar.gz
tar xzf python.tar.gz --strip-components=1
rm python.tar.gz

./configure --prefix "$PREFIX" --with-ensurepip=install
make -j$(nproc)
make install -j$(nproc)

cd ..

rm -rf build

bin/pip3 install numpy scipy pandas pycryptodome whoosh bcrypt passlib sympy xxhash base58 cryptography PyNaCl
# Trigger rebuild Sat Sep  6 11:13:45 PM CST 2025
# Restore python build Sat Sep  6 11:29:13 PM CST 2025
