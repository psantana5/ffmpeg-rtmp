#!/usr/bin/env python3
"""
Docker Overhead Monitoring Exporter
Monitors Docker engine and container resource usage
"""

import time
import subprocess
import json
import logging
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import Dict, List, Optional
import os

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class DockerStatsCollector:
    """Collects Docker engine and container statistics"""
    
    def __init__(self):
        self.total_cpu_cores = self._get_cpu_cores()
        logger.info(f"Detected {self.total_cpu_cores} CPU cores")
        
    def _get_cpu_cores(self) -> int:
        """Get total number of CPU cores"""
        try:
            # Try reading from cgroups first (more reliable in containers)
            cpu_quota_file = '/sys/fs/cgroup/cpu/cpu.cfs_quota_us'
            cpu_period_file = '/sys/fs/cgroup/cpu/cpu.cfs_period_us'
            
            if os.path.exists(cpu_quota_file) and os.path.exists(cpu_period_file):
                with open(cpu_quota_file) as f:
                    quota = int(f.read().strip())
                with open(cpu_period_file) as f:
                    period = int(f.read().strip())
                if quota > 0:
                    return max(1, quota // period)
            
            # Fallback to /proc/cpuinfo
            with open('/host/proc/cpuinfo' if os.path.exists('/host/proc') else '/proc/cpuinfo', 'r') as f:
                return len([line for line in f if line.startswith('processor')])
        except Exception as e:
            logger.warning(f"Error detecting CPU cores: {e}, defaulting to 4")
            return os.cpu_count() or 4
    
    def _run_command(self, cmd: str, timeout: int = 5) -> Optional[str]:
        """Run shell command and return output"""
        try:
            result = subprocess.run(
                cmd, 
                shell=True, 
                capture_output=True, 
                text=True, 
                timeout=timeout
            )
            if result.returncode == 0:
                return result.stdout.strip()
            else:
                logger.debug(f"Command failed: {cmd}, stderr: {result.stderr}")
                return None
        except subprocess.TimeoutExpired:
            logger.warning(f"Command timed out: {cmd}")
            return None
        except Exception as e:
            logger.error(f"Error running command '{cmd}': {e}")
            return None
    
    def get_docker_engine_stats(self) -> Optional[Dict]:
        """Get Docker engine (dockerd/containerd) process stats"""
        # Try both dockerd and containerd
        for process_name in ['dockerd', 'containerd']:
            cmd = f"ps aux | grep {process_name} | grep -v grep | head -1"
            output = self._run_command(cmd)
            
            if not output:
                continue
            
            parts = output.split()
            if len(parts) < 11:
                continue
            
            try:
                return {
                    'process': process_name,
                    'cpu_percent': float(parts[2]),
                    'memory_percent': float(parts[3]),
                    'memory_kb': float(parts[5]) if len(parts) > 5 else 0
                }
            except (ValueError, IndexError) as e:
                logger.debug(f"Error parsing {process_name} stats: {e}")
                continue
        
        logger.warning("Could not find Docker/containerd process stats")
        return None
    
    def get_container_stats(self) -> List[Dict]:
        """Get stats for all running containers"""
        cmd = "docker stats --no-stream --format '{{json .}}' 2>/dev/null"
        output = self._run_command(cmd, timeout=10)
        
        if not output:
            return []
        
        containers = []
        for line in output.split('\n'):
            line = line.strip()
            if not line:
                continue
                
            try:
                container = json.loads(line)
                cpu_str = container.get('CPUPerc', '0%').replace('%', '')
                mem_str = container.get('MemPerc', '0%').replace('%', '')
                
                # Parse memory usage
                mem_usage = container.get('MemUsage', '0B / 0B')
                mem_parts = mem_usage.split('/')
                used_mem = mem_parts[0].strip() if len(mem_parts) > 0 else '0B'
                
                containers.append({
                    'name': container.get('Name', 'unknown'),
                    'id': container.get('Container', 'unknown')[:12],
                    'cpu_percent': float(cpu_str) if cpu_str else 0.0,
                    'memory_percent': float(mem_str) if mem_str else 0.0,
                    'memory_usage': used_mem,
                    'net_io': container.get('NetIO', 'N/A'),
                    'block_io': container.get('BlockIO', 'N/A')
                })
            except (json.JSONDecodeError, ValueError) as e:
                logger.debug(f"Error parsing container stats line: {e}")
                continue
        
        return containers


class MetricsHandler(BaseHTTPRequestHandler):
    """HTTP handler for Prometheus metrics endpoint"""
    collector = None
    
    def do_GET(self):
        if self.path == '/metrics':
            self.send_response(200)
            self.send_header('Content-type', 'text/plain; charset=utf-8')
            self.end_headers()
            
            output = []
            
            # Docker engine stats
            engine_stats = self.collector.get_docker_engine_stats()
            if engine_stats:
                output.append("# HELP docker_engine_cpu_percent CPU percentage used by Docker engine")
                output.append("# TYPE docker_engine_cpu_percent gauge")
                output.append(f'docker_engine_cpu_percent{{process="{engine_stats["process"]}"}} {engine_stats["cpu_percent"]:.2f}')
                
                output.append("# HELP docker_engine_memory_percent Memory percentage used by Docker engine")
                output.append("# TYPE docker_engine_memory_percent gauge")
                output.append(f'docker_engine_memory_percent{{process="{engine_stats["process"]}"}} {engine_stats["memory_percent"]:.2f}')
                
                output.append("# HELP docker_engine_memory_kb Memory in KB used by Docker engine")
                output.append("# TYPE docker_engine_memory_kb gauge")
                output.append(f'docker_engine_memory_kb{{process="{engine_stats["process"]}"}} {engine_stats["memory_kb"]:.0f}')
            
            # Container stats
            containers = self.collector.get_container_stats()
            if containers:
                output.append("# HELP docker_container_cpu_percent CPU percentage used by container")
                output.append("# TYPE docker_container_cpu_percent gauge")
                
                output.append("# HELP docker_container_memory_percent Memory percentage used by container")
                output.append("# TYPE docker_container_memory_percent gauge")
                
                for container in containers:
                    name = container['name']
                    cid = container['id']
                    output.append(f'docker_container_cpu_percent{{container="{name}",id="{cid}"}} {container["cpu_percent"]:.2f}')
                    output.append(f'docker_container_memory_percent{{container="{name}",id="{cid}"}} {container["memory_percent"]:.2f}')
                
                # Total container CPU usage
                total_container_cpu = sum(c['cpu_percent'] for c in containers)
                output.append("# HELP docker_containers_total_cpu_percent Total CPU percentage across all containers")
                output.append("# TYPE docker_containers_total_cpu_percent gauge")
                output.append(f"docker_containers_total_cpu_percent {total_container_cpu:.2f}")
                
                # Container count
                output.append("# HELP docker_containers_running Number of running containers")
                output.append("# TYPE docker_containers_running gauge")
                output.append(f"docker_containers_running {len(containers)}")
            
            # Timestamp
            output.append("# HELP docker_stats_scrape_timestamp_seconds Unix timestamp of last scrape")
            output.append("# TYPE docker_stats_scrape_timestamp_seconds gauge")
            output.append(f"docker_stats_scrape_timestamp_seconds {time.time():.0f}")
            
            self.wfile.write('\n'.join(output).encode('utf-8'))
            self.wfile.write(b'\n')
        elif self.path == '/health':
            self.send_response(200)
            self.send_header('Content-type', 'text/plain')
            self.end_headers()
            self.wfile.write(b'OK\n')
        else:
            self.send_response(404)
            self.end_headers()
    
    def log_message(self, format, *args):
        pass


def main():
    port = int(os.getenv('DOCKER_STATS_PORT', '9501'))
    
    logger.info(f"Starting Docker Overhead Exporter on port {port}")
    
    collector = DockerStatsCollector()
    MetricsHandler.collector = collector
    
    server = HTTPServer(('0.0.0.0', port), MetricsHandler)
    
    logger.info(f"Exporter ready at http://0.0.0.0:{port}/metrics")
    
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        logger.info("Shutting down...")
        server.shutdown()
    
    return 0


if __name__ == '__main__':
    exit(main())
