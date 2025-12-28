#!/usr/bin/env python3
"""
ML Overview - Display trained model information.

Shows currently trained models with metadata:
- Model name, version, and type
- Feature list used for prediction
- Training sample count
- Performance metrics (R², RMSE, MAE)
- Hardware profile associated with the model
"""

import argparse
import json
import logging
import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from advisor.modeling import MultivariatePredictor, PowerPredictor
from scripts.utils.model_loader import ModelLoader

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def format_feature_list(features: list, max_display: int = 10) -> str:
    """Format feature list for display."""
    if not features:
        return "None"
    
    if len(features) <= max_display:
        return ", ".join(features)
    
    displayed = ", ".join(features[:max_display])
    return f"{displayed}, ... ({len(features)} total)"


def format_model_type(model_type: str) -> str:
    """Format model type string for display."""
    type_map = {
        'linear': 'Linear Regression',
        'poly2': 'Polynomial Regression (degree 2)',
        'poly3': 'Polynomial Regression (degree 3)',
        'rf': 'Random Forest',
        'gbm': 'Gradient Boosting',
        'polynomial': 'Polynomial Regression',
    }
    return type_map.get(model_type, model_type)


def print_model_overview(model_path: Path, model: object, metadata: dict, json_output: bool = False):
    """Print overview for a single model."""
    if json_output:
        output = {
            'model_path': str(model_path),
            'metadata': metadata
        }
        print(json.dumps(output, indent=2))
        return
    
    # Human-readable output
    print(f"\n{'='*70}")
    print(f"Model: {model_path.name}")
    print(f"{'='*70}")
    
    print(f"Path:           {model_path}")
    print(f"Type:           {metadata.get('model_type', 'Unknown')}")
    print(f"Trained:        {'Yes' if metadata.get('trained') else 'No'}")
    
    if not metadata.get('trained'):
        print("\nModel not trained yet.")
        return
    
    # Best model (for ensemble)
    if 'best_model' in metadata:
        print(f"Best Model:     {format_model_type(metadata['best_model'])}")
    elif 'model_type' in metadata and metadata['model_type'] in ['linear', 'polynomial']:
        print(f"Algorithm:      {format_model_type(metadata['model_type'])}")
    
    # Training info
    print(f"\nTraining:")
    print(f"  Samples:      {metadata.get('n_samples', 0)}")
    
    if 'stream_range' in metadata and metadata['stream_range']:
        min_s, max_s = metadata['stream_range']
        print(f"  Stream Range: {min_s} - {max_s}")
    
    # Features
    features = metadata.get('features', metadata.get('feature_names', []))
    if features:
        print(f"\nFeatures ({len(features)}):")
        print(f"  {format_feature_list(features)}")
    
    # Target
    if 'target' in metadata and metadata['target']:
        print(f"\nTarget:         {metadata['target']}")
    
    # Performance metrics
    performance = metadata.get('performance', {})
    if performance:
        print(f"\nPerformance:")
        
        if 'best_score' in metadata:
            best_score = metadata['best_score']
            if 'r2' in best_score:
                print(f"  R²:           {best_score['r2']:.4f}")
            if 'rmse' in best_score:
                print(f"  RMSE:         {best_score['rmse']:.2f}")
        
        # Show all models in ensemble
        if isinstance(performance, dict) and len(performance) > 1:
            print(f"\n  Model Comparison:")
            for model_name, scores in sorted(performance.items(), key=lambda x: x[1].get('r2', 0), reverse=True):
                r2 = scores.get('r2', 0)
                rmse = scores.get('rmse', 0)
                print(f"    {format_model_type(model_name):30s} R²={r2:.4f}  RMSE={rmse:.2f}")
    
    # Hardware info
    hardware = metadata.get('hardware', metadata.get('hardware_model'))
    if hardware:
        print(f"\nHardware:       {hardware}")
    
    # Model version
    if 'version' in metadata:
        print(f"Version:        {metadata['version']}")
    
    # Confidence level (for multivariate)
    if 'confidence_level' in metadata:
        print(f"Confidence:     {metadata['confidence_level']*100:.0f}%")


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='Display ML model overview and metadata',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Show all trained models
  python3 ml_overview.py
  
  # Show specific model
  python3 ml_overview.py --model models/multivariate_predictor.pkl
  
  # JSON output
  python3 ml_overview.py --json
        """
    )
    
    parser.add_argument(
        '--model',
        type=Path,
        help='Path to specific model file (default: search for all models)'
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
    
    # Initialize model loader
    loader = ModelLoader()
    
    # Find models
    if args.model:
        model_paths = [args.model] if args.model.exists() else []
        if not model_paths:
            logger.error(f"Model file not found: {args.model}")
            sys.exit(1)
    else:
        model_paths = loader.find_models()
    
    if not model_paths:
        logger.warning("No trained models found")
        print("\nNo trained models found.")
        print("\nTip: Train a model by running:")
        print("  python3 analyze_results.py")
        sys.exit(0)
    
    logger.info(f"Found {len(model_paths)} model(s)")
    
    # Process each model
    all_results = []
    
    for model_path in model_paths:
        try:
            model = loader.load_model(model_path)
            if model is None:
                continue
            
            metadata = loader.get_model_metadata(model)
            
            if args.json:
                all_results.append({
                    'model_path': str(model_path),
                    'metadata': metadata
                })
            else:
                print_model_overview(model_path, model, metadata, json_output=False)
        
        except Exception as e:
            logger.error(f"Error processing model {model_path}: {e}")
            continue
    
    if args.json and all_results:
        print(json.dumps({'models': all_results}, indent=2))
    
    if not args.json:
        print(f"\n{'='*70}")
        print(f"Total models: {len(model_paths)}")
        print(f"{'='*70}\n")


if __name__ == '__main__':
    main()
