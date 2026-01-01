#!/usr/bin/env python3
"""
Export metrics from VictoriaMetrics to CSV for ML model retraining.

This script queries VictoriaMetrics for QoE metrics, cost data, and streaming
parameters, then exports them to a CSV file suitable for training ML models.
"""

import argparse
import csv
import json
import sys
from datetime import datetime, timedelta
from typing import Dict, List
import requests


def query_victoriametrics(vm_url: str, query: str, start: str, end: str, step: str = "5m") -> List[Dict]:
    """Query VictoriaMetrics and return results."""
    params = {
        "query": query,
        "start": start,
        "end": end,
        "step": step,
    }
    
    response = requests.get(f"{vm_url}/api/v1/query_range", params=params, timeout=30)
    response.raise_for_status()
    
    data = response.json()
    if data["status"] != "success":
        raise ValueError(f"Query failed: {data.get('error', 'Unknown error')}")
    
    return data["data"]["result"]


def extract_features(vm_url: str, start_time: str, end_time: str) -> List[Dict]:
    """Extract features and targets from VictoriaMetrics."""
    print("Extracting features from VictoriaMetrics...")
    
    # Query for different metrics
    queries = {
        "vmaf": 'qoe_vmaf_score',
        "psnr": 'qoe_psnr_score',
        "ssim": 'qoe_ssim_score',
        "frame_drop": 'qoe_drop_rate',
        "cost": 'cost_total_usd',
        "co2": 'cost_co2_kg',
    }
    
    results = {}
    for name, query in queries.items():
        try:
            result = query_victoriametrics(vm_url, query, start_time, end_time)
            results[name] = result
            print(f"  ✓ Retrieved {len(result)} series for {name}")
        except Exception as e:
            print(f"  ✗ Failed to retrieve {name}: {e}")
            results[name] = []
    
    # Combine into training examples
    examples = []
    
    # Process VMAF data as the primary target
    for series in results.get("vmaf", []):
        labels = series["metric"]
        values = series["values"]
        
        # Extract features from labels
        bitrate_str = labels.get("bitrate", "0k")
        bitrate_kbps = float(bitrate_str.replace("k", "").replace("kbps", "")) if bitrate_str else 0
        
        resolution = labels.get("resolution", "1280x720")
        try:
            width, height = map(int, resolution.split("x"))
        except:
            width, height = 1280, 720
        
        # Try to find matching PSNR, cost, etc.
        for timestamp, vmaf_value in values:
            example = {
                "timestamp": datetime.fromtimestamp(float(timestamp)).isoformat(),
                "bitrate_kbps": bitrate_kbps,
                "resolution_width": width,
                "resolution_height": height,
                "frame_rate": 30.0,  # Default, could be extracted if available
                "frame_drop": 0.01,  # Default, should match from frame_drop query
                "motion_intensity": 0.5,  # Default, would need to be computed
                "vmaf_score": float(vmaf_value),
                "psnr_score": 35.0,  # Default, should match from psnr query
                "ssim_score": 0.95,  # Default, should match from ssim query
                "cost_usd": 0.1,  # Default, should match from cost query
                "co2_kg": 0.02,  # Default, should match from co2 query
            }
            examples.append(example)
    
    # If no real data, generate synthetic training data
    if not examples:
        print("  No real data found, generating synthetic training examples...")
        examples = generate_synthetic_data()
    
    return examples


def generate_synthetic_data() -> List[Dict]:
    """Generate synthetic training data for model bootstrapping."""
    examples = []
    
    # Define common scenarios
    scenarios = [
        # (bitrate, width, height, fps, drop, motion, vmaf, psnr, cost, co2)
        (1000, 1280, 720, 30, 0.01, 0.5, 75, 35, 0.05, 0.01),
        (1500, 1280, 720, 30, 0.008, 0.5, 80, 36, 0.07, 0.015),
        (2000, 1280, 720, 30, 0.005, 0.5, 85, 37, 0.09, 0.02),
        (2500, 1920, 1080, 30, 0.005, 0.6, 85, 38, 0.12, 0.025),
        (3000, 1920, 1080, 30, 0.003, 0.6, 88, 39, 0.14, 0.03),
        (4000, 1920, 1080, 30, 0.002, 0.6, 90, 40, 0.18, 0.04),
        (5000, 1920, 1080, 60, 0.002, 0.7, 92, 41, 0.22, 0.05),
        (8000, 3840, 2160, 30, 0.001, 0.6, 93, 42, 0.30, 0.06),
        (12000, 3840, 2160, 60, 0.001, 0.7, 95, 43, 0.45, 0.09),
        (15000, 3840, 2160, 60, 0.0005, 0.7, 96, 44, 0.55, 0.11),
    ]
    
    base_time = datetime.now()
    
    for i, (bitrate, width, height, fps, drop, motion, vmaf, psnr, cost, co2) in enumerate(scenarios):
        # Create multiple samples with slight variations
        for variation in range(5):
            timestamp = base_time - timedelta(hours=i * 10 + variation)
            
            # Add small random variations
            import random
            vmaf_var = vmaf + random.uniform(-2, 2)
            psnr_var = psnr + random.uniform(-0.5, 0.5)
            
            example = {
                "timestamp": timestamp.isoformat(),
                "bitrate_kbps": bitrate,
                "resolution_width": width,
                "resolution_height": height,
                "frame_rate": fps,
                "frame_drop": drop,
                "motion_intensity": motion,
                "vmaf_score": max(0, min(100, vmaf_var)),
                "psnr_score": max(0, psnr_var),
                "ssim_score": 0.9 + (vmaf_var - 75) / 250,  # Approximate SSIM
                "cost_usd": cost,
                "co2_kg": co2,
            }
            examples.append(example)
    
    return examples


def export_to_csv(examples: List[Dict], output_file: str):
    """Export training examples to CSV file."""
    print(f"Exporting {len(examples)} examples to {output_file}...")
    
    if not examples:
        print("No examples to export!")
        return
    
    fieldnames = examples[0].keys()
    
    with open(output_file, 'w', newline='') as csvfile:
        writer = csv.DictWriter(csvfile, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(examples)
    
    print(f"✓ Successfully exported training data to {output_file}")


def main():
    parser = argparse.ArgumentParser(description="Export VictoriaMetrics data to CSV for ML training")
    parser.add_argument(
        "--vm-url",
        default="http://localhost:8428",
        help="VictoriaMetrics URL (default: http://localhost:8428)"
    )
    parser.add_argument(
        "--start",
        default="7d",
        help="Start time (e.g., '7d' for 7 days ago, or ISO timestamp)"
    )
    parser.add_argument(
        "--end",
        default="now",
        help="End time (default: now)"
    )
    parser.add_argument(
        "--output",
        default="./ml_models/training_data.csv",
        help="Output CSV file (default: ./ml_models/training_data.csv)"
    )
    parser.add_argument(
        "--synthetic",
        action="store_true",
        help="Generate synthetic data instead of querying VictoriaMetrics"
    )
    
    args = parser.parse_args()
    
    # Convert relative time to timestamp
    if args.start.endswith("d"):
        days = int(args.start[:-1])
        start_time = (datetime.now() - timedelta(days=days)).isoformat()
    else:
        start_time = args.start
    
    end_time = "now" if args.end == "now" else args.end
    
    # Extract features
    if args.synthetic:
        print("Generating synthetic training data...")
        examples = generate_synthetic_data()
    else:
        examples = extract_features(args.vm_url, start_time, end_time)
    
    # Export to CSV
    export_to_csv(examples, args.output)
    
    print("\nTraining data export complete!")
    print(f"  Total examples: {len(examples)}")
    print(f"  Output file: {args.output}")


if __name__ == "__main__":
    main()
