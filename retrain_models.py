#!/usr/bin/env python3
"""
ML Model Retraining Script

Automatically retrains power prediction models from test results.
Supports:
- Per-hardware model versioning
- Multiple model types (PowerPredictor, MultivariatePredictor)
- Model artifact storage and versioning
- Automatic model selection based on performance

Usage:
    python3 retrain_models.py --results-dir ./test_results
    python3 retrain_models.py --results-dir ./test_results --hardware-id intel_i7_9700k
    make retrain-models
"""

import argparse
import json
import logging
import platform
import sys
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional

from advisor import MultivariatePredictor, PowerPredictor

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class ModelRetrainer:
    """
    Handles model retraining from test results.
    
    Features:
        - Load scenarios from JSON result files
        - Train PowerPredictor and MultivariatePredictor
        - Store models with versioning
        - Generate model metadata
        - Hardware-aware model organization
    """
    
    def __init__(
        self,
        results_dir: Path,
        models_dir: Path,
        hardware_id: Optional[str] = None
    ):
        """
        Initialize model retrainer.
        
        Args:
            results_dir: Directory containing test_results_*.json files
            models_dir: Directory to store trained models
            hardware_id: Hardware identifier (auto-detected if not provided)
        """
        self.results_dir = results_dir
        self.models_dir = models_dir
        self.hardware_id = hardware_id or self._detect_hardware_id()
        
        # Create hardware-specific model directory
        self.hardware_model_dir = self.models_dir / self.hardware_id
        self.hardware_model_dir.mkdir(parents=True, exist_ok=True)
        
        logger.info(f"ModelRetrainer initialized for hardware: {self.hardware_id}")
        logger.info(f"Results dir: {self.results_dir}")
        logger.info(f"Models dir: {self.hardware_model_dir}")
    
    def _detect_hardware_id(self) -> str:
        """
        Auto-detect hardware identifier from system information.
        
        Returns:
            Hardware ID string (e.g., "intel_i7_9700k_linux")
        """
        try:
            # Get CPU info
            cpu_info = platform.processor() or 'unknown_cpu'
            # Sanitize for filesystem
            cpu_id = cpu_info.replace(' ', '_').replace('(', '').replace(')', '')
            cpu_id = ''.join(c for c in cpu_id if c.isalnum() or c == '_')
            
            # Get OS
            os_name = platform.system().lower()
            
            hardware_id = f"{cpu_id}_{os_name}"
            
            # Truncate if too long
            if len(hardware_id) > 100:
                hardware_id = hardware_id[:100]
            
            return hardware_id
        except Exception as e:
            logger.warning(f"Could not detect hardware ID: {e}")
            return "unknown_hardware"
    
    def load_scenarios(self) -> List[Dict]:
        """
        Load all scenarios from JSON result files.
        
        Returns:
            List of scenario dicts
        """
        scenarios = []
        
        # Find all test result JSON files
        json_files = list(self.results_dir.glob('test_results_*.json'))
        
        if not json_files:
            logger.warning(f"No test result files found in {self.results_dir}")
            return scenarios
        
        logger.info(f"Found {len(json_files)} result files")
        
        for json_file in json_files:
            try:
                with open(json_file) as f:
                    data = json.load(f)
                
                file_scenarios = data.get('scenarios', [])
                scenarios.extend(file_scenarios)
                
                logger.debug(f"Loaded {len(file_scenarios)} scenarios from {json_file.name}")
            
            except Exception as e:
                logger.error(f"Error loading {json_file}: {e}")
                continue
        
        logger.info(f"Total scenarios loaded: {len(scenarios)}")
        return scenarios
    
    def retrain_power_predictor(self, scenarios: List[Dict]) -> bool:
        """
        Retrain PowerPredictor model.
        
        Args:
            scenarios: List of scenario dicts
            
        Returns:
            True if training successful
        """
        logger.info("Training PowerPredictor...")
        
        predictor = PowerPredictor()
        
        if not predictor.fit(scenarios):
            logger.warning("PowerPredictor training failed (insufficient data)")
            return False
        
        # Get model info
        info = predictor.get_model_info()
        logger.info(
            f"PowerPredictor trained: {info['model_type']}, "
            f"{info['n_samples']} samples, "
            f"stream range: {info['stream_range']}"
        )
        
        # Save model
        timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
        model_filename = f"power_predictor_{timestamp}.pkl"
        model_path = self.hardware_model_dir / model_filename
        
        self._save_power_predictor(predictor, model_path, info)
        
        # Create "latest" symlink
        latest_path = self.hardware_model_dir / "power_predictor_latest.pkl"
        if latest_path.exists():
            latest_path.unlink()
        try:
            latest_path.symlink_to(model_filename)
        except Exception:
            # Symlinks may not work on all systems, just copy instead
            import shutil
            shutil.copy(model_path, latest_path)
        
        logger.info(f"PowerPredictor saved to {model_path}")
        
        return True
    
    def _save_power_predictor(
        self, predictor: PowerPredictor, path: Path, info: Dict
    ):
        """Save PowerPredictor with metadata."""
        import pickle
        
        model_data = {
            'model': predictor.model,
            'poly_features': predictor.poly_features,
            'is_polynomial': predictor.is_polynomial,
            'training_data': predictor.training_data,
            'info': info,
            'hardware_id': self.hardware_id,
            'timestamp': datetime.now().isoformat(),
            'version': '1.0'
        }
        
        with open(path, 'wb') as f:
            pickle.dump(model_data, f)
    
    def retrain_multivariate_predictor(self, scenarios: List[Dict]) -> bool:
        """
        Retrain MultivariatePredictor model.
        
        Args:
            scenarios: List of scenario dicts
            
        Returns:
            True if training successful
        """
        logger.info("Training MultivariatePredictor...")
        
        predictor = MultivariatePredictor()
        
        if not predictor.fit(scenarios, hardware_id=self.hardware_id):
            logger.warning("MultivariatePredictor training failed (insufficient data)")
            return False
        
        # Get model info
        info = predictor.get_model_info()
        logger.info(
            f"MultivariatePredictor trained: {info['best_model']}, "
            f"{info['n_samples']} samples, "
            f"R²={info['best_score']['r2']:.4f}"
        )
        
        # Save model
        timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
        model_filename = f"multivariate_predictor_{timestamp}.pkl"
        model_path = self.hardware_model_dir / model_filename
        
        predictor.save(model_path)
        
        # Create "latest" symlink
        latest_path = self.hardware_model_dir / "multivariate_predictor_latest.pkl"
        if latest_path.exists():
            latest_path.unlink()
        try:
            latest_path.symlink_to(model_filename)
        except Exception:
            # Symlinks may not work on all systems, just copy instead
            import shutil
            shutil.copy(model_path, latest_path)
        
        logger.info(f"MultivariatePredictor saved to {model_path}")
        
        return True
    
    def generate_metadata(self) -> Dict:
        """
        Generate metadata file for trained models.
        
        Returns:
            Metadata dict
        """
        metadata = {
            'hardware_id': self.hardware_id,
            'timestamp': datetime.now().isoformat(),
            'platform': {
                'system': platform.system(),
                'processor': platform.processor(),
                'machine': platform.machine(),
                'python_version': platform.python_version()
            },
            'models': {
                'power_predictor': {
                    'path': 'power_predictor_latest.pkl',
                    'type': 'PowerPredictor'
                },
                'multivariate_predictor': {
                    'path': 'multivariate_predictor_latest.pkl',
                    'type': 'MultivariatePredictor'
                }
            }
        }
        
        return metadata
    
    def save_metadata(self):
        """Save model metadata to JSON file."""
        metadata = self.generate_metadata()
        metadata_path = self.hardware_model_dir / 'metadata.json'
        
        with open(metadata_path, 'w') as f:
            json.dump(metadata, f, indent=2)
        
        logger.info(f"Metadata saved to {metadata_path}")
    
    def retrain_all(self) -> bool:
        """
        Retrain all models.
        
        Returns:
            True if at least one model trained successfully
        """
        # Load scenarios
        scenarios = self.load_scenarios()
        
        if not scenarios:
            logger.error("No scenarios to train on")
            return False
        
        # Train models
        power_success = self.retrain_power_predictor(scenarios)
        multivariate_success = self.retrain_multivariate_predictor(scenarios)
        
        if not (power_success or multivariate_success):
            logger.error("All model training failed")
            return False
        
        # Save metadata
        self.save_metadata()
        
        logger.info("Model retraining complete!")
        return True


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='Retrain ML models from test results'
    )
    parser.add_argument(
        '--results-dir',
        type=Path,
        default=Path('./test_results'),
        help='Directory containing test result JSON files'
    )
    parser.add_argument(
        '--models-dir',
        type=Path,
        default=Path('./models'),
        help='Directory to store trained models'
    )
    parser.add_argument(
        '--hardware-id',
        type=str,
        help='Hardware identifier (auto-detected if not provided)'
    )
    
    args = parser.parse_args()
    
    # Validate directories
    if not args.results_dir.exists():
        logger.error(f"Results directory not found: {args.results_dir}")
        return 1
    
    # Create models directory if needed
    args.models_dir.mkdir(parents=True, exist_ok=True)
    
    # Retrain models
    retrainer = ModelRetrainer(
        results_dir=args.results_dir,
        models_dir=args.models_dir,
        hardware_id=args.hardware_id
    )
    
    if retrainer.retrain_all():
        logger.info("✓ Model retraining successful")
        return 0
    else:
        logger.error("✗ Model retraining failed")
        return 1


if __name__ == '__main__':
    sys.exit(main())
