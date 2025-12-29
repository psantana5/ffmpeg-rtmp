#!/usr/bin/env python3
"""
Automated Streaming Energy Test Runner
Runs predefined test scenarios and collects data automatically
"""

import argparse
import json
import logging
import signal
import subprocess
import sys
import time
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional

logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")
logger = logging.getLogger(__name__)

# Add parent directory to path for advisor imports
sys.path.insert(0, str(Path(__file__).parent.parent))

try:
    from advisor.quality.psnr import compute_psnr, is_psnr_available
    from advisor.quality.vmaf_integration import compute_vmaf, is_vmaf_available

    QUALITY_AVAILABLE = True
except ImportError:
    QUALITY_AVAILABLE = False
    logger.warning("Quality computation modules not available")


class TestScenario:
    """Represents a single test scenario"""

    def __init__(
        self,
        name: str,
        bitrate: str,
        resolution: str = "1280x720",
        fps: int = 30,
        duration: int = 300,
        outputs: Optional[List[Dict]] = None,
        encoder: str = "h264",
        preset: str = "veryfast",
    ):
        self.name = name
        self.bitrate = bitrate
        self.resolution = resolution
        self.fps = fps
        self.duration = duration
        self.outputs = outputs  # List of output ladder configs
        self.encoder = encoder
        self.preset = preset
        self.start_time: Optional[float] = None
        self.end_time: Optional[float] = None

    def to_dict(self) -> Dict:
        result = {
            "name": self.name,
            "bitrate": self.bitrate,
            "resolution": self.resolution,
            "fps": self.fps,
            "duration": self.duration,
            "encoder": self.encoder,
            "preset": self.preset,
            "start_time": self.start_time,
            "end_time": self.end_time,
            "outputs": self.outputs,  # Always include, even if None
        }
        # Add quality scores if they exist
        if hasattr(self, 'vmaf_score') and self.vmaf_score is not None:
            result['vmaf_score'] = self.vmaf_score
        if hasattr(self, 'psnr_score') and self.psnr_score is not None:
            result['psnr_score'] = self.psnr_score
        return result


class TestRunner:
    """Orchestrates automated testing"""

    def __init__(self, output_dir: str = "./test_results", compute_quality: bool = False):
        self.output_dir = Path(output_dir)
        self.output_dir.mkdir(exist_ok=True)
        self.current_process: Optional[subprocess.Popen] = None
        self.test_results: List[Dict] = []
        self.compute_quality = compute_quality

        # Create videos directory if quality computation is enabled
        if self.compute_quality:
            self.videos_dir = self.output_dir / "videos"
            self.videos_dir.mkdir(exist_ok=True)

            # Check quality computation availability
            if not QUALITY_AVAILABLE:
                logger.warning("Quality computation requested but modules not available")
                self.compute_quality = False
            else:
                vmaf_avail = is_vmaf_available()
                psnr_avail = is_psnr_available()
                logger.info(f"Quality computation enabled - VMAF: {vmaf_avail}, PSNR: {psnr_avail}")
                if not vmaf_avail and not psnr_avail:
                    logger.warning("Neither VMAF nor PSNR available, disabling quality computation")
                    self.compute_quality = False

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

    def _generate_reference_video(self, scenario: TestScenario, output_path: Path) -> bool:
        """
        Generate a reference video for quality comparison.

        Args:
            scenario: Test scenario with video parameters
            output_path: Path to save reference video

        Returns:
            True if successful, False otherwise
        """
        try:
            # Generate short reference video (10 seconds is enough for quality comparison)
            duration = min(10, scenario.duration)

            cmd = [
                'ffmpeg',
                '-f',
                'lavfi',
                '-i',
                f'testsrc=size={scenario.resolution}:rate={scenario.fps}',
                '-f',
                'lavfi',
                '-i',
                'sine=frequency=1000:sample_rate=48000',
                '-c:v',
                'libx264',
                '-preset',
                'medium',  # Better quality for reference
                '-crf',
                '18',  # High quality
                '-pix_fmt',
                'yuv420p',
                '-c:a',
                'aac',
                '-b:a',
                '128k',
                '-t',
                str(duration),
                '-y',  # Overwrite if exists
                str(output_path),
            ]

            logger.info(f"Generating reference video: {output_path.name}")
            result = subprocess.run(cmd, capture_output=True, timeout=60, check=False)

            if result.returncode != 0:
                logger.error(f"Failed to generate reference video: {result.stderr.decode()}")
                return False

            logger.info("Reference video generated successfully")
            return True

        except Exception as e:
            logger.error(f"Error generating reference video: {e}")
            return False

    def _record_output_video(
        self, scenario: TestScenario, stream_key: str, output_path: Path, duration: int = 10
    ) -> bool:
        """
        Record output video from RTMP stream for quality comparison.

        Args:
            scenario: Test scenario
            stream_key: RTMP stream key
            output_path: Path to save recorded video
            duration: Duration to record (seconds)

        Returns:
            True if successful, False otherwise
        """
        try:
            # Wait a bit for stream to stabilize
            time.sleep(2)

            cmd = [
                'ffmpeg',
                '-i',
                f'rtmp://localhost:1935/live/{stream_key}',
                '-c',
                'copy',
                '-t',
                str(duration),
                '-y',
                str(output_path),
            ]

            logger.info(f"Recording output video from stream: {stream_key}")
            result = subprocess.run(cmd, capture_output=True, timeout=duration + 30, check=False)

            if result.returncode != 0:
                logger.error(f"Failed to record output video: {result.stderr.decode()}")
                return False

            if not output_path.exists() or output_path.stat().st_size == 0:
                logger.error("Output video file is empty or missing")
                return False

            logger.info("Output video recorded successfully")
            return True

        except Exception as e:
            logger.error(f"Error recording output video: {e}")
            return False

    def _compute_quality_scores(
        self, reference_path: Path, output_path: Path, scenario: TestScenario
    ) -> None:
        """
        Compute VMAF and PSNR quality scores.

        Args:
            reference_path: Path to reference video
            output_path: Path to output video
            scenario: Test scenario to store scores in
        """
        if not QUALITY_AVAILABLE:
            return

        logger.info("Computing quality scores...")

        # Compute VMAF if available
        if is_vmaf_available():
            try:
                vmaf_score = compute_vmaf(str(reference_path), str(output_path))
                if vmaf_score is not None:
                    scenario.vmaf_score = vmaf_score
                    logger.info(f"VMAF score: {vmaf_score:.2f}")
                else:
                    logger.warning("VMAF computation returned None")
            except Exception as e:
                logger.error(f"Error computing VMAF: {e}")

        # Compute PSNR if available
        if is_psnr_available():
            try:
                psnr_score = compute_psnr(str(reference_path), str(output_path))
                if psnr_score is not None:
                    scenario.psnr_score = psnr_score
                    logger.info(f"PSNR score: {psnr_score:.2f} dB")
                else:
                    logger.warning("PSNR computation returned None")
            except Exception as e:
                logger.error(f"Error computing PSNR: {e}")

    def wait_for_services(self, timeout: int = 60) -> bool:
        """Wait for all required services to be ready"""
        logger.info("Waiting for services to be ready...")

        services = {
            "Nginx RTMP": "http://localhost:8080/health",
            "Prometheus": "http://localhost:9090/-/healthy",
            "RAPL Exporter": "http://localhost:9500/health",
            "Docker Stats": "http://localhost:9501/health",
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
        self,
        scenario: TestScenario,
        stabilization_time: int = 30,
        cooldown_time: int = 30,
    ) -> Dict:
        """Run a single test scenario"""
        logger.info(f"Starting scenario: {scenario.name}")
        logger.info(
            f"  Bitrate: {scenario.bitrate}, Resolution: {scenario.resolution}, "
            f"FPS: {scenario.fps}, Encoder: {scenario.encoder}, "
            f"Preset: {scenario.preset}"
        )

        # Build ffmpeg command
        stream_key = scenario.name.replace(" ", "_").replace("(", "").replace(")", "")
        cmd = build_ffmpeg_cmd(
            name=scenario.name,
            stream_key=stream_key,
            bitrate=scenario.bitrate,
            resolution=scenario.resolution,
            fps=scenario.fps,
            encoder=scenario.encoder,
            preset=scenario.preset,
        )

        try:
            # Generate reference video if quality computation is enabled
            reference_path = None
            output_path = None
            compute_quality_for_scenario = self.compute_quality

            if compute_quality_for_scenario:
                # Sanitize stream key for filesystem (handle special characters)
                safe_name = "".join(
                    c if c.isalnum() or c in ('-', '_') else '_' for c in stream_key
                )
                reference_path = self.videos_dir / f"reference_{safe_name}.mp4"
                output_path = self.videos_dir / f"output_{safe_name}.mp4"

                if not self._generate_reference_video(scenario, reference_path):
                    logger.warning(
                        "Failed to generate reference video, "
                        "skipping quality computation for this scenario"
                    )
                    compute_quality_for_scenario = False

            # Start streaming
            self.current_process = subprocess.Popen(
                cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
            )

            logger.info(f"Stream started, waiting {stabilization_time}s for stabilization...")
            time.sleep(stabilization_time)

            # Record output video if quality computation is enabled
            if compute_quality_for_scenario and reference_path and output_path:
                if not self._record_output_video(scenario, stream_key, output_path, duration=10):
                    logger.warning(
                        "Failed to record output video, "
                        "skipping quality computation for this scenario"
                    )
                    compute_quality_for_scenario = False
                    reference_path = None
                    output_path = None

            # Record start time after stabilization and recording
            scenario.start_time = time.time()
            logger.info(f"Recording data for {scenario.duration}s...")

            # Wait for test duration
            time.sleep(scenario.duration)

            # Record end time
            scenario.end_time = time.time()

            # Stop streaming
            self.cleanup()

            # Compute quality scores if videos were recorded
            if compute_quality_for_scenario and reference_path and output_path:
                if reference_path.exists() and output_path.exists():
                    self._compute_quality_scores(reference_path, output_path, scenario)
                else:
                    logger.warning(
                        "Reference or output video missing, skipping quality computation"
                    )

            logger.info(f"Scenario '{scenario.name}' complete")

            # Wait before next test
            logger.info(f"Cooling down for {cooldown_time}s...")
            time.sleep(cooldown_time)

            result = scenario.to_dict()
            self.test_results.append(result)

            return result

        except Exception as e:
            logger.error(f"Error running scenario '{scenario.name}': {e}")
            self.cleanup()
            raise
            self.cleanup()
            raise
            raise

    def run_streams_mixed(
        self,
        bitrates: List[str],
        duration: int = 300,
        stabilization_time: int = 30,
        resolution: str = "1280x720",
        fps: int = 30,
        cooldown_time: int = 30,
        name: str = "Mixed Streams",
    ) -> Dict:
        scenario = TestScenario(
            f"{len(bitrates)} Streams (Mixed)", ",".join(bitrates), duration=duration
        )
        processes = []

        try:
            for i, br in enumerate(bitrates):
                cmd = build_ffmpeg_cmd(
                    name=name,
                    stream_key=f"mix{i}",
                    bitrate=br,
                    resolution=resolution,
                    fps=fps,
                )
                proc = subprocess.Popen(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                processes.append(proc)
                time.sleep(2)

            logger.info(f"All streams started, waiting {stabilization_time}s for stabilization...")
            time.sleep(stabilization_time)

            scenario.start_time = time.time()
            logger.info(f"Recording data for {duration}s...")
            time.sleep(duration)
            scenario.end_time = time.time()

            for proc in processes:
                proc.terminate()
                try:
                    proc.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    proc.kill()

            logger.info("Mixed stream test complete")

            logger.info(f"Cooling down for {cooldown_time}s...")
            time.sleep(cooldown_time)

            result = scenario.to_dict()
            self.test_results.append(result)
            return result

        except Exception as e:
            logger.error(f"Error running mixed streams: {e}")
            for proc in processes:
                try:
                    proc.kill()
                except Exception:
                    pass
            raise

    def run_multiple_streams(
        self,
        count: int,
        bitrate: str,
        duration: int = 300,
        stabilization_time: int = 30,
        resolution: str = "1280x720",
        fps: int = 30,
        cooldown_time: int = 30,
    ) -> Dict:
        """Run multiple concurrent streams"""
        logger.info(f"Starting {count} concurrent streams at {bitrate}...")

        scenario = TestScenario(f"{count} Streams @ {bitrate}", bitrate, duration=duration)
        processes = []

        try:
            # Start all streams
            for i in range(count):
                cmd = build_ffmpeg_cmd(
                    name=scenario.name,
                    stream_key=f"stream{i}",
                    bitrate=bitrate,
                    resolution=resolution,
                    fps=fps,
                )

                proc = subprocess.Popen(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                processes.append(proc)
                time.sleep(2)  # Stagger starts

            logger.info(f"All streams started, waiting {stabilization_time}s for stabilization...")
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

            logger.info("Multiple stream test complete")

            # Cooling down
            logger.info(f"Cooling down for {cooldown_time}s...")
            time.sleep(cooldown_time)

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
    parser = argparse.ArgumentParser(prog="run_tests.py")
    parser.add_argument("--output-dir", default="./test_results")
    parser.add_argument("--wait-timeout", type=int, default=60)
    parser.add_argument("--skip-wait", action="store_true")
    parser.add_argument(
        "--compute-quality",
        action="store_true",
        help="Compute VMAF/PSNR quality scores (increases test duration)",
    )

    subparsers = parser.add_subparsers(dest="command")

    suite_p = subparsers.add_parser("suite")
    suite_p.add_argument("--baseline-duration", type=int, default=120)
    suite_p.add_argument("--scenario-duration", type=int, default=300)
    suite_p.add_argument("--stabilization", type=int, default=30)
    suite_p.add_argument("--cooldown", type=int, default=30)
    suite_p.add_argument(
        "--compute-quality",
        action="store_true",
        help="Compute VMAF/PSNR quality scores (increases test duration)",
    )

    single_p = subparsers.add_parser("single")
    single_p.add_argument("--name", required=True)
    single_p.add_argument("--bitrate", required=True)
    single_p.add_argument("--resolution", default="1280x720")
    single_p.add_argument("--fps", type=int, default=30)
    single_p.add_argument("--duration", type=int, default=300)
    single_p.add_argument("--stabilization", type=int, default=30)
    single_p.add_argument("--cooldown", type=int, default=30)
    single_p.add_argument(
        "--encoder",
        default="h264",
        choices=["h264", "h264_nvenc", "h265", "hevc_nvenc"],
        help="Video encoder to use (default: h264)",
    )
    single_p.add_argument(
        "--preset",
        default="veryfast",
        choices=["ultrafast", "veryfast", "fast", "medium", "slow", "slower"],
        help="Encoder preset (default: veryfast)",
    )
    single_p.add_argument("--with-baseline", action="store_true")
    single_p.add_argument("--baseline-duration", type=int, default=120)
    single_p.add_argument(
        "--compute-quality",
        action="store_true",
        help="Compute VMAF/PSNR quality scores (increases test duration)",
    )

    multi_p = subparsers.add_parser("multi")
    multi_p.add_argument("--count", type=int, required=True)
    multi_p.add_argument("--bitrate", default="2500k")
    multi_p.add_argument("--bitrates", default=None)
    multi_p.add_argument("--resolution", default="1280x720")
    multi_p.add_argument("--fps", type=int, default=30)
    multi_p.add_argument("--duration", type=int, default=300)
    multi_p.add_argument("--stabilization", type=int, default=30)
    multi_p.add_argument("--cooldown", type=int, default=30)
    multi_p.add_argument("--with-baseline", action="store_true")
    multi_p.add_argument("--baseline-duration", type=int, default=120)
    multi_p.add_argument(
        "--compute-quality",
        action="store_true",
        help="Compute VMAF/PSNR quality scores (increases test duration)",
    )

    batch_p = subparsers.add_parser("batch")
    batch_p.add_argument("--file", required=True)
    batch_p.add_argument("--stabilization", type=int, default=30)
    batch_p.add_argument("--cooldown", type=int, default=30)
    batch_p.add_argument(
        "--compute-quality",
        action="store_true",
        help="Compute VMAF/PSNR quality scores (increases test duration)",
    )

    args = parser.parse_args()

    runner = TestRunner(output_dir=args.output_dir, compute_quality=args.compute_quality)

    logger.info("=" * 60)
    logger.info("AUTOMATED STREAMING ENERGY TESTS")
    if args.compute_quality:
        logger.info("Quality scoring: ENABLED")
    logger.info("=" * 60)

    if not args.skip_wait:
        if not runner.wait_for_services(timeout=args.wait_timeout):
            logger.error("Services not ready, aborting")
            return 1

    try:
        if args.command is None:
            scenarios = build_default_suite(scenario_duration=300)
            logger.info(f"Running {len(scenarios)} test scenarios...")
            runner.run_baseline(duration=120)
            for scenario in scenarios[1:]:
                runner.run_scenario(scenario)
            logger.info("\nRunning multiple concurrent streams tests...")
            runner.run_multiple_streams(count=2, bitrate="2500k", duration=300)
            runner.run_multiple_streams(count=4, bitrate="2500k", duration=300)

        elif args.command == "suite":
            scenarios = build_default_suite(scenario_duration=args.scenario_duration)
            runner.run_baseline(duration=args.baseline_duration)
            for scenario in scenarios[1:]:
                runner.run_scenario(
                    scenario,
                    stabilization_time=args.stabilization,
                    cooldown_time=args.cooldown,
                )

        elif args.command == "single":
            if args.with_baseline:
                runner.run_baseline(duration=args.baseline_duration)
            scenario = TestScenario(
                args.name,
                args.bitrate,
                resolution=args.resolution,
                fps=args.fps,
                duration=args.duration,
                encoder=args.encoder,
                preset=args.preset,
            )
            runner.run_scenario(
                scenario,
                stabilization_time=args.stabilization,
                cooldown_time=args.cooldown,
            )

        elif args.command == "multi":
            if args.with_baseline:
                runner.run_baseline(duration=args.baseline_duration)
            if args.bitrates:
                bitrates = parse_csv_list(args.bitrates)
                if len(bitrates) != args.count:
                    raise ValueError(
                        f"Expected --bitrates to have {args.count} entries, got {len(bitrates)}"
                    )
                runner.run_streams_mixed(
                    bitrates=bitrates,
                    duration=args.duration,
                    stabilization_time=args.stabilization,
                    resolution=args.resolution,
                    fps=args.fps,
                    cooldown_time=args.cooldown,
                    name="Mixed Streams",
                )
            else:
                runner.run_multiple_streams(
                    count=args.count,
                    bitrate=args.bitrate,
                    duration=args.duration,
                    stabilization_time=args.stabilization,
                    resolution=args.resolution,
                    fps=args.fps,
                    cooldown_time=args.cooldown,
                )

        elif args.command == "batch":
            scenarios = load_batch_file(args.file)
            for entry in scenarios:
                entry_type = entry.get("type", "single")
                if entry_type == "baseline":
                    runner.run_baseline(duration=int(entry.get("duration", 120)))
                    continue

                if entry_type == "multi":
                    count = int(entry["count"])
                    duration = int(entry.get("duration", 300))
                    stabilization = int(entry.get("stabilization", args.stabilization))
                    cooldown = int(entry.get("cooldown", args.cooldown))
                    resolution = entry.get("resolution", "1280x720")
                    fps = int(entry.get("fps", 30))
                    if "bitrates" in entry:
                        bitrates = entry["bitrates"]
                        if len(bitrates) != count:
                            raise ValueError(
                                f"Batch scenario expects {count} bitrates, got {len(bitrates)}"
                            )
                        runner.run_streams_mixed(
                            bitrates=bitrates,
                            duration=duration,
                            stabilization_time=stabilization,
                            resolution=resolution,
                            fps=fps,
                            cooldown_time=cooldown,
                            name=entry.get("name", "Mixed Streams"),
                        )
                    else:
                        runner.run_multiple_streams(
                            count=count,
                            bitrate=entry.get("bitrate", "2500k"),
                            duration=duration,
                            stabilization_time=stabilization,
                            resolution=resolution,
                            fps=fps,
                            cooldown_time=cooldown,
                        )
                    continue

                scenario = TestScenario(
                    entry.get("name", "scenario"),
                    entry["bitrate"],
                    resolution=entry.get("resolution", "1280x720"),
                    fps=int(entry.get("fps", 30)),
                    duration=int(entry.get("duration", 300)),
                    outputs=entry.get("outputs"),  # Pass outputs if present
                    encoder=entry.get("encoder", "h264"),
                    preset=entry.get("preset", "veryfast"),
                )
                runner.run_scenario(
                    scenario,
                    stabilization_time=int(entry.get("stabilization", args.stabilization)),
                    cooldown_time=int(entry.get("cooldown", args.cooldown)),
                )

        results_file = runner.save_results()

        logger.info("=" * 60)
        logger.info("TESTS COMPLETE!")
        logger.info(f"Results saved to: {results_file}")
        logger.info("=" * 60)
        logger.info("\nNext steps:")
        logger.info("1. Run: python3 scripts/analyze_results.py")
        logger.info("2. View Grafana dashboards at http://localhost:3000")

        return 0

    except KeyboardInterrupt:
        logger.info("\nTests interrupted by user")
        runner.cleanup()
        return 1
    except Exception as e:
        logger.error(f"Tests failed: {e}")
        runner.cleanup()
        return 1


def parse_csv_list(value: str) -> List[str]:
    return [v.strip() for v in value.split(",") if v.strip()]


def build_ffmpeg_cmd(
    name: str,
    stream_key: str,
    bitrate: str,
    resolution: str,
    fps: int,
    encoder: str = "h264",
    preset: str = "veryfast",
) -> List[str]:
    """Build FFmpeg command with specified encoder and preset.
    
    Args:
        name: Test scenario name
        stream_key: RTMP stream key
        bitrate: Target video bitrate (e.g., "2500k")
        resolution: Video resolution (e.g., "1920x1080")
        fps: Frames per second
        encoder: Video encoder to use - "h264", "h264_nvenc", "h265", or "hevc_nvenc"
        preset: Encoder preset - "ultrafast", "veryfast", "fast", "medium", "slow", "slower"
    
    Returns:
        List of command arguments for FFmpeg
    """
    bufsize = f"{parse_bitrate_to_kbps(bitrate) * 2}k"
    
    # Map encoder names to FFmpeg codec names
    encoder_map = {
        "h264": "libx264",
        "h264_nvenc": "h264_nvenc",
        "h265": "libx265",
        "hevc_nvenc": "hevc_nvenc",
    }
    
    codec = encoder_map.get(encoder, "libx264")
    
    cmd = [
        "ffmpeg",
        "-re",
        "-f",
        "lavfi",
        "-i",
        f"testsrc=size={resolution}:rate={fps}",
        "-f",
        "lavfi",
        "-i",
        "sine=frequency=1000:sample_rate=48000",
        "-c:v",
        codec,
        "-preset",
        preset,
    ]
    
    # Add tune for CPU encoders only (GPU encoders don't support tune)
    # Use zerolatency for x264 (streaming), but skip for x265 to preserve quality
    if codec == "libx264":
        cmd.extend(["-tune", "zerolatency"])
    
    cmd.extend([
        "-b:v",
        bitrate,
        "-maxrate",
        bitrate,
        "-bufsize",
        bufsize,
        "-pix_fmt",
        "yuv420p",
        "-g",
        str(fps * 2),
        "-c:a",
        "aac",
        "-b:a",
        "128k",
        "-ar",
        "48000",
        "-f",
        "flv",
        f"rtmp://localhost:1935/live/{stream_key}",
    ])
    
    return cmd


def parse_bitrate_to_kbps(bitrate: str) -> int:
    value = bitrate.strip()
    if not value:
        raise ValueError("Bitrate cannot be empty")

    lower = value.lower()
    if lower.endswith("k"):
        return int(float(lower[:-1]))
    if lower.endswith("m"):
        return int(float(lower[:-1]) * 1000)

    if lower.isdigit():
        # Assume input is already in kbps when unit is omitted
        return int(lower)

    raise ValueError(f"Unsupported bitrate format: {bitrate}")


def build_default_suite(scenario_duration: int = 300) -> List[TestScenario]:
    return [
        TestScenario("Baseline", "0k", duration=120),
        TestScenario("1 Mbps Stream", "1000k", duration=scenario_duration),
        TestScenario("2.5 Mbps Stream", "2500k", duration=scenario_duration),
        TestScenario("5 Mbps Stream", "5000k", duration=scenario_duration),
        TestScenario("10 Mbps Stream", "10000k", duration=scenario_duration),
        TestScenario("480p @ 2.5Mbps", "2500k", "854x480", 30, scenario_duration),
        TestScenario("720p @ 2.5Mbps", "2500k", "1280x720", 30, scenario_duration),
        TestScenario("1080p @ 2.5Mbps", "2500k", "1920x1080", 30, scenario_duration),
        TestScenario("30fps @ 5Mbps", "5000k", "1280x720", 30, scenario_duration),
        TestScenario("60fps @ 5Mbps", "5000k", "1280x720", 60, scenario_duration),
    ]


def load_batch_file(path: str) -> List[Dict]:
    with open(path) as f:
        data = json.load(f)

    if isinstance(data, list):
        return data
    if isinstance(data, dict) and isinstance(data.get("scenarios"), list):
        return data["scenarios"]
    raise ValueError("Batch file must be a JSON list or a JSON object with a 'scenarios' list")


if __name__ == "__main__":
    exit(main())
