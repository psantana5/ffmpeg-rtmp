#!/usr/bin/env python3
"""
Hardware Info - Display system hardware information.

Shows CPU model, frequency, core count, and other hardware details.
"""

import argparse
import json
import logging
import platform
import sys
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def get_cpu_info():
    """Get CPU information."""
    info = {
        'processor': platform.processor(),
        'machine': platform.machine(),
        'system': platform.system(),
    }
    
    # Try to get more detailed CPU info on Linux
    try:
        with open('/proc/cpuinfo', 'r') as f:
            cpuinfo = f.read()
            
        # Extract model name
        for line in cpuinfo.split('\n'):
            if 'model name' in line.lower():
                info['model'] = line.split(':')[1].strip()
                break
        
        # Count cores
        core_count = cpuinfo.count('processor')
        info['cores'] = core_count
        
    except Exception as e:
        logger.debug(f"Could not read /proc/cpuinfo: {e}")
    
    return info


def get_memory_info():
    """Get memory information."""
    info = {}
    
    try:
        with open('/proc/meminfo', 'r') as f:
            meminfo = f.read()
        
        for line in meminfo.split('\n'):
            if 'MemTotal' in line:
                # Extract memory in KB and convert to GB
                mem_kb = int(line.split()[1])
                info['total_gb'] = round(mem_kb / 1024 / 1024, 2)
            elif 'MemAvailable' in line:
                mem_kb = int(line.split()[1])
                info['available_gb'] = round(mem_kb / 1024 / 1024, 2)
    
    except Exception as e:
        logger.debug(f"Could not read /proc/meminfo: {e}")
    
    return info


def check_rapl_support():
    """Check if Intel RAPL is available."""
    rapl_path = Path('/sys/class/powercap/intel-rapl:0')
    return rapl_path.exists()


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='Display system hardware information',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Show hardware info
  python3 hardware_info.py
  
  # JSON output
  python3 hardware_info.py --json
        """
    )
    
    parser.add_argument(
        '--json',
        action='store_true',
        help='Output in JSON format'
    )
    
    parser.add_argument(
        '--verbose', '-v',
        action='store_true',
        help='Enable verbose logging'
    )
    
    args = parser.parse_args()
    
    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)
    
    # Collect hardware info
    cpu_info = get_cpu_info()
    memory_info = get_memory_info()
    rapl_available = check_rapl_support()
    
    hardware_info = {
        'cpu': cpu_info,
        'memory': memory_info,
        'rapl_support': rapl_available,
        'platform': {
            'system': platform.system(),
            'release': platform.release(),
            'version': platform.version(),
            'python_version': platform.python_version(),
        }
    }
    
    if args.json:
        print(json.dumps(hardware_info, indent=2))
    else:
        # Human-readable output
        print(f"\n{'='*70}")
        print("HARDWARE INFORMATION")
        print(f"{'='*70}")
        
        print(f"\nCPU:")
        print(f"  Model:      {cpu_info.get('model', cpu_info.get('processor', 'Unknown'))}")
        print(f"  Cores:      {cpu_info.get('cores', 'Unknown')}")
        print(f"  Machine:    {cpu_info.get('machine', 'Unknown')}")
        
        if memory_info:
            print(f"\nMemory:")
            print(f"  Total:      {memory_info.get('total_gb', 'Unknown')} GB")
            print(f"  Available:  {memory_info.get('available_gb', 'Unknown')} GB")
        
        print(f"\nPower Monitoring:")
        print(f"  Intel RAPL: {'Available' if rapl_available else 'Not Available'}")
        
        print(f"\nPlatform:")
        print(f"  OS:         {platform.system()} {platform.release()}")
        print(f"  Python:     {platform.python_version()}")
        
        print(f"\n{'='*70}\n")


if __name__ == '__main__':
    main()
