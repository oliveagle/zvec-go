#!/bin/bash
# Build zvec static libraries for macOS (arm64 or x86_64)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZVEC_DIR="$SCRIPT_DIR/zvec"
LIB_DIR="$SCRIPT_DIR/lib"

# Determine architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    OS_ARCH="arm64"
elif [ "$ARCH" = "x86_64" ]; then
    OS_ARCH="x86_64"
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

echo "Building for macOS $OS_ARCH..."

# Check if submodules are initialized
if [ ! -f "$ZVEC_DIR/CMakeLists.txt" ]; then
    echo "Initializing git submodules..."
    cd "$SCRIPT_DIR"
    git submodule update --init --recursive
fi

# Create build directory
cd "$ZVEC_DIR"
mkdir -p build && cd build

# Configure CMake
echo "Configuring CMake..."
cmake -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
      -DCMAKE_SYSTEM_PROCESSOR="$OS_ARCH" \
      -DCMAKE_OSX_ARCHITECTURES="$OS_ARCH" \
      -DCMAKE_INSTALL_NAME_DIR=@rpath \
      ..

# Build
echo "Building..."
make -j$(sysctl -n hw.ncpu)

# Copy libraries
echo "Copying libraries to $LIB_DIR..."
mkdir -p "$LIB_DIR"
cp "$ZVEC_DIR/build/lib/libzvec_core.a" "$LIB_DIR/libzvec_core-macos-$OS_ARCH.a"
cp "$ZVEC_DIR/build/lib/libzvec_ailego.a" "$LIB_DIR/libzvec_ailego-macos-$OS_ARCH.a"

# Get git hash
GIT_HASH=$(cd "$ZVEC_DIR" && git rev-parse --short HEAD)
cp "$ZVEC_DIR/build/lib/libzvec_core.a" "$LIB_DIR/libzvec_core-$OS_ARCH-$GIT_HASH.a"
cp "$ZVEC_DIR/build/lib/libzvec_ailego.a" "$LIB_DIR/libzvec_ailego-$OS_ARCH-$GIT_HASH.a"

echo "Build complete!"
echo "Libraries in $LIB_DIR:"
ls -la "$LIB_DIR"
