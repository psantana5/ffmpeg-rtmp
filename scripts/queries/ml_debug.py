#!/usr/bin/env python3
"""
ML Debug - Evaluate internal behavior of prediction models.

Performs detailed analysis:
- Per-sample residual analysis (measured vs predicted)
- Outlier detection with configurable thresholds
- Feature importance visualization (sorted list)
- Confidence interval width overview
- Optional: Deep dive into specific scenario
"""

import argparse
import json
import logging
import sys
from pathlib import Path
from typing import Dict, List, Optional

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from advisor.modeling import MultivariatePredictor, PowerPredictor
from scripts.utils.model_loader import ModelLoader

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def print_residual_analysis(
    residuals: List[tuple],
    outliers: List[tuple],
    json_output: bool = False
):
    """Print residual analysis results."""
    if json_output:
        output = {
            'residuals': [
                {
                    'scenario': name,
                    'measured': measured,
                    'predicted': predicted,
                    'residual': residual
                }
                for name, measured, predicted, residual in residuals
            ],
            'outliers': [
                {'scenario': name, 'residual': residual}
                for name, residual in outliers
            ]
        }
        print(json.dumps(output, indent=2))
        return
    
    # Human-readable output
    print(f"\n{'='*70}")
    print("RESIDUAL ANALYSIS (Measured - Predicted)")
    print(f"{'='*70}")
    
    if not residuals:
        print("\nNo residuals to analyze.")
        return
    
    print(f"\n{'Scenario':<40s} {'Measured':>10s} {'Predicted':>10s} {'Residual':>10s}")
    print(f"{'-'*70}")
    
    for name, measured, predicted, residual in residuals:
        status = ""
        if any(name == outlier_name for outlier_name, _ in outliers):
            status = " ⚠️ OUTLIER"
        
        print(f"{name[:40]:<40s} {measured:>10.2f} {predicted:>10.2f} {residual:>10.2f}{status}")
    
    if outliers:
        print(f"\n{'='*70}")
        print(f"OUTLIERS DETECTED: {len(outliers)}")
        print(f"{'='*70}")
        
        for name, residual in outliers:
            print(f"  {name}: residual = {residual:.2f} W")


def print_feature_importance(
    feature_importance: List[tuple],
    json_output: bool = False
):
    """Print feature importance results."""
    if json_output:
        output = {
            'feature_importance': [
                {'feature': name, 'importance': importance}
                for name, importance in feature_importance
            ]
        }
        print(json.dumps(output, indent=2))
        return
    
    # Human-readable output
    print(f"\n{'='*70}")
    print("FEATURE IMPORTANCE (Sorted by Impact)")
    print(f"{'='*70}")
    
    if not feature_importance:
        print("\nFeature importance not available for this model type.")
        return
    
    print(f"\n{'Feature':<40s} {'Importance':>15s}")
    print(f"{'-'*70}")
    
    for feature, importance in feature_importance:
        print(f"{feature:<40s} {importance:>15.6f}")


def print_scenario_deep_dive(
    scenario: Dict,
    model: object,
    loader: ModelLoader,
    json_output: bool = False
):
    """Print detailed analysis of a specific scenario."""
    if json_output:
        output = {
            'scenario_name': scenario.get('name'),
            'scenario_details': scenario
        }
        print(json.dumps(output, indent=2))
        return
    
    # Human-readable output
    name = scenario.get('name', 'Unknown')
    print(f"\n{'='*70}")
    print(f"SCENARIO DEEP DIVE: {name}")
    print(f"{'='*70}")
    
    # Basic info
    print(f"\nConfiguration:")
    print(f"  Bitrate:      {scenario.get('bitrate', 'N/A')}")
    print(f"  Resolution:   {scenario.get('resolution', 'N/A')}")
    print(f"  FPS:          {scenario.get('fps', 'N/A')}")
    print(f"  Duration:     {scenario.get('duration', 'N/A')} seconds")
    
    # Power measurements
    power = scenario.get('power', {})
    if power:
        print(f"\nPower Measurements:")
        print(f"  Mean:         {power.get('mean_watts', 'N/A')} W")
        print(f"  Min:          {power.get('min_watts', 'N/A')} W")
        print(f"  Max:          {power.get('max_watts', 'N/A')} W")
        print(f"  Stdev:        {power.get('stdev_watts', 'N/A')} W")
        print(f"  Total Energy: {power.get('total_energy_joules', 'N/A')} J")
    
    # Prediction
    try:
        predicted = None
        confidence = None
        
        if hasattr(model, '_infer_stream_count') and hasattr(model, 'predict'):
            # PowerPredictor
            stream_count = model._infer_stream_count(name)
            if stream_count is not None:
                predicted = model.predict(stream_count)
        
        elif hasattr(model, '_extract_features') and hasattr(model, 'predict'):
            # MultivariatePredictor
            features = model._extract_features(scenario)
            if features is not None:
                result = model.predict(features, return_confidence=True)
                predicted = result.get('mean')
                ci_low = result.get('ci_low')
                ci_high = result.get('ci_high')
                if ci_low is not None and ci_high is not None:
                    confidence = (ci_low, ci_high)
        
        if predicted is not None:
            print(f"\nPrediction:")
            print(f"  Predicted:    {predicted:.2f} W")
            
            measured = power.get('mean_watts')
            if measured is not None:
                residual = measured - predicted
                error_pct = abs(residual / measured * 100) if measured > 0 else 0
                print(f"  Measured:     {measured:.2f} W")
                print(f"  Residual:     {residual:.2f} W")
                print(f"  Error:        {error_pct:.1f}%")
            
            if confidence:
                ci_low, ci_high = confidence
                print(f"\nConfidence Interval (95%):")
                print(f"  Lower Bound:  {ci_low:.2f} W")
                print(f"  Upper Bound:  {ci_high:.2f} W")
                print(f"  Width:        {ci_high - ci_low:.2f} W")
                print(f"\n  Interpretation: The model predicts with 95% confidence that")
                print(f"  the true power consumption is between {ci_low:.1f}W and {ci_high:.1f}W")
    
    except Exception as e:
        logger.warning(f"Error making prediction for scenario: {e}")
    
    # Efficiency score
    efficiency = scenario.get('efficiency_score')
    if efficiency is not None:
        print(f"\nEfficiency Score: {efficiency:.4f}")


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='ML model debugging and diagnostics',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Show residual analysis and feature importance
  python3 ml_debug.py
  
  # Analyze specific scenario
  python3 ml_debug.py --scenario "4 Streams @ 2500k"
  
  # Adjust outlier detection threshold
  python3 ml_debug.py --outlier-threshold 3.0
  
  # JSON output
  python3 ml_debug.py --json
        """
    )
    
    parser.add_argument(
        '--model',
        type=Path,
        help='Path to specific model file (default: search for latest)'
    )
    
    parser.add_argument(
        '--scenario',
        type=str,
        help='Scenario name for deep dive analysis'
    )
    
    parser.add_argument(
        '--outlier-threshold',
        type=float,
        default=2.0,
        help='Threshold for outlier detection (standard deviations, default: 2.0)'
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
    
    # Initialize loader
    loader = ModelLoader()
    
    # Find model
    if args.model:
        model_path = args.model
        if not model_path.exists():
            logger.error(f"Model file not found: {model_path}")
            sys.exit(1)
    else:
        model_paths = loader.find_models()
        if not model_paths:
            logger.error("No trained models found")
            print("\nNo trained models found. Train a model first:")
            print("  python3 analyze_results.py")
            sys.exit(1)
        model_path = model_paths[0]  # Use first/latest
    
    logger.info(f"Using model: {model_path}")
    
    # Load model
    model = loader.load_model(model_path)
    if model is None:
        logger.error("Failed to load model")
        sys.exit(1)
    
    # Load test results
    test_results = loader.load_test_results(args.results_dir)
    if test_results is None:
        logger.error("Failed to load test results")
        sys.exit(1)
    
    scenarios = test_results.get('scenarios', [])
    if not scenarios:
        logger.warning("No scenarios found in test results")
        sys.exit(0)
    
    # Scenario deep dive mode
    if args.scenario:
        matching = [s for s in scenarios if args.scenario.lower() in s.get('name', '').lower()]
        if not matching:
            logger.error(f"Scenario not found: {args.scenario}")
            print(f"\nAvailable scenarios:")
            for s in scenarios:
                print(f"  - {s.get('name', 'Unknown')}")
            sys.exit(1)
        
        print_scenario_deep_dive(matching[0], model, loader, args.json)
        sys.exit(0)
    
    # Full analysis mode
    if not args.json:
        print(f"\n{'='*70}")
        print(f"ML MODEL DIAGNOSTICS")
        print(f"{'='*70}")
        print(f"Model:     {model_path.name}")
        print(f"Scenarios: {len(scenarios)}")
    
    # Compute residuals
    residuals = loader.compute_residuals(model, scenarios)
    
    if not residuals:
        logger.warning("No residuals could be computed")
        sys.exit(0)
    
    # Detect outliers
    outliers = loader.detect_outliers(residuals, threshold=args.outlier_threshold)
    
    # Print residual analysis
    print_residual_analysis(residuals, outliers, args.json)
    
    # Get feature importance
    feature_importance = loader.get_feature_importance(model)
    
    if feature_importance:
        if not args.json:
            print()  # Spacing
        print_feature_importance(feature_importance, args.json)
    
    if not args.json:
        print(f"\n{'='*70}")
        print(f"Analysis complete")
        print(f"{'='*70}\n")


if __name__ == '__main__':
    main()
