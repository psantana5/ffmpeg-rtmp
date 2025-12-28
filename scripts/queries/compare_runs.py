#!/usr/bin/env python3
"""
Compare Runs - Compare multiple test runs side by side.

Compares scenarios across different test runs to identify:
- Performance differences
- Power consumption changes
- Efficiency improvements/regressions
"""

import argparse
import json
import logging
import sys
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def load_results_file(filepath: Path) -> dict:
    """Load a test results file."""
    try:
        with open(filepath, 'r') as f:
            return json.load(f)
    except Exception as e:
        logger.error(f"Error loading {filepath}: {e}")
        return None


def find_matching_scenario(scenario_name: str, scenarios: list) -> dict:
    """Find a scenario by name in a list."""
    for scenario in scenarios:
        if scenario.get('name') == scenario_name:
            return scenario
    return None


def compare_scenarios(name: str, scenario1: dict, scenario2: dict) -> dict:
    """Compare two scenarios and return differences."""
    comparison = {
        'scenario': name,
        'file1': {},
        'file2': {},
        'differences': {}
    }
    
    # Compare power
    power1 = scenario1.get('power', {}) if scenario1 else {}
    power2 = scenario2.get('power', {}) if scenario2 else {}
    
    mean1 = power1.get('mean_watts')
    mean2 = power2.get('mean_watts')
    
    if mean1 is not None and mean2 is not None:
        diff = mean2 - mean1
        pct_change = (diff / mean1) * 100 if mean1 > 0 else 0
        
        comparison['file1']['power'] = mean1
        comparison['file2']['power'] = mean2
        comparison['differences']['power_diff'] = diff
        comparison['differences']['power_pct_change'] = pct_change
    
    # Compare efficiency
    eff1 = scenario1.get('efficiency_score') if scenario1 else None
    eff2 = scenario2.get('efficiency_score') if scenario2 else None
    
    if eff1 is not None and eff2 is not None:
        diff = eff2 - eff1
        pct_change = (diff / eff1) * 100 if eff1 > 0 else 0
        
        comparison['file1']['efficiency'] = eff1
        comparison['file2']['efficiency'] = eff2
        comparison['differences']['efficiency_diff'] = diff
        comparison['differences']['efficiency_pct_change'] = pct_change
    
    return comparison


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='Compare multiple test runs',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Compare two specific runs
  python3 compare_runs.py --file1 test_results/test_results_20240101_120000.json \\
                          --file2 test_results/test_results_20240102_120000.json
  
  # Compare latest two runs
  python3 compare_runs.py --latest 2
  
  # JSON output
  python3 compare_runs.py --latest 2 --json
        """
    )
    
    parser.add_argument(
        '--file1',
        type=Path,
        help='First results file'
    )
    
    parser.add_argument(
        '--file2',
        type=Path,
        help='Second results file'
    )
    
    parser.add_argument(
        '--latest',
        type=int,
        help='Compare latest N runs (requires --results-dir)'
    )
    
    parser.add_argument(
        '--results-dir',
        type=Path,
        default=Path('test_results'),
        help='Directory containing test results (default: test_results)'
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
    
    # Determine files to compare
    if args.file1 and args.file2:
        files = [args.file1, args.file2]
    elif args.latest:
        if not args.results_dir.exists():
            logger.error(f"Results directory not found: {args.results_dir}")
            sys.exit(1)
        
        all_files = sorted(args.results_dir.glob('test_results_*.json'), reverse=True)
        if len(all_files) < args.latest:
            logger.error(f"Only {len(all_files)} results files found, need {args.latest}")
            sys.exit(1)
        
        files = all_files[:args.latest]
    else:
        logger.error("Specify either --file1/--file2 or --latest")
        sys.exit(1)
    
    # For now, just compare first two files
    if len(files) < 2:
        logger.error("Need at least 2 files to compare")
        sys.exit(1)
    
    file1, file2 = files[0], files[1]
    
    # Load files
    data1 = load_results_file(file1)
    data2 = load_results_file(file2)
    
    if data1 is None or data2 is None:
        logger.error("Failed to load results files")
        sys.exit(1)
    
    scenarios1 = data1.get('scenarios', [])
    scenarios2 = data2.get('scenarios', [])
    
    # Find common scenarios
    names1 = {s.get('name') for s in scenarios1}
    names2 = {s.get('name') for s in scenarios2}
    common_names = names1 & names2
    
    if not common_names:
        logger.warning("No common scenarios found between the two runs")
        sys.exit(0)
    
    # Compare scenarios
    comparisons = []
    
    for name in sorted(common_names):
        scenario1 = find_matching_scenario(name, scenarios1)
        scenario2 = find_matching_scenario(name, scenarios2)
        
        comparison = compare_scenarios(name, scenario1, scenario2)
        comparisons.append(comparison)
    
    # Output
    if args.json:
        print(json.dumps({
            'file1': str(file1),
            'file2': str(file2),
            'comparisons': comparisons
        }, indent=2))
    else:
        print(f"\n{'='*70}")
        print(f"RUN COMPARISON")
        print(f"{'='*70}")
        print(f"File 1: {file1.name}")
        print(f"File 2: {file2.name}")
        print(f"Common Scenarios: {len(comparisons)}")
        
        print(f"\n{'-'*70}")
        print(f"{'Scenario':<30s} {'Power Change':>15s} {'Efficiency Change':>20s}")
        print(f"{'-'*70}")
        
        for comp in comparisons:
            name = comp['scenario'][:30]
            
            diffs = comp.get('differences', {})
            
            power_change = ''
            if 'power_pct_change' in diffs:
                pct = diffs['power_pct_change']
                power_change = f"{pct:+.1f}%"
            
            eff_change = ''
            if 'efficiency_pct_change' in diffs:
                pct = diffs['efficiency_pct_change']
                eff_change = f"{pct:+.1f}%"
            
            print(f"{name:<30s} {power_change:>15s} {eff_change:>20s}")
        
        print(f"\n{'='*70}\n")


if __name__ == '__main__':
    main()
