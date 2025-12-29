#!/usr/bin/env python3
"""
RAPL Power Monitoring Exporter for Prometheus
Reads Intel RAPL (Running Average Power Limit) data and exposes as Prometheus metrics
"""

import os
import time
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path


class RAPLReader:
    def __init__(self):
        self.base_path = Path("/sys/class/powercap")
        self.zones = self._discover_zones()
        self.previous_readings = {}

    def _discover_zones(self):
        """Discover available RAPL zones (package, core, uncore, dram, etc.)"""
        zones = {}

        if not self.base_path.exists():
            print(
                "ERROR: RAPL interface not found. Are you running on Intel CPU with root/privileged access?"
            )
            return zones

        for item in self.base_path.glob("intel-rapl:*"):
            if item.is_dir():
                name_file = item / "name"
                if name_file.exists():
                    zone_name = name_file.read_text().strip()
                    energy_uj_file = item / "energy_uj"
                    max_energy_file = item / "max_energy_range_uj"

                    if energy_uj_file.exists():
                        zones[zone_name] = {
                            "path": energy_uj_file,
                            "max_range": int(max_energy_file.read_text().strip())
                            if max_energy_file.exists()
                            else None,
                            "subzones": {},
                        }

                        # Check for subzones (like core, uncore within package)
                        for subitem in item.glob("intel-rapl:*:*"):
                            if subitem.is_dir():
                                subname_file = subitem / "name"
                                if subname_file.exists():
                                    subzone_name = subname_file.read_text().strip()
                                    sub_energy_file = subitem / "energy_uj"
                                    sub_max_file = subitem / "max_energy_range_uj"

                                    if sub_energy_file.exists():
                                        zones[zone_name]["subzones"][subzone_name] = {
                                            "path": sub_energy_file,
                                            "max_range": int(
                                                sub_max_file.read_text().strip()
                                            )
                                            if sub_max_file.exists()
                                            else None,
                                        }

        return zones

    def _read_energy_uj(self, path):
        """Read energy counter in microjoules"""
        try:
            return int(path.read_text().strip())
        except (IOError, ValueError) as e:
            print(f"Error reading {path}: {e}")
            return None

    def get_power_watts(self):
        """Calculate power consumption in watts based on energy delta"""
        current_time = time.time()
        power_data = {}

        for zone_name, zone_info in self.zones.items():
            current_energy = self._read_energy_uj(zone_info["path"])

            if current_energy is not None:
                key = f"main_{zone_name}"

                if key in self.previous_readings:
                    prev_energy, prev_time = self.previous_readings[key]
                    time_delta = current_time - prev_time

                    # Handle counter wraparound
                    energy_delta = current_energy - prev_energy
                    if energy_delta < 0 and zone_info["max_range"]:
                        energy_delta += zone_info["max_range"]

                    # Convert microjoules to joules, then to watts (J/s)
                    if time_delta > 0:
                        power_watts = (energy_delta / 1_000_000) / time_delta
                        power_data[zone_name] = power_watts

                self.previous_readings[key] = (current_energy, current_time)

            # Process subzones
            for subzone_name, subzone_info in zone_info["subzones"].items():
                current_sub_energy = self._read_energy_uj(subzone_info["path"])

                if current_sub_energy is not None:
                    sub_key = f"sub_{zone_name}_{subzone_name}"

                    if sub_key in self.previous_readings:
                        prev_energy, prev_time = self.previous_readings[sub_key]
                        time_delta = current_time - prev_time

                        energy_delta = current_sub_energy - prev_energy
                        if energy_delta < 0 and subzone_info["max_range"]:
                            energy_delta += subzone_info["max_range"]

                        if time_delta > 0:
                            power_watts = (energy_delta / 1_000_000) / time_delta
                            power_data[f"{zone_name}_{subzone_name}"] = power_watts

                    self.previous_readings[sub_key] = (current_sub_energy, current_time)

        return power_data


class MetricsHandler(BaseHTTPRequestHandler):
    rapl_reader = None

    def do_GET(self):
        if self.path == "/metrics":
            self.send_response(200)
            self.send_header("Content-type", "text/plain; charset=utf-8")
            self.end_headers()

            # Get power readings
            power_data = self.rapl_reader.get_power_watts()

            # Generate Prometheus metrics
            output = []
            output.append(
                "# HELP rapl_power_watts Current power consumption in watts from RAPL"
            )
            output.append("# TYPE rapl_power_watts gauge")

            for zone, watts in power_data.items():
                # Sanitize zone name for Prometheus
                safe_zone = zone.lower().replace("-", "_").replace(" ", "_")
                output.append(f'rapl_power_watts{{zone="{safe_zone}"}} {watts:.4f}')

            self.wfile.write("\n".join(output).encode("utf-8"))
            self.wfile.write(b"\n")
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):
        # Suppress default logging
        pass


def main():
    port = int(os.getenv("RAPL_EXPORTER_PORT", "9500"))

    print(f"Starting RAPL Power Exporter on port {port}")

    # Initialize RAPL reader
    rapl_reader = RAPLReader()

    if not rapl_reader.zones:
        print("No RAPL zones found. Exiting.")
        return

    print(f"Discovered RAPL zones: {list(rapl_reader.zones.keys())}")

    # Initial reading to establish baseline
    time.sleep(1)
    rapl_reader.get_power_watts()

    # Set up HTTP server
    MetricsHandler.rapl_reader = rapl_reader
    server = HTTPServer(("0.0.0.0", port), MetricsHandler)

    print(f"Exporter ready at http://0.0.0.0:{port}/metrics")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...")
        server.shutdown()


if __name__ == "__main__":
    main()
