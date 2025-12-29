#!/usr/bin/env python3
"""
Hardware-aware benchmark recommendation tool.
Automatically recommends optimal test configurations based on detected hardware.
"""

import os
import platform
import subprocess
import sys
from pathlib import Path
from typing import Dict, Optional


def get_cpu_info() -> Dict[str, any]:
    """Detect CPU information including model, cores, and threads."""
    cpu_info = {
        "model": "Unknown",
        "physical_cores": 1,
        "logical_cores": 1,
        "threads": 1,
    }

    try:
        # Try to get CPU count
        import multiprocessing

        cpu_info["logical_cores"] = multiprocessing.cpu_count()
        cpu_info["threads"] = cpu_info["logical_cores"]

        # Try to get physical cores (works on Linux and macOS)
        if hasattr(os, "sched_getaffinity"):
            cpu_info["threads"] = len(os.sched_getaffinity(0))
        
        # Get CPU model name
        if platform.system() == "Linux":
            try:
                with open("/proc/cpuinfo", "r") as f:
                    for line in f:
                        if "model name" in line:
                            cpu_info["model"] = line.split(":")[1].strip()
                            break
            except Exception:
                pass
        elif platform.system() == "Darwin":  # macOS
            try:
                result = subprocess.run(
                    ["sysctl", "-n", "machdep.cpu.brand_string"],
                    capture_output=True,
                    text=True,
                    timeout=5,
                )
                if result.returncode == 0:
                    cpu_info["model"] = result.stdout.strip()
            except Exception:
                pass
        elif platform.system() == "Windows":
            try:
                result = subprocess.run(
                    ["wmic", "cpu", "get", "name"],
                    capture_output=True,
                    text=True,
                    timeout=5,
                )
                if result.returncode == 0:
                    lines = result.stdout.strip().split("\n")
                    if len(lines) > 1:
                        cpu_info["model"] = lines[1].strip()
            except Exception:
                pass

        # Try to get physical core count
        try:
            if platform.system() == "Linux":
                # Count unique physical IDs in /proc/cpuinfo
                physical_ids = set()
                cores_per_socket = 1
                with open("/proc/cpuinfo", "r") as f:
                    current_physical_id = None
                    for line in f:
                        if "physical id" in line:
                            current_physical_id = line.split(":")[1].strip()
                            physical_ids.add(current_physical_id)
                        elif "cpu cores" in line:
                            cores_per_socket = int(line.split(":")[1].strip())
                if physical_ids:
                    cpu_info["physical_cores"] = len(physical_ids) * cores_per_socket
                else:
                    cpu_info["physical_cores"] = cpu_info["threads"]
            else:
                # For non-Linux, assume physical cores = threads / 2 (SMT assumption)
                cpu_info["physical_cores"] = max(1, cpu_info["threads"] // 2)
        except Exception:
            cpu_info["physical_cores"] = cpu_info["threads"]

    except Exception as e:
        print(f"Warning: Could not fully detect CPU info: {e}", file=sys.stderr)

    return cpu_info


def detect_nvidia_gpu() -> Optional[Dict[str, str]]:
    """Detect NVIDIA GPU using nvidia-smi."""
    try:
        result = subprocess.run(
            ["nvidia-smi", "--query-gpu=name,driver_version", "--format=csv,noheader"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode == 0 and result.stdout.strip():
            lines = result.stdout.strip().split("\n")
            if lines:
                parts = lines[0].split(",")
                return {
                    "name": parts[0].strip() if len(parts) > 0 else "NVIDIA GPU",
                    "driver": parts[1].strip() if len(parts) > 1 else "Unknown",
                }
    except FileNotFoundError:
        pass
    except Exception:
        pass
    return None


def get_total_ram_gb() -> float:
    """Get total system RAM in GB."""
    try:
        if platform.system() == "Linux":
            with open("/proc/meminfo", "r") as f:
                for line in f:
                    if "MemTotal" in line:
                        kb = int(line.split()[1])
                        return kb / (1024 * 1024)
        elif platform.system() == "Darwin":  # macOS
            result = subprocess.run(
                ["sysctl", "-n", "hw.memsize"],
                capture_output=True,
                text=True,
                timeout=5,
            )
            if result.returncode == 0:
                bytes_mem = int(result.stdout.strip())
                return bytes_mem / (1024 ** 3)
        elif platform.system() == "Windows":
            result = subprocess.run(
                ["wmic", "computersystem", "get", "totalphysicalmemory"],
                capture_output=True,
                text=True,
                timeout=5,
            )
            if result.returncode == 0:
                lines = result.stdout.strip().split("\n")
                if len(lines) > 1:
                    bytes_mem = int(lines[1].strip())
                    return bytes_mem / (1024 ** 3)
    except Exception:
        pass
    return 8.0  # Default assumption


def is_laptop() -> bool:
    """Detect if the system is a laptop (battery-powered)."""
    try:
        # Linux: Check for battery in /sys/class/power_supply
        if platform.system() == "Linux":
            power_supply_path = Path("/sys/class/power_supply")
            if power_supply_path.exists():
                for item in power_supply_path.iterdir():
                    if "BAT" in item.name.upper():
                        return True
        
        # macOS: Check battery using system_profiler
        elif platform.system() == "Darwin":
            result = subprocess.run(
                ["system_profiler", "SPPowerDataType"],
                capture_output=True,
                text=True,
                timeout=5,
            )
            if "Battery" in result.stdout:
                return True
        
        # Windows: Check using WMIC
        elif platform.system() == "Windows":
            result = subprocess.run(
                ["wmic", "path", "win32_battery", "get", "status"],
                capture_output=True,
                text=True,
                timeout=5,
            )
            # If battery exists, it will return status
            if result.returncode == 0 and "Status" in result.stdout:
                return True
    except Exception:
        pass
    
    return False


def detect_system_type(cpu_threads: int, ram_gb: float, has_battery: bool) -> str:
    """
    Classify system as laptop, desktop, or server.
    
    Logic:
    - Laptop: Has battery
    - Server: >16 threads AND >32GB RAM AND no battery
    - Desktop: Everything else
    """
    if has_battery:
        return "laptop"
    elif cpu_threads > 16 and ram_gb > 32:
        return "server"
    else:
        return "desktop"


def recommend_config(
    cpu_threads: int,
    has_gpu: bool,
    system_type: str,
    ram_gb: float,
) -> Dict[str, any]:
    """
    Recommend optimal test configuration based on hardware.
    
    Logic:
    - If NVIDIA GPU exists → prioritize NVENC benchmarks
    - If CPU has > 12 threads → test 4K 30/60fps
    - If CPU has 8–12 threads → test 1440p or 1080p60
    - If < 8 threads → test 1080p30 or 720p
    - If laptop + high temps → shorter duration (120s)
    - Servers → full duration (300s) & multi-output ABR ladder if possible
    
    Returns recommended configuration dict.
    """
    config = {
        "name": "recommended_test",
        "encoder": "h264",
        "preset": "fast",
        "bitrate": "5000k",
        "resolution": "1280x720",
        "fps": 30,
        "duration": 300,
    }
    
    reasons = []
    
    # 1. Encoder selection: GPU first, then CPU
    if has_gpu:
        config["encoder"] = "h264_nvenc"
        # NVENC presets: slow, medium, fast, hp, hq, bd, ll, llhq, llhp, lossless
        config["preset"] = "medium"
        reasons.append(
            "NVIDIA GPU detected → Using hardware-accelerated NVENC encoder"
        )
    else:
        config["encoder"] = "h264"
        if cpu_threads >= 12:
            config["preset"] = "fast"
            reasons.append(
                f"CPU with {cpu_threads} threads detected → "
                "Using x264 'fast' preset for better quality"
            )
        elif cpu_threads >= 8:
            config["preset"] = "fast"
            reasons.append(
                f"CPU with {cpu_threads} threads → Using x264 'fast' preset"
            )
        else:
            config["preset"] = "veryfast"
            reasons.append(
                f"CPU with {cpu_threads} threads → "
                "Using x264 'veryfast' preset for lighter load"
            )
    
    # 2. Resolution and FPS selection based on CPU threads
    if cpu_threads > 12:
        # High-end system: Test 4K
        config["resolution"] = "3840x2160"
        config["fps"] = 30
        config["bitrate"] = "15000k"
        reasons.append(
            f"High thread count ({cpu_threads}) → "
            "Testing 4K resolution for comprehensive benchmark"
        )

        # If even more powerful, try 60fps
        if cpu_threads >= 16:
            config["fps"] = 60
            config["bitrate"] = "20000k"
            reasons.append(
                f"Very high thread count ({cpu_threads}) → "
                "Testing 4K@60fps for maximum stress"
            )
    
    elif 8 <= cpu_threads <= 12:
        # Mid-range system: Test 1440p or 1080p60
        if has_gpu:
            config["resolution"] = "2560x1440"
            config["fps"] = 60
            config["bitrate"] = "12000k"
            reasons.append(f"Mid-range CPU ({cpu_threads} threads) + GPU → Testing 1440p@60fps")
        else:
            config["resolution"] = "1920x1080"
            config["fps"] = 60
            config["bitrate"] = "8000k"
            reasons.append(f"Mid-range CPU ({cpu_threads} threads) → Testing 1080p@60fps")
    
    else:
        # Lower-end system: Test 1080p30 or 720p
        if cpu_threads >= 4:
            config["resolution"] = "1920x1080"
            config["fps"] = 30
            config["bitrate"] = "5000k"
            reasons.append(f"Lower thread count ({cpu_threads}) → Testing 1080p@30fps")
        else:
            config["resolution"] = "1280x720"
            config["fps"] = 30
            config["bitrate"] = "3000k"
            reasons.append(
                f"Limited threads ({cpu_threads}) → "
                "Testing 720p@30fps for stability"
            )

    # 3. Duration adjustment based on system type
    if system_type == "laptop":
        config["duration"] = 120
        reasons.append(
            "Laptop detected → "
            "Reduced test duration (120s) to minimize thermal impact"
        )
    elif system_type == "server":
        config["duration"] = 300
        reasons.append(
            "Server environment → "
            "Extended test duration (300s) for thorough analysis"
        )
    else:  # desktop
        config["duration"] = 180
        reasons.append("Desktop system → Standard test duration (180s)")

    # 4. Add reason about bitrate selection
    reasons.append(
        f"Selected bitrate {config['bitrate']} appropriate for "
        f"{config['resolution']}@{config['fps']}fps"
    )
    
    config["reasons"] = reasons
    return config


def format_command(config: Dict[str, any]) -> str:
    """Format the recommended configuration as a run_tests.py command."""
    cmd = f"""python3 scripts/run_tests.py single \\
  --name {config['name']} \\
  --encoder {config['encoder']} \\
  --preset {config['preset']} \\
  --bitrate {config['bitrate']} \\
  --resolution {config['resolution']} \\
  --fps {config['fps']} \\
  --duration {config['duration']}"""
    return cmd


def main():
    """Main entry point."""
    print("=" * 70)
    print("Hardware-Aware Benchmark Recommendation Tool")
    print("=" * 70)
    print()
    
    # Detect hardware
    print("🔍 Detecting hardware configuration...")
    print()
    
    cpu_info = get_cpu_info()
    gpu_info = detect_nvidia_gpu()
    ram_gb = get_total_ram_gb()
    has_battery = is_laptop()
    system_type = detect_system_type(cpu_info["threads"], ram_gb, has_battery)
    
    # Print detected hardware
    print(f"CPU: {cpu_info['model']}")
    print(f"Threads: {cpu_info['threads']} (Physical cores: {cpu_info['physical_cores']})")
    print(f"RAM: {ram_gb:.1f} GB")
    if gpu_info:
        print(f"GPU: {gpu_info['name']} (Driver: {gpu_info['driver']})")
    else:
        print("GPU: Not detected")
    print(f"System Type: {system_type.upper()}")
    if has_battery:
        print("⚡ Battery detected - mobile system")
    print()
    
    # Generate recommendation
    print("-" * 70)
    print("🎯 Generating optimal benchmark configuration...")
    print()
    
    config = recommend_config(
        cpu_threads=cpu_info["threads"],
        has_gpu=gpu_info is not None,
        system_type=system_type,
        ram_gb=ram_gb,
    )
    
    # Print reasoning
    print("💡 Configuration Rationale:")
    for i, reason in enumerate(config["reasons"], 1):
        print(f"  {i}. {reason}")
    print()
    
    # Print recommended command
    print("-" * 70)
    print("✅ Recommended Command:")
    print()
    print(format_command(config))
    print()
    print("=" * 70)
    print()
    print("Note: This recommendation prioritizes comprehensive benchmarking over speed.")
    print("You can adjust parameters based on your specific needs.")
    print()
    
    return 0


if __name__ == "__main__":
    sys.exit(main())
