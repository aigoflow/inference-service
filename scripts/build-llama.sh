#!/bin/bash
set -e

# Build script for llama.cpp with various acceleration backends
# Usage: ./build-llama.sh [cpu|metal|cuda|rocm|vulkan]

BUILD_TYPE=${1:-cpu}
LLAMA_DIR="llama.cpp"
BUILD_DIR="build"
TARGET_DIR="internal/llama"
CACHE_DIR=".build_cache"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Building llama.cpp with $BUILD_TYPE acceleration..."

# Clone or update llama.cpp
if [ ! -d "$LLAMA_DIR" ]; then
    echo "Cloning llama.cpp..."
    git clone https://github.com/ggerganov/llama.cpp.git
else
    echo "Updating llama.cpp..."
    cd "$LLAMA_DIR" && git pull && cd ..
fi

# Get current commit hash for caching
cd "$LLAMA_DIR"
CURRENT_COMMIT=$(git rev-parse HEAD)
CURRENT_TAG=$(git describe --tags --exact-match 2>/dev/null || echo "")
cd ..

# Check if we have a cached build
CACHE_FILE="$CACHE_DIR/${BUILD_TYPE}_${CURRENT_COMMIT}"
if [ -f "$CACHE_FILE" ] && [ -f "$TARGET_DIR/libllama.a" ]; then
    echo "Found cached build for commit $CURRENT_COMMIT ($BUILD_TYPE)"
    if [ "$CURRENT_TAG" != "" ]; then
        echo "Tagged release: $CURRENT_TAG"
    fi
    echo "Skipping build - using cached libraries"
    exit 0
fi

echo "Building new version:"
echo "  Commit: $CURRENT_COMMIT"
if [ "$CURRENT_TAG" != "" ]; then
    echo "  Tag: $CURRENT_TAG"
fi
echo "  Type: $BUILD_TYPE"

cd "$LLAMA_DIR"

# Clean previous build
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Configure build based on type
case "$BUILD_TYPE" in
    "metal")
        echo "Configuring for Metal (macOS)..."
        cmake -B "$BUILD_DIR" \
            -DCMAKE_BUILD_TYPE=Release \
            -DGGML_METAL=ON \
            -DGGML_NATIVE=ON \
            -DBUILD_SHARED_LIBS=OFF
        ;;
    "cuda")
        echo "Configuring for CUDA..."
        cmake -B "$BUILD_DIR" \
            -DCMAKE_BUILD_TYPE=Release \
            -DGGML_CUDA=ON \
            -DGGML_NATIVE=ON \
            -DBUILD_SHARED_LIBS=OFF
        ;;
    "rocm")
        echo "Configuring for ROCm (AMD)..."
        cmake -B "$BUILD_DIR" \
            -DCMAKE_BUILD_TYPE=Release \
            -DGGML_HIPBLAS=ON \
            -DGGML_NATIVE=ON \
            -DBUILD_SHARED_LIBS=OFF
        ;;
    "vulkan")
        echo "Configuring for Vulkan..."
        cmake -B "$BUILD_DIR" \
            -DCMAKE_BUILD_TYPE=Release \
            -DGGML_VULKAN=ON \
            -DGGML_NATIVE=ON \
            -DBUILD_SHARED_LIBS=OFF
        ;;
    "cpu"|*)
        echo "Configuring for CPU only..."
        cmake -B "$BUILD_DIR" \
            -DCMAKE_BUILD_TYPE=Release \
            -DGGML_NATIVE=ON \
            -DBUILD_SHARED_LIBS=OFF
        ;;
esac

# Build
echo "Building llama.cpp..."
NCPU=$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)
cmake --build "$BUILD_DIR" --config Release -j"$NCPU"

# Create target directory
cd ..
mkdir -p "$TARGET_DIR"

# Copy libraries
echo "Copying libraries..."
cp "$LLAMA_DIR/$BUILD_DIR/src/libllama.a" "$TARGET_DIR/"
cp "$LLAMA_DIR/$BUILD_DIR/ggml/src/libggml.a" "$TARGET_DIR/"
cp "$LLAMA_DIR/$BUILD_DIR/ggml/src/libggml-base.a" "$TARGET_DIR/"
cp "$LLAMA_DIR/$BUILD_DIR/ggml/src/libggml-cpu.a" "$TARGET_DIR/"
if [ -f "$LLAMA_DIR/$BUILD_DIR/ggml/src/ggml-blas/libggml-blas.a" ]; then
    cp "$LLAMA_DIR/$BUILD_DIR/ggml/src/ggml-blas/libggml-blas.a" "$TARGET_DIR/"
fi
if [ -f "$LLAMA_DIR/$BUILD_DIR/ggml/src/ggml-metal/libggml-metal.a" ]; then
    cp "$LLAMA_DIR/$BUILD_DIR/ggml/src/ggml-metal/libggml-metal.a" "$TARGET_DIR/"
fi

# Copy headers
echo "Copying headers..."
cp -r "$LLAMA_DIR/include" "$TARGET_DIR/"
cp -r "$LLAMA_DIR/src" "$TARGET_DIR/"
cp -r "$LLAMA_DIR/ggml/include" "$TARGET_DIR/ggml_include"

# Copy Metal shader if built with Metal
if [ "$BUILD_TYPE" = "metal" ] && [ -f "$LLAMA_DIR/$BUILD_DIR/bin/ggml-metal.metal" ]; then
    echo "Copying Metal shader..."
    cp "$LLAMA_DIR/$BUILD_DIR/bin/ggml-metal.metal" "$TARGET_DIR/"
fi

# Build our custom binding
echo "Building custom binding..."
cd "$TARGET_DIR"

# Compile binding
c++ -O3 -DNDEBUG -std=c++11 -fPIC -c binding.cpp \
    -I./include \
    -I./src \
    -I./ggml_include

# Create binding library
ar rcs libbinding.a binding.o

# Create build info
echo "{\"build_type\":\"$BUILD_TYPE\",\"build_time\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"commit\":\"$CURRENT_COMMIT\",\"tag\":\"$CURRENT_TAG\",\"gpu_support\":true}" > build_info.json

# Create cache marker
mkdir -p "../$CACHE_DIR"
touch "../$CACHE_FILE"

echo "Build completed successfully!"
echo "Built with: $BUILD_TYPE acceleration"
echo "Commit: $CURRENT_COMMIT"
if [ "$CURRENT_TAG" != "" ]; then
    echo "Tag: $CURRENT_TAG"
fi
echo "Libraries: libllama.a, libggml.a, libbinding.a"
echo "Target: $TARGET_DIR"
echo "Cached: $CACHE_FILE"