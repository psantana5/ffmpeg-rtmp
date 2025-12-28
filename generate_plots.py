#!/usr/bin/env python3
"""
Generate professional plots from FFmpeg power monitoring test results.
Creates publication-ready figures for power consumption analysis.
"""

import argparse
import json
from pathlib import Path
from typing import Any, Dict

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd


def setup_plot_style():
    """Configure professional matplotlib style."""
    plt.style.use('seaborn-v0_8-whitegrid')
    plt.rcParams.update({
        'font.size': 10,
        'axes.labelsize': 12,
        'axes.titlesize': 14,
        'xtick.labelsize': 10,
        'ytick.labelsize': 10,
        'legend.fontsize': 10,
        'figure.titlesize': 16,
        'axes.grid': True,
        'grid.alpha': 0.3,
        'lines.linewidth': 2,
        'figure.dpi': 100,
        'savefig.dpi': 300,
        'savefig.bbox': 'tight',
        'savefig.pad_inches': 0.1
    })


def load_test_results(json_path: Path) -> Dict[str, Any]:
    """Load test results from JSON file."""
    with open(json_path, 'r') as f:
        return json.load(f)


def load_analysis_csv(csv_path: Path) -> pd.DataFrame:
    """Load analysis results from CSV file."""
    return pd.read_csv(csv_path)


def parse_bitrate(bitrate_str: str) -> float:
    """Parse bitrate string to Mbps."""
    if not isinstance(bitrate_str, str):
        return 0.0
    s = bitrate_str.strip().lower()
    if s.endswith('k'):
        return float(s[:-1]) / 1000.0
    elif s.endswith('m'):
        return float(s[:-1])
    elif s.endswith('kbps'):
        return float(s[:-4]) / 1000.0
    elif s.endswith('mbps'):
        return float(s[:-4])
    else:
        # Assume kbps if reasonable
        val = float(s)
        return val / 1000.0 if val > 1000 else val


def plot_power_vs_bitrate(df: pd.DataFrame, output_dir: Path):
    """Create power vs bitrate scatter plot."""
    fig, ax = plt.subplots(figsize=(10, 6))
    
    # Filter out baseline and extract bitrate/power data
    test_df = df[~df['name'].str.contains('baseline', case=False)].copy()
    test_df['bitrate_mbps'] = test_df['bitrate'].apply(parse_bitrate)
    
    # Create scatter plot
    scatter = ax.scatter(test_df['bitrate_mbps'], test_df['mean_power_w'], 
                        c=test_df['mean_power_w'], cmap='viridis', 
                        s=100, alpha=0.7, edgecolors='black', linewidth=1)
    
    # Add trend line
    if len(test_df) > 1:
        z = np.polyfit(test_df['bitrate_mbps'], test_df['mean_power_w'], 1)
        p = np.poly1d(z)
        ax.plot(test_df['bitrate_mbps'], p(test_df['bitrate_mbps']), 
                "r--", alpha=0.8, linewidth=2, label=f'Trend: y={z[0]:.2f}x+{z[1]:.1f}')
    
    # Add baseline reference
    baseline_df = df[df['name'].str.contains('baseline', case=False)]
    if not baseline_df.empty:
        baseline_power = baseline_df['mean_power_w'].iloc[0]
        ax.axhline(y=baseline_power, color='red', linestyle=':', alpha=0.7, 
                  label=f'Baseline: {baseline_power:.1f}W')
    
    ax.set_xlabel('Bitrate (Mbps)')
    ax.set_ylabel('Mean Power Consumption (Watts)')
    ax.set_title('Power Consumption vs Streaming Bitrate')
    ax.legend()
    ax.grid(True, alpha=0.3)
    
    # Add colorbar
    cbar = plt.colorbar(scatter, ax=ax)
    cbar.set_label('Power (W)')
    
    plt.savefig(output_dir / 'power_vs_bitrate.png')
    plt.close()


def plot_power_by_resolution(df: pd.DataFrame, output_dir: Path):
    """Create power comparison by resolution."""
    fig, ax = plt.subplots(figsize=(10, 6))
    
    # Filter resolution tests (same bitrate, different resolutions)
    res_tests = df[df['name'].str.contains(r'\d+p', regex=True)].copy()
    
    if not res_tests.empty:
        # Extract resolution from name
        res_tests['resolution'] = res_tests['name'].str.extract(r'(\d+p)')[0]
        res_tests = res_tests.dropna(subset=['resolution'])
        
        # Group by resolution
        res_groups = res_tests.groupby('resolution')['mean_power_w'].mean()
        
        bars = ax.bar(res_groups.index, res_groups.values, 
                     color=['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728'], 
                     alpha=0.8, edgecolor='black', linewidth=1)
        
        # Add value labels on bars
        for bar, value in zip(bars, res_groups.values):
            height = bar.get_height()
            ax.text(bar.get_x() + bar.get_width()/2., height + 0.5,
                   f'{value:.1f}W', ha='center', va='bottom')
        
        ax.set_xlabel('Resolution')
        ax.set_ylabel('Mean Power Consumption (Watts)')
        ax.set_title('Power Consumption by Resolution (2.5 Mbps)')
        ax.grid(True, alpha=0.3, axis='y')
    
    plt.savefig(output_dir / 'power_by_resolution.png')
    plt.close()


def plot_power_by_streams(df: pd.DataFrame, output_dir: Path):
    """Create power vs concurrent streams plot."""
    fig, ax = plt.subplots(figsize=(10, 6))
    
    # Filter multi-stream tests
    stream_tests = df[df['name'].str.contains(r'\d+ Streams', regex=True)].copy()
    
    if not stream_tests.empty:
        # Extract number of streams
        stream_tests['num_streams'] = stream_tests['name'].str.extract(r'(\d+) Streams').astype(int)
        stream_tests = stream_tests.sort_values('num_streams')
        
        # Create bar plot
        bars = ax.bar(stream_tests['num_streams'], stream_tests['mean_power_w'],
                     color='#1f77b4', alpha=0.8, edgecolor='black', linewidth=1)
        
        # Add value labels
        for bar, value in zip(bars, stream_tests['mean_power_w']):
            height = bar.get_height()
            ax.text(bar.get_x() + bar.get_width()/2., height + 0.5,
                   f'{value:.1f}W', ha='center', va='bottom')
        
        # Add per-stream efficiency line
        baseline_df = df[df['name'].str.contains('baseline', case=False)]
        if not baseline_df.empty and not stream_tests.empty:
            baseline_power = baseline_df['mean_power_w'].iloc[0]
            per_stream = [(p - baseline_power) / n for p, n in 
                         zip(stream_tests['mean_power_w'], stream_tests['num_streams'])]
            
            ax2 = ax.twinx()
            ax2.plot(stream_tests['num_streams'], per_stream, 'ro-', 
                    linewidth=2, markersize=8, label='Per-stream overhead')
            ax2.set_ylabel('Per-stream Power (W)', color='red')
            ax2.tick_params(axis='y', labelcolor='red')
        
        ax.set_xlabel('Number of Concurrent Streams')
        ax.set_ylabel('Mean Power Consumption (Watts)')
        ax.set_title('Power Consumption vs Concurrent Streams')
        ax.grid(True, alpha=0.3, axis='y')
        ax.set_xticks(stream_tests['num_streams'])
    
    plt.savefig(output_dir / 'power_by_streams.png')
    plt.close()


def plot_efficiency_comparison(df: pd.DataFrame, output_dir: Path):
    """Create energy efficiency comparison (Wh per Mbps)."""
    fig, ax = plt.subplots(figsize=(12, 8))
    
    # Filter out baseline
    test_df = df[~df['name'].str.contains('baseline', case=False)].copy()
    
    if not test_df.empty:
        # Calculate metrics
        test_df['bitrate_mbps'] = test_df['bitrate'].apply(parse_bitrate)
        test_df['duration_hours'] = test_df['duration'] / 3600
        test_df['energy_wh'] = test_df['total_energy_wh']
        test_df['wh_per_mbps'] = test_df.apply(
            lambda row: row['energy_wh'] / row['bitrate_mbps'] if row['bitrate_mbps'] > 0 else np.nan, 
            axis=1
        )
        test_df['wh_per_hour'] = test_df['total_energy_wh'] / test_df['duration_hours']
        
        # Create subplot layout
        fig, ((ax1, ax2), (ax3, ax4)) = plt.subplots(2, 2, figsize=(15, 12))
        
        # Plot 1: Energy per bitrate
        valid_data = test_df.dropna(subset=['wh_per_mbps'])
        if not valid_data.empty:
            bars1 = ax1.bar(range(len(valid_data)), valid_data['wh_per_mbps'],
                           color='#2ca02c', alpha=0.8, edgecolor='black')
            ax1.set_xticks(range(len(valid_data)))
            ax1.set_xticklabels(valid_data['name'], rotation=45, ha='right')
            ax1.set_ylabel('Energy per Mbps (Wh/Mbps)')
            ax1.set_title('Energy Efficiency by Bitrate')
            ax1.grid(True, alpha=0.3, axis='y')
        
        # Plot 2: Power vs bitrate scatter
        ax2.scatter(valid_data['bitrate_mbps'], valid_data['mean_power_w'],
                   c=valid_data['wh_per_mbps'], cmap='plasma', s=100, alpha=0.7)
        ax2.set_xlabel('Bitrate (Mbps)')
        ax2.set_ylabel('Power (W)')
        ax2.set_title('Power vs Bitrate (colored by efficiency)')
        
        # Plot 3: Energy per hour
        bars3 = ax3.bar(range(len(valid_data)), valid_data['wh_per_hour'],
                       color='#ff7f0e', alpha=0.8, edgecolor='black')
        ax3.set_xticks(range(len(valid_data)))
        ax3.set_xticklabels(valid_data['name'], rotation=45, ha='right')
        ax3.set_ylabel('Energy per Hour (Wh)')
        ax3.set_title('Energy Consumption Rate')
        ax3.grid(True, alpha=0.3, axis='y')
        
        # Plot 4: Efficiency summary
        metrics = ['Mean Power (W)', 'Energy/Mbps (Wh)', 'Energy/Hour (Wh)']
        values = [
            valid_data['mean_power_w'].mean(),
            valid_data['wh_per_mbps'].mean(),
            valid_data['wh_per_hour'].mean()
        ]
        bars4 = ax4.bar(metrics, values, color=['#1f77b4', '#2ca02c', '#ff7f0e'],
                       alpha=0.8, edgecolor='black')
        ax4.set_ylabel('Average Value')
        ax4.set_title('Overall Efficiency Metrics')
        ax4.grid(True, alpha=0.3, axis='y')
        
        # Add value labels on bars
        for ax, bars in [(ax1, bars1), (ax3, bars3), (ax4, bars4)]:
            for bar, value in zip(bars, values if ax == ax4 else bars):
                height = bar.get_height()
                ax.text(bar.get_x() + bar.get_width()/2., height + (height * 0.01),
                       f'{height:.3f}', ha='center', va='bottom')
    
    plt.tight_layout()
    plt.savefig(output_dir / 'efficiency_comparison.png')
    plt.close()


def create_summary_dashboard(df: pd.DataFrame, output_dir: Path):
    """Create a comprehensive dashboard with multiple subplots."""
    fig = plt.figure(figsize=(16, 12))
    
    # Create grid layout
    gs = fig.add_gridspec(3, 3, hspace=0.3, wspace=0.3)
    
    # 1. Power timeline
    ax1 = fig.add_subplot(gs[0, :])
    test_df = df[~df['name'].str.contains('baseline', case=False)].copy()
    if not test_df.empty:
        ax1.plot(range(len(test_df)), test_df['mean_power_w'], 'o-', 
                linewidth=2, markersize=8, color='#1f77b4')
        ax1.set_title('Power Consumption Timeline')
        ax1.set_ylabel('Power (W)')
        ax1.grid(True, alpha=0.3)
        ax1.set_xticks(range(len(test_df)))
        ax1.set_xticklabels(test_df['name'], rotation=45, ha='right')
    
    # 2. Power distribution
    ax2 = fig.add_subplot(gs[1, 0])
    ax2.hist(df['mean_power_w'], bins=10, alpha=0.7, color='#2ca02c', edgecolor='black')
    ax2.set_title('Power Distribution')
    ax2.set_xlabel('Power (W)')
    ax2.set_ylabel('Frequency')
    ax2.grid(True, alpha=0.3)
    
    # 3. Duration vs Power
    ax3 = fig.add_subplot(gs[1, 1])
    ax3.scatter(df['duration'], df['mean_power_w'], alpha=0.7, 
               c=df['mean_power_w'], cmap='viridis', s=100)
    ax3.set_title('Duration vs Power')
    ax3.set_xlabel('Duration (s)')
    ax3.set_ylabel('Power (W)')
    ax3.grid(True, alpha=0.3)
    
    # 4. Energy consumption
    ax4 = fig.add_subplot(gs[1, 2])
    energy_data = df.dropna(subset=['total_energy_wh'])
    if not energy_data.empty:
        bars = ax4.bar(range(len(energy_data)), energy_data['total_energy_wh'],
                      alpha=0.8, edgecolor='black')
        ax4.set_title('Total Energy Consumption')
        ax4.set_ylabel('Energy (Wh)')
        ax4.set_xticks(range(len(energy_data)))
        ax4.set_xticklabels(energy_data['name'], rotation=45, ha='right')
        ax4.grid(True, alpha=0.3, axis='y')
    
    # 5. Baseline comparison
    ax5 = fig.add_subplot(gs[2, :2])
    baseline_df = df[df['name'].str.contains('baseline', case=False)]
    if not baseline_df.empty:
        baseline_power = baseline_df['mean_power_w'].iloc[0]
        test_df = df[~df['name'].str.contains('baseline', case=False)].copy()
        test_df['power_above_baseline'] = test_df['mean_power_w'] - baseline_power
        
        bars = ax5.bar(range(len(test_df)), test_df['power_above_baseline'],
                      color='#d62728', alpha=0.8, edgecolor='black')
        ax5.axhline(y=0, color='black', linestyle='-', alpha=0.5)
        ax5.set_title(f'Power Above Baseline (Baseline: {baseline_power:.1f}W)')
        ax5.set_ylabel('Power Above Baseline (W)')
        ax5.set_xticks(range(len(test_df)))
        ax5.set_xticklabels(test_df['name'], rotation=45, ha='right')
        ax5.grid(True, alpha=0.3, axis='y')
    
    # 6. Summary statistics
    ax6 = fig.add_subplot(gs[2, 2])
    ax6.axis('off')
    
    # Calculate statistics
    stats_text = f"""
    Summary Statistics
    
    Tests: {len(df)}
    Baseline Power: {baseline_df['mean_power_w'].iloc[0]:.2f} W
    Avg Test Power: {df['mean_power_w'].mean():.2f} W
    Max Power: {df['mean_power_w'].max():.2f} W
    Min Power: {df['mean_power_w'].min():.2f} W
    Std Dev: {df['mean_power_w'].std():.2f} W
    
    Total Energy: {df['total_energy_wh'].sum():.3f} Wh
    Avg Duration: {df['duration'].mean():.1f} s
    """
    
    ax6.text(0.1, 0.9, stats_text, transform=ax6.transAxes, fontsize=11,
            verticalalignment='top', fontfamily='monospace',
            bbox=dict(boxstyle='round', facecolor='lightgray', alpha=0.8))
    
    plt.suptitle('FFmpeg Power Monitoring Analysis Dashboard', fontsize=18, y=0.98)
    plt.savefig(output_dir / 'analysis_dashboard.png')
    plt.close()


def main():
    parser = argparse.ArgumentParser(description='Generate plots from FFmpeg power monitoring results')
    parser.add_argument('--input', '-i', type=str, required=True,
                       help='Path to test results JSON file')
    parser.add_argument('--csv', '-c', type=str,
                       help='Path to analysis CSV file (optional)')
    parser.add_argument('--output', '-o', type=str, default='plots',
                       help='Output directory for plots')
    
    args = parser.parse_args()
    
    # Setup
    setup_plot_style()
    input_path = Path(args.input)
    output_dir = Path(args.output)
    output_dir.mkdir(exist_ok=True)
    
    # Load data
    test_data = load_test_results(input_path)
    
    # Try to load CSV analysis
    csv_path = None
    if args.csv:
        csv_path = Path(args.csv)
    else:
        # Look for corresponding CSV file
        csv_pattern = input_path.stem.replace('test_results_', '') + '_analysis.csv'
        csv_path = input_path.parent / csv_pattern
        if not csv_path.exists():
            print(f"Warning: No CSV analysis file found at {csv_path}")
            csv_path = None
    
    if csv_path and csv_path.exists():
        df = load_analysis_csv(csv_path)
        print(f"Loaded CSV analysis with {len(df)} scenarios")
        
        # Generate plots
        plot_power_vs_bitrate(df, output_dir)
        plot_power_by_resolution(df, output_dir)
        plot_power_by_streams(df, output_dir)
        plot_efficiency_comparison(df, output_dir)
        create_summary_dashboard(df, output_dir)
        
        print(f"Generated plots in {output_dir}/")
        print("  - power_vs_bitrate.png")
        print("  - power_by_resolution.png") 
        print("  - power_by_streams.png")
        print("  - efficiency_comparison.png")
        print("  - analysis_dashboard.png")
    else:
        print("No CSV analysis data available. Using JSON metadata only.")
        # Could add basic plots from JSON metadata here


if __name__ == '__main__':
    main()
