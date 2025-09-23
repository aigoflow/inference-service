#!/bin/bash

# GPU detection script for llama.cpp build configuration

detect_gpu() {
    echo "Detecting GPU support..."
    
    # Check for macOS Metal
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "Detected: macOS - Metal acceleration available"
        echo "metal"
        return
    fi
    
    # Check for NVIDIA CUDA
    if command -v nvidia-smi >/dev/null 2>&1; then
        echo "Detected: NVIDIA GPU - CUDA acceleration available"
        nvidia-smi --query-gpu=name --format=csv,noheader,nounits | head -1
        echo "cuda"
        return
    fi
    
    # Check for AMD ROCm
    if command -v rocm-smi >/dev/null 2>&1; then
        echo "Detected: AMD GPU - ROCm acceleration available"
        echo "rocm"
        return
    fi
    
    # Check for Vulkan support
    if command -v vulkaninfo >/dev/null 2>&1; then
        echo "Detected: Vulkan support available"
        echo "vulkan"
        return
    fi
    
    # Fallback to CPU
    echo "No GPU acceleration detected - using CPU only"
    echo "cpu"
}

# Main execution
GPU_TYPE=$(detect_gpu)
echo "Recommended build type: $GPU_TYPE"