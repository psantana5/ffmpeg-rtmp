#!/usr/bin/env python3
"""
Validate Results - Validate test results files for completeness and correctness.

Checks test results JSON files for:
- Required fields
- Data consistency
- Power measurement validity
- Timestamp ordering
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


def validate_scenario(scenario: dict, index: int) -> list:
    """
    Validate a single scenario.
    
    Returns:
        List of validation error messages
    """
    errors = []
    
    # Check required fields
    required_fields = ['name', 'start_time', 'end_time']
    for field in required_fields:
        if field not in scenario:
            errors.append(f"Scenario {index}: Missing required field '{field}'")
    
    # Check timestamps
    if 'start_time' in scenario and 'end_time' in scenario:
        start = scenario['start_time']
        end = scenario['end_time']
        
        if start is None or end is None:
            errors.append(f"Scenario {index} ('{scenario.get('name', 'Unknown')}'): Null timestamps")
        elif start >= end:
            errors.append(f"Scenario {index} ('{scenario.get('name', 'Unknown')}'): start_time >= end_time")
    
    # Check power data
    if 'power' in scenario:
        power = scenario['power']
        if not isinstance(power, dict):
            errors.append(f"Scenario {index}: 'power' should be a dict")
        else:
            if 'mean_watts' not in power:
                errors.append(f"Scenario {index}: Missing 'mean_watts' in power data")
            elif power['mean_watts'] is not None:
                mean_watts = power['mean_watts']
                if mean_watts < 0:
                    errors.append(f"Scenario {index}: Negative power value ({mean_watts})")
                elif mean_watts > 1000:
                    errors.append(f"Scenario {index}: Suspiciously high power value ({mean_watts} W)")
    
    return errors


def validate_results_file(filepath: Path, json_output: bool = False) -> tuple:
    """
    Validate a test results file.
    
    Returns:
        Tuple of (is_valid, errors_list)
    """
    if not filepath.exists():
        return (False, [f"File not found: {filepath}"])
    
    errors = []
    
    # Load JSON
    try:
        with open(filepath, 'r') as f:
            data = json.load(f)
    except json.JSONDecodeError as e:
        return (False, [f"Invalid JSON: {e}"])
    except Exception as e:
        return (False, [f"Error reading file: {e}"])
    
    # Check top-level structure
    if not isinstance(data, dict):
        errors.append("Root element should be a dict")
        return (False, errors)
    
    if 'scenarios' not in data:
        errors.append("Missing 'scenarios' field")
        return (False, errors)
    
    scenarios = data['scenarios']
    if not isinstance(scenarios, list):
        errors.append("'scenarios' should be a list")
        return (False, errors)
    
    if len(scenarios) == 0:
        errors.append("No scenarios in results file")
        return (False, errors)
    
    # Validate each scenario
    for i, scenario in enumerate(scenarios):
        scenario_errors = validate_scenario(scenario, i)
        errors.extend(scenario_errors)
    
    is_valid = len(errors) == 0
    return (is_valid, errors)


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='Validate test results files',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Validate latest results
  python3 validate_results.py
  
  # Validate specific file
  python3 validate_results.py --file test_results/test_results_20240101_120000.json
  
  # Validate all files in directory
  python3 validate_results.py --all
  
  # JSON output
  python3 validate_results.py --json
        """
    )
    
    parser.add_argument(
        '--file',
        type=Path,
        help='Specific results file to validate'
    )
    
    parser.add_argument(
        '--results-dir',
        type=Path,
        default=Path('test_results'),
        help='Directory containing test results (default: test_results)'
    )
    
    parser.add_argument(
        '--all',
        action='store_true',
        help='Validate all files in results directory'
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
    
    # Determine files to validate
    files_to_validate = []
    
    if args.file:
        files_to_validate = [args.file]
    elif args.all:
        if not args.results_dir.exists():
            logger.error(f"Results directory not found: {args.results_dir}")
            sys.exit(1)
        files_to_validate = sorted(args.results_dir.glob('test_results_*.json'))
    else:
        # Latest file
        if not args.results_dir.exists():
            logger.error(f"Results directory not found: {args.results_dir}")
            sys.exit(1)
        files = sorted(args.results_dir.glob('test_results_*.json'), reverse=True)
        if files:
            files_to_validate = [files[0]]
    
    if not files_to_validate:
        logger.error("No test result files found")
        sys.exit(1)
    
    # Validate files
    all_results = []
    total_valid = 0
    total_invalid = 0
    
    for filepath in files_to_validate:
        is_valid, errors = validate_results_file(filepath, args.json)
        
        if is_valid:
            total_valid += 1
        else:
            total_invalid += 1
        
        result = {
            'file': str(filepath),
            'valid': is_valid,
            'errors': errors
        }
        all_results.append(result)
        
        if not args.json:
            status = '✓ VALID' if is_valid else '✗ INVALID'
            print(f"\n{filepath.name}: {status}")
            
            if errors:
                for error in errors:
                    print(f"  - {error}")
    
    if args.json:
        print(json.dumps({
            'results': all_results,
            'summary': {
                'total': len(files_to_validate),
                'valid': total_valid,
                'invalid': total_invalid
            }
        }, indent=2))
    else:
        print(f"\n{'='*70}")
        print(f"Summary: {total_valid} valid, {total_invalid} invalid (of {len(files_to_validate)} total)")
        print(f"{'='*70}\n")
    
    # Exit with error code if any invalid
    sys.exit(0 if total_invalid == 0 else 1)


if __name__ == '__main__':
    main()
