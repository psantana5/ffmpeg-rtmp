#!/usr/bin/env python3
"""
RAPL Power Monitoring Exporter for Prometheus
Reads Intel RAPL (Running Average Power Limit) data and exposes as Prometheus metrics
"""

import time
import os
import logging
from pathlib import Path
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import Dict, Optional, Tuple

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class RAPLReader:
    """Reads Intel RAPL power consumption data"""
    
    def __init__(self, base_path: str = "/sys/class/powercap"):
        self.base_path = Path(base_path)
        self.zones = self._discover_zones()
        self.previous_readings: Dict[str, Tuple[int, float]] = {}
        
        if not self.zones:
            logger.error("No RAPL zones found. Check if running on Intel CPU with root access.")
        else:
            logger.info(f"Discovered RAPL zones: {list(self.zones.keys())}")
        
    def _discover_zones(self) -> Dict:
        """Discover available RAPL zones (package, core, uncore, dram, etc.)"""
        zones = {}
        
        if not self.base_path.exists():
            logger.error(f"RAPL interface not found at {self.base_path}")
            return zones
            
        try:
            for item in self.base_path.glob("intel-rapl:*"):
                if not item.is_dir():
                    continue
                    
                name_file = item / "name"
                if not name_file.exists():
                    continue
                    
                zone_name = name_file.read_text().strip()
                energy_uj_file = item / "energy_uj"
                max_energy_file = item / "max_energy_range_uj"
                
                if energy_uj_file.exists():
                    max_range = None
                    if max_energy_file.exists():
                        try:
                            max_range = int(max_energy_file.read_text().strip())
                        except (ValueError, IOError):
                            pass
                    
                    zones[zone_name] = {
                        'path': energy_uj_file,
                        'max_range': max_range,
                        'subzones': {}
                    }
                    
                    # Check for subzones
                    for subitem in item.glob("intel-rapl:*:*"):
                        if not subitem.is_dir():
                            continue
                            
                        subname_file = subitem / "name"
                        if not subname_file.exists():
                            continue
                            
                        subzone_name = subname_file.read_text().strip()
                        sub_energy_file = subitem / "energy_uj"
                        
                        if sub_energy_file.exists():
                            sub_max_range = None
                            sub_max_file = subitem / "max_energy_range_uj"
                            if sub_max_file.exists():
                                try:
                                    sub_max_range = int(sub_max_file.read_text().strip())
                                except (ValueError, IOError):
                                    pass
                            
                            zones[zone_name]['subzones'][subzone_name] = {
                                'path': sub_energy_file,
                                'max_range': sub_max_range
                            }
        except Exception as e:
            logger.error(f"Error discovering RAPL zones: {e}")
        
        return zones
    
    def _read_energy_uj(self, path: Path) -> Optional[int]:
        """Read energy counter in microjoules"""
        try:
            return int(path.read_text().strip())
        except (IOError, ValueError) as e:
            logger.debug(f"Error reading {path}: {e}")
            return None
    
    def get_power_watts(self) -> Dict[str, float]:
        """Calculate power consumption in watts based on energy delta"""
        current_time = time.time()
        power_data = {}
        
        for zone_name, zone_info in self.zones.items():
            current_energy = self._read_energy_uj(zone_info['path'])
            
            if current_energy is not None:
                key = f"main_{zone_name}"
                
                if key in self.previous_readings:
                    prev_energy, prev_time = self.previous_readings[key]
                    time_delta = current_time - prev_time
                    
                    if time_delta > 0:
                        energy_delta = current_energy - prev_energy
                        
                        # Handle counter wraparound
                        if energy_delta < 0 and zone_info['max_range']:
                            energy_delta += zone_info['max_range']
                        
                        # Convert microjoules to watts
                        if energy_delta >= 0:
                            power_watts = (energy_delta / 1_000_000) / time_delta
                            power_data[zone_name] = power_watts
                
                self.previous_readings[key] = (current_energy, current_time)
            
            # Process subzones
            for subzone_name, subzone_info in zone_info['subzones'].items():
                current_sub_energy = self._read_energy_uj(subzone_info['path'])
                
                if current_sub_energy is not None:
                    sub_key = f"sub_{zone_name}_{subzone_name}"
                    
                    if sub_key in self.previous_readings:
                        prev_energy, prev_time = self.previous_readings[sub_key]
                        time_delta = current_time - prev_time
                        
                        if time_delta > 0:
                            energy_delta = current_sub_energy - prev_energy
                            
                            if energy_delta < 0 and subzone_info['max_range']:
                                energy_delta += subzone_info['max_range']
                            
                            if energy_delta >= 0:
                                power_watts = (energy_delta / 1_000_000) / time_delta
                                power_data[f"{zone_name}_{subzone_name}"] = power_watts
                    
                    self.previous_readings[sub_key] = (current_sub_energy, current_time)
        
        return power_data


class MetricsHandler(BaseHTTPRequestHandler):
    """HTTP handler for Prometheus metrics endpoint"""
    rapl_reader = None
    
    def do_GET(self):
        if self.path == '/metrics':
            self.send_response(200)
            self.send_header('Content-type', 'text/plain; charset=utf-8')
            self.end_headers()
            
            power_data = self.rapl_reader.get_power_watts()
            
            output = []
            output.append("# HELP rapl_power_watts Current power consumption in watts from RAPL")
            output.append("# TYPE rapl_power_watts gauge")
            
            for zone, watts in power_data.items():
                safe_zone = zone.lower().replace('-', '_').replace(' ', '_')
                output.append(f'rapl_power_watts{{zone="{safe_zone}"}} {watts:.4f}')
            
            # Add timestamp
            output.append(f"# HELP rapl_scrape_timestamp_seconds Unix timestamp of last scrape")
            output.append(f"# TYPE rapl_scrape_timestamp_seconds gauge")
            output.append(f"rapl_scrape_timestamp_seconds {time.time():.0f}")
            
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
    port = int(os.getenv('RAPL_EXPORTER_PORT', '9500'))
    
    logger.info(f"Starting RAPL Power Exporter on port {port}")
    
    rapl_reader = RAPLReader()
    
    if not rapl_reader.zones:
        logger.error("No RAPL zones found. Exiting.")
        return 1
    
    # Initial reading to establish baseline
    time.sleep(1)
    rapl_reader.get_power_watts()
    
    MetricsHandler.rapl_reader = rapl_reader
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
