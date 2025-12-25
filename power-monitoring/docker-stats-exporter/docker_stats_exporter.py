#!/usr/bin/env python3
"""
Docker Overhead Monitoring Exporter
Monitors Docker engine and container resource usage for overhead calculation
"""

import time
import subprocess
import json
import re
from http.server import HTTPServer, BaseHTTPRequestHandler
import os
import socket


class DockerStatsCollector:
    def __init__(self):
        self.total_cpu_cores = self._get_cpu_cores()
        
    def _get_cpu_cores(self):
        """Get total number of CPU cores"""
        try:
            with open('/proc/cpuinfo', 'r') as f:
                return len([line for line in f if line.startswith('processor')])
        except:
            return os.cpu_count() or 4
    
    def _run_command(self, cmd):
        """Run shell command and return output"""
        try:
            result = subprocess.run(
                cmd, 
                shell=True, 
                capture_output=True, 
                text=True, 
                timeout=5
            )
            return result.stdout.strip()
        except Exception as e:
            print(f"Error running command '{cmd}': {e}")
            return None
    
    def get_docker_engine_stats(self):
        """Get Docker engine (dockerd) process stats"""
        cmd = "ps aux | grep dockerd | grep -v grep"
        output = self._run_command(cmd)
        
        if not output:
            return None
        
        parts = output.split()
        if len(parts) < 11:
            return None
        
        try:
            return {
                'cpu_percent': float(parts[2]),
                'memory_percent': float(parts[3]),
                'memory_kb': float(parts[5])
            }
        except (ValueError, IndexError):
            return None
    
    def get_container_stats(self):
        """Get stats for all running containers"""
        cmd = "docker stats --no-stream --format '{{json .}}'"
        output = self._run_command(cmd)
        
        if not output:
            return []
        
        containers = []
        for line in output.split('\n'):
            if line.strip():
                try:
                    container = json.loads(line)
                    # Parse CPU percentage (remove %)
                    cpu_str = container.get('CPUPerc', '0%').replace('%', '')
                    mem_str = container.get('MemPerc', '0%').replace('%', '')
                    
                    containers.append({
                        'name': container.get('Name', 'unknown'),
                        'id': container.get('Container', 'unknown')[:12],
                        'cpu_percent': float(cpu_str) if cpu_str else 0.0,
                        'memory_percent': float(mem_str) if mem_str else 0.0,
                        'memory_usage': container.get('MemUsage', 'N/A'),
                        'net_io': container.get('NetIO', 'N/A'),
                        'block_io': container.get('BlockIO', 'N/A')
                    })
                except (json.JSONDecodeError, ValueError) as e:
                    print(f"Error parsing container stats: {e}")
                    continue
        
        return containers
    
    def calculate_overhead_watts(self, docker_cpu_percent, total_system_watts):
        """
        Estimate Docker overhead in watts
        
        Args:
            docker_cpu_percent: CPU % used by dockerd process
            total_system_watts: Total system power consumption
        
        Returns:
            Estimated watts consumed by Docker overhead
        """
        # Rough estimation: Docker overhead watts = (docker_cpu% / 100) * total_watts
        # This is a simplification; actual relationship may be more complex
        if docker_cpu_percent and total_system_watts:
            overhead_watts = (docker_cpu_percent / 100.0) * total_system_watts
            return overhead_watts
        return 0.0


class MetricsHandler(BaseHTTPRequestHandler):
    collector = None
    
    def do_GET(self):
        if self.path == '/metrics':
            try:
                self.send_response(200)
                self.send_header('Content-type', 'text/plain; charset=utf-8')
                self.end_headers()
                
                output = []
                
                # Docker engine stats
                engine_stats = self.collector.get_docker_engine_stats()
                if engine_stats:
                    output.append("# HELP docker_engine_cpu_percent CPU percentage used by Docker engine")
                    output.append("# TYPE docker_engine_cpu_percent gauge")
                    output.append(f"docker_engine_cpu_percent {engine_stats['cpu_percent']:.2f}")
                    
                    output.append("# HELP docker_engine_memory_percent Memory percentage used by Docker engine")
                    output.append("# TYPE docker_engine_memory_percent gauge")
                    output.append(f"docker_engine_memory_percent {engine_stats['memory_percent']:.2f}")
                    
                    output.append("# HELP docker_engine_memory_kb Memory in KB used by Docker engine")
                    output.append("# TYPE docker_engine_memory_kb gauge")
                    output.append(f"docker_engine_memory_kb {engine_stats['memory_kb']:.0f}")
                
                # Container stats
                containers = self.collector.get_container_stats()
                if containers:
                    output.append("# HELP docker_container_cpu_percent CPU percentage used by container")
                    output.append("# TYPE docker_container_cpu_percent gauge")
                    
                    output.append("# HELP docker_container_memory_percent Memory percentage used by container")
                    output.append("# TYPE docker_container_memory_percent gauge")
                    
                    for container in containers:
                        name = container['name']
                        output.append(f'docker_container_cpu_percent{{container="{name}"}} {container["cpu_percent"]:.2f}')
                        output.append(f'docker_container_memory_percent{{container="{name}"}} {container["memory_percent"]:.2f}')
                    
                    # Total container CPU usage
                    total_container_cpu = sum(c['cpu_percent'] for c in containers)
                    output.append("# HELP docker_containers_total_cpu_percent Total CPU percentage across all containers")
                    output.append("# TYPE docker_containers_total_cpu_percent gauge")
                    output.append(f"docker_containers_total_cpu_percent {total_container_cpu:.2f}")
                
                self.wfile.write('\n'.join(output).encode('utf-8'))
                self.wfile.write(b'\n')
            except (BrokenPipeError, ConnectionResetError, socket.error):
                pass
        elif self.path == '/health':
            try:
                self.send_response(200)
                self.send_header('Content-type', 'text/plain; charset=utf-8')
                self.end_headers()
                self.wfile.write(b'OK\n')
            except (BrokenPipeError, ConnectionResetError, socket.error):
                pass
        else:
            try:
                self.send_response(404)
                self.end_headers()
            except (BrokenPipeError, ConnectionResetError, socket.error):
                pass
    
    def log_message(self, format, *args):
        pass


def main():
    port = int(os.getenv('DOCKER_STATS_PORT', '9501'))
    
    print(f"Starting Docker Overhead Exporter on port {port}")
    
    collector = DockerStatsCollector()
    MetricsHandler.collector = collector
    
    server = HTTPServer(('0.0.0.0', port), MetricsHandler)
    
    print(f"Exporter ready at http://0.0.0.0:{port}/metrics")
    
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...")
        server.shutdown()


if __name__ == '__main__':
    main()
