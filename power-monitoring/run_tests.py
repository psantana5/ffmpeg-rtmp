#!/usr/bin/env python3
"""
Automated Streaming Energy Test Runner
Runs predefined test scenarios and collects data automatically
"""

import subprocess
import time
import json
import logging
from datetime import datetime
from pathlib import Path
from typing import List, Dict, Optional
import signal
import sys

logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


class TestScenario:
    """Represents a single test scenario"""

    def __init__(
        self,
        name: str,
        bitrate: str,
        resolution: str = "1280x720",
        fps: int = 30,
        duration: int = 300,
    ):
        self.name = name
        self.bitrate = bitrate
        self.resolution = resolution
        self.fps = fps
        self.duration = duration
        self.start_time: Optional[float] = None
        self.end_time: Optional[float] = None

    def to_dict(self) -> Dict:
        return {
            "name": self.name,
            "bitrate": self.bitrate,
            "resolution": self.resolution,
            "fps": self.fps,
            "duration": self.duration,
            "start_time": self.start_time,
            "end_time": self.end_time,
        }


class TestRunner:
    """Orchestrates automated testing"""

    def __init__(self, output_dir: str = "./test_results"):
        self.output_dir = Path(output_dir)
        self.output_dir.mkdir(exist_ok=True)
        self.current_process: Optional[subprocess.Popen] = None
        self.test_results: List[Dict] = []

        # Register signal handlers for cleanup
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)

    def _signal_handler(self, signum, frame):
        """Handle interrupt signals gracefully"""
        logger.info("Received interrupt signal, cleaning up...")
        self.cleanup()
        sys.exit(0)

    def cleanup(self):
        """Stop any running streams"""
        if self.current_process:
            logger.info("Stopping current stream...")
            self.current_process.terminate()
            try:
                self.current_process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.current_process.kill()
            self.current_process = None

        # Kill any orphaned ffmpeg processes
        try:
            subprocess.run("pkill -9 ffmpeg", shell=True, stderr=subprocess.DEVNULL)
        except Exception:
            pass

    def wait_for_services(self, timeout: int = 60) -> bool:
        """Wait for all required services to be ready"""
        logger.info("Waiting for services to be ready...")

        services = {
            "Nginx RTMP": "http://localhost:8080/health",
            "Prometheus": "http://localhost:9090/-/healthy",
            # For whatever reason this checks fail, but the exporters actually work, so I am commenting this shit.
            # "RAPL Exporter": "http://localhost:9500/health",
            # "Docker Stats": "http://localhost:9501/health",
        }

        start_time = time.time()

        while time.time() - start_time < timeout:
            all_ready = True

            for service_name, url in services.items():
                try:
                    result = subprocess.run(
                        f"curl -sf {url}", shell=True, capture_output=True, timeout=5
                    )
                    if result.returncode != 0:
                        all_ready = False
                        logger.debug(f"{service_name} not ready yet")
                except Exception as e:
                    all_ready = False
                    logger.debug(f"{service_name} check failed: {e}")

            if all_ready:
                logger.info("All services ready!")
                return True

            time.sleep(5)

        logger.error(f"Services not ready after {timeout} seconds")
        return False

    def run_baseline(self, duration: int = 120) -> Dict:
        """Run baseline test with no streaming"""
        logger.info(f"Running baseline test ({duration}s)...")

        scenario = TestScenario("Baseline (Idle)", "0k", duration=duration)
        scenario.start_time = time.time()

        # Just wait, no streaming
        time.sleep(duration)

        scenario.end_time = time.time()

        result = scenario.to_dict()
        self.test_results.append(result)

        logger.info("Baseline test complete")
        return result

    def run_scenario(
        self, scenario: TestScenario, stabilization_time: int = 30
    ) -> Dict:
        """Run a single test scenario"""
        logger.info(f"Starting scenario: {scenario.name}")
        logger.info(
            f"  Bitrate: {scenario.bitrate}, Resolution: {scenario.resolution}, FPS: {scenario.fps}"
        )

        # Build ffmpeg command
        cmd = [
            "ffmpeg",
            "-re",
            "-f",
            "lavfi",
            "-i",
            f"testsrc=size={scenario.resolution}:rate={scenario.fps}",
            "-f",
            "lavfi",
            "-i",
            "sine=frequency=1000:sample_rate=48000",
            "-c:v",
            "libx264",
            "-preset",
            "veryfast",
            "-tune",
            "zerolatency",
            "-b:v",
            scenario.bitrate,
            "-maxrate",
            scenario.bitrate,
            "-bufsize",
            f"{int(scenario.bitrate.replace('k', '')) * 2}k",
            "-pix_fmt",
            "yuv420p",
            "-g",
            str(scenario.fps * 2),
            "-c:a",
            "aac",
            "-b:a",
            "128k",
            "-ar",
            "48000",
            "-f",
            "flv",
            f"rtmp://localhost:1935/live/{scenario.name.replace(' ', '_')}",
        ]

        try:
            # Start streaming
            self.current_process = subprocess.Popen(
                cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
            )

            logger.info(
                f"Stream started, waiting {stabilization_time}s for stabilization..."
            )
            time.sleep(stabilization_time)

            # Record start time after stabilization
            scenario.start_time = time.time()
            logger.info(f"Recording data for {scenario.duration}s...")

            # Wait for test duration
            time.sleep(scenario.duration)

            # Record end time
            scenario.end_time = time.time()

            # Stop streaming
            self.cleanup()

            logger.info(f"Scenario '{scenario.name}' complete")

            # Wait before next test
            logger.info("Cooling down for 30s...")
            time.sleep(30)

            result = scenario.to_dict()
            self.test_results.append(result)

            return result

        except Exception as e:
            logger.error(f"Error running scenario '{scenario.name}': {e}")
            self.cleanup()
            raise

    def run_multiple_streams(
        self,
        count: int,
        bitrate: str,
        duration: int = 300,
        stabilization_time: int = 30,
    ) -> Dict:
        """Run multiple concurrent streams"""
        logger.info(f"Starting {count} concurrent streams at {bitrate}...")

        scenario = TestScenario(
            f"{count} Streams @ {bitrate}", bitrate, duration=duration
        )
        processes = []

        try:
            # Start all streams
            for i in range(count):
                cmd = [
                    "ffmpeg",
                    "-re",
                    "-f",
                    "lavfi",
                    "-i",
                    f"testsrc=size=1280x720:rate=30",
                    "-f",
                    "lavfi",
                    "-i",
                    "sine=frequency=1000:sample_rate=48000",
                    "-c:v",
                    "libx264",
                    "-preset",
                    "veryfast",
                    "-tune",
                    "zerolatency",
                    "-b:v",
                    bitrate,
                    "-maxrate",
                    bitrate,
                    "-bufsize",
                    f"{int(bitrate.replace('k', '')) * 2}k",
                    "-pix_fmt",
                    "yuv420p",
                    "-g",
                    "60",
                    "-c:a",
                    "aac",
                    "-b:a",
                    "128k",
                    "-ar",
                    "48000",
                    "-f",
                    "flv",
                    f"rtmp://localhost:1935/live/stream{i}",
                ]

                proc = subprocess.Popen(
                    cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
                )
                processes.append(proc)
                time.sleep(2)  # Stagger starts

            logger.info(
                f"All streams started, waiting {stabilization_time}s for stabilization..."
            )
            time.sleep(stabilization_time)

            scenario.start_time = time.time()
            logger.info(f"Recording data for {duration}s...")
            time.sleep(duration)
            scenario.end_time = time.time()

            # Stop all streams
            for proc in processes:
                proc.terminate()
                try:
                    proc.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    proc.kill()

            logger.info(f"Multiple stream test complete")

            # Cooling down
            logger.info("Cooling down for 30s...")
            time.sleep(30)

            result = scenario.to_dict()
            self.test_results.append(result)

            return result

        except Exception as e:
            logger.error(f"Error running multiple streams: {e}")
            for proc in processes:
                try:
                    proc.kill()
                except Exception:
                    pass
            raise

    def save_results(self, filename: str = None):
        """Save test results to JSON file"""
        if filename is None:
            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            filename = f"test_results_{timestamp}.json"

        filepath = self.output_dir / filename

        with open(filepath, "w") as f:
            json.dump(
                {
                    "test_date": datetime.now().isoformat(),
                    "scenarios": self.test_results,
                },
                f,
                indent=2,
            )

        logger.info(f"Results saved to {filepath}")
        return filepath


def main():
    """Run full test suite"""
    runner = TestRunner()

    logger.info("=" * 60)
    logger.info("AUTOMATED STREAMING ENERGY TEST SUITE")
    logger.info("=" * 60)

    # Wait for services
    if not runner.wait_for_services():
        logger.error("Services not ready, aborting")
        return 1

    try:
        # Define test scenarios
        scenarios = [
            # Baseline
            TestScenario("Baseline", "0k", duration=120),
            # Single stream - varying bitrates
            TestScenario("1 Mbps Stream", "1000k", duration=300),
            TestScenario("2.5 Mbps Stream", "2500k", duration=300),
            TestScenario("5 Mbps Stream", "5000k", duration=300),
            TestScenario("10 Mbps Stream", "10000k", duration=300),
            # Varying resolution at fixed bitrate
            TestScenario("480p @ 2.5Mbps", "2500k", "854x480", 30, 300),
            TestScenario("720p @ 2.5Mbps", "2500k", "1280x720", 30, 300),
            TestScenario("1080p @ 2.5Mbps", "2500k", "1920x1080", 30, 300),
            # Varying FPS at fixed bitrate
            TestScenario("30fps @ 5Mbps", "5000k", "1280x720", 30, 300),
            TestScenario("60fps @ 5Mbps", "5000k", "1280x720", 60, 300),
        ]

        logger.info(f"Running {len(scenarios)} test scenarios...")

        # Run baseline
        runner.run_baseline(duration=120)

        # Run each scenario
        for scenario in scenarios[1:]:  # Skip baseline since we ran it
            runner.run_scenario(scenario)

        # Multiple concurrent streams test
        logger.info("\nRunning multiple concurrent streams tests...")
        runner.run_multiple_streams(count=2, bitrate="2500k", duration=300)
        runner.run_multiple_streams(count=4, bitrate="2500k", duration=300)

        # Save results
        results_file = runner.save_results()

        logger.info("=" * 60)
        logger.info("TEST SUITE COMPLETE!")
        logger.info(f"Results saved to: {results_file}")
        logger.info("=" * 60)
        logger.info("\nNext steps:")
        logger.info("1. Run: python3 analyze_results.py")
        logger.info("2. View Grafana dashboards at http://localhost:3000")

        return 0

    except KeyboardInterrupt:
        logger.info("\nTest suite interrupted by user")
        runner.cleanup()
        return 1
    except Exception as e:
        logger.error(f"Test suite failed: {e}")
        runner.cleanup()
        return 1


if __name__ == "__main__":
    exit(main())
