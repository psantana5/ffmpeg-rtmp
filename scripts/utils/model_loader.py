#!/usr/bin/env python3
"""
Model loader utility for ML diagnostics.

Handles loading trained models from project paths, inspecting metadata,
and graceful error handling for missing models.
"""

import json
import logging
import pickle
from pathlib import Path
from typing import Dict, List, Optional, Tuple

import numpy as np

logger = logging.getLogger(__name__)


class ModelLoader:
    """Load and inspect trained ML models."""
    
    DEFAULT_MODEL_PATHS = [
        'models/multivariate_predictor.pkl',
        'models/power_predictor.pkl',
        'test_results/latest_model.pkl',
    ]
    
    def __init__(self, search_paths: Optional[List[str]] = None):
        """
        Initialize model loader.
        
        Args:
            search_paths: Optional list of paths to search for models
        """
        self.search_paths = search_paths or self.DEFAULT_MODEL_PATHS
        logger.debug(f"ModelLoader initialized with {len(self.search_paths)} search paths")
    
    def find_models(self, base_dir: Path = None) -> List[Path]:
        """
        Find all available model files.
        
        Args:
            base_dir: Base directory to search (defaults to current directory)
            
        Returns:
            List of Path objects for found model files
        """
        if base_dir is None:
            base_dir = Path.cwd()
        
        found_models = []
        
        for search_path in self.search_paths:
            model_path = base_dir / search_path
            if model_path.exists():
                found_models.append(model_path)
                logger.debug(f"Found model: {model_path}")
        
        # Also search for any .pkl files in models/ directory
        models_dir = base_dir / 'models'
        if models_dir.exists():
            for pkl_file in models_dir.glob('*.pkl'):
                if pkl_file not in found_models:
                    found_models.append(pkl_file)
                    logger.debug(f"Found additional model: {pkl_file}")
        
        return found_models
    
    def load_model(self, model_path: Path) -> Optional[object]:
        """
        Load a pickled model from disk.
        
        Args:
            model_path: Path to model file
            
        Returns:
            Loaded model object or None on error
        """
        if not model_path.exists():
            logger.error(f"Model file not found: {model_path}")
            return None
        
        try:
            with open(model_path, 'rb') as f:
                model = pickle.load(f)
            logger.info(f"Model loaded from {model_path}")
            return model
        
        except pickle.UnpicklingError as e:
            logger.error(f"Failed to unpickle model from {model_path}: {e}")
            return None
        except Exception as e:
            logger.error(f"Error loading model from {model_path}: {e}")
            return None
    
    def get_model_metadata(self, model: object) -> Dict:
        """
        Extract metadata from a loaded model.
        
        Args:
            model: Loaded model object
            
        Returns:
            Dict with model metadata
        """
        metadata = {
            'model_type': type(model).__name__,
            'trained': False,
            'features': [],
            'target': None,
            'n_samples': 0,
            'performance': {},
        }
        
        # Try to extract metadata based on model type
        try:
            # Check for get_model_info method (our custom models)
            if hasattr(model, 'get_model_info'):
                info = model.get_model_info()
                metadata.update(info)
                
                # Add features if not present
                if 'features' not in metadata or not metadata['features']:
                    # PowerPredictor
                    if hasattr(model, 'training_data'):
                        metadata['features'] = ['stream_count']
                    # MultivariatePredictor
                    elif hasattr(model, 'feature_names'):
                        metadata['features'] = model.feature_names
            
            # PowerPredictor (fallback if no get_model_info)
            elif hasattr(model, 'model') and hasattr(model, 'training_data'):
                metadata['trained'] = model.model is not None
                metadata['n_samples'] = len(model.training_data)
                metadata['features'] = ['stream_count']
                
                if hasattr(model, 'is_polynomial'):
                    metadata['model_type'] = 'polynomial' if model.is_polynomial else 'linear'
                
                if model.training_data:
                    streams = [s for s, _ in model.training_data]
                    metadata['stream_range'] = (min(streams), max(streams))
            
            # MultivariatePredictor (fallback if no get_model_info)
            elif hasattr(model, 'pipelines') and hasattr(model, 'feature_names'):
                metadata['trained'] = bool(model.pipelines)
                metadata['features'] = model.feature_names
                
                if hasattr(model, 'target_name'):
                    metadata['target'] = model.target_name
                
                if hasattr(model, 'best_model_name'):
                    metadata['best_model'] = model.best_model_name
                
                if hasattr(model, 'model_scores'):
                    metadata['performance'] = model.model_scores
                
                if hasattr(model, 'X_train') and model.X_train is not None:
                    metadata['n_samples'] = len(model.X_train)
            
        except Exception as e:
            logger.warning(f"Error extracting model metadata: {e}")
        
        return metadata
    
    def compute_residuals(
        self,
        model: object,
        scenarios: List[Dict]
    ) -> List[Tuple[str, float, float, float]]:
        """
        Compute residuals (measured - predicted) for scenarios.
        
        Args:
            model: Trained model with predict method
            scenarios: List of scenario dicts with measured values
            
        Returns:
            List of (scenario_name, measured, predicted, residual) tuples
        """
        residuals = []
        
        for scenario in scenarios:
            try:
                # Get measured value
                power = scenario.get('power', {})
                measured = power.get('mean_watts')
                if measured is None:
                    continue
                
                # Make prediction
                predicted = None
                
                # Try different prediction methods
                if hasattr(model, '_infer_stream_count') and hasattr(model, 'predict'):
                    # PowerPredictor
                    stream_count = model._infer_stream_count(scenario.get('name', ''))
                    if stream_count is not None:
                        predicted = model.predict(stream_count)
                
                elif hasattr(model, '_extract_features') and hasattr(model, 'predict'):
                    # MultivariatePredictor
                    features = model._extract_features(scenario)
                    if features is not None and isinstance(features, dict):
                        result = model.predict(features, return_confidence=False)
                        if isinstance(result, dict):
                            predicted = result.get('mean')
                
                if predicted is not None:
                    residual = measured - predicted
                    residuals.append((
                        scenario.get('name', 'Unknown'),
                        measured,
                        predicted,
                        residual
                    ))
            
            except Exception as e:
                logger.warning(f"Error computing residual for scenario '{scenario.get('name', 'Unknown')}': {e}")
                continue
        
        return residuals
    
    def detect_outliers(
        self,
        residuals: List[Tuple[str, float, float, float]],
        threshold: float = 2.0
    ) -> List[Tuple[str, float]]:
        """
        Detect outliers based on residual standard deviations.
        
        Args:
            residuals: List of (name, measured, predicted, residual) tuples
            threshold: Number of standard deviations to classify as outlier
            
        Returns:
            List of (scenario_name, residual) tuples for outliers
        """
        if not residuals:
            return []
        
        residual_values = [r[3] for r in residuals]
        
        if len(residual_values) < 2:
            return []
        
        mean_residual = np.mean(residual_values)
        std_residual = np.std(residual_values)
        
        if std_residual == 0:
            return []
        
        outliers = []
        for name, measured, predicted, residual in residuals:
            z_score = abs((residual - mean_residual) / std_residual)
            if z_score > threshold:
                outliers.append((name, residual))
        
        return outliers
    
    def get_feature_importance(self, model: object) -> List[Tuple[str, float]]:
        """
        Extract feature importance from model.
        
        Args:
            model: Trained model
            
        Returns:
            List of (feature_name, importance) tuples sorted by importance
        """
        feature_importance = []
        
        try:
            # MultivariatePredictor with tree-based models
            if hasattr(model, 'pipelines') and hasattr(model, 'best_model_name'):
                best_model_name = model.best_model_name
                if best_model_name in ['rf', 'gbm']:
                    pipeline = model.pipelines.get(best_model_name)
                    if pipeline is not None:
                        # Get the final estimator from pipeline
                        estimator = pipeline.named_steps.get('model')
                        if hasattr(estimator, 'feature_importances_'):
                            importances = estimator.feature_importances_
                            
                            # Account for polynomial features if present
                            feature_names = model.feature_names
                            if 'poly' in pipeline.named_steps:
                                # Polynomial features expanded, use base feature names
                                poly_transformer = pipeline.named_steps['poly']
                                n_output_features = poly_transformer.n_output_features_
                                if len(importances) == n_output_features:
                                    # Sum importances for polynomial terms back to original features
                                    # This is a simplification
                                    feature_names = model.feature_names
                                    importances = importances[:len(feature_names)]
                            
                            for name, importance in zip(feature_names, importances):
                                feature_importance.append((name, float(importance)))
                
                # Linear models - use coefficient magnitude
                elif best_model_name in ['linear', 'poly2', 'poly3']:
                    pipeline = model.pipelines.get(best_model_name)
                    if pipeline is not None:
                        estimator = pipeline.named_steps.get('model')
                        if hasattr(estimator, 'coef_'):
                            coefs = np.abs(estimator.coef_)
                            feature_names = model.feature_names
                            
                            # For polynomial, only show base features
                            if len(coefs) > len(feature_names):
                                coefs = coefs[:len(feature_names)]
                            
                            for name, coef in zip(feature_names, coefs):
                                feature_importance.append((name, float(coef)))
            
            # Sort by importance descending
            feature_importance.sort(key=lambda x: x[1], reverse=True)
        
        except Exception as e:
            logger.warning(f"Error extracting feature importance: {e}")
        
        return feature_importance
    
    def load_test_results(self, results_dir: Path = None) -> Optional[Dict]:
        """
        Load the latest test results JSON file.
        
        Args:
            results_dir: Directory containing test results
            
        Returns:
            Test results dict or None if not found
        """
        if results_dir is None:
            results_dir = Path.cwd() / 'test_results'
        
        if not results_dir.exists():
            logger.error(f"Results directory not found: {results_dir}")
            return None
        
        # Find latest test_results_*.json file
        json_files = sorted(results_dir.glob('test_results_*.json'), reverse=True)
        
        if not json_files:
            logger.error(f"No test result files found in {results_dir}")
            return None
        
        latest_file = json_files[0]
        logger.info(f"Loading test results from {latest_file}")
        
        try:
            with open(latest_file, 'r') as f:
                return json.load(f)
        except Exception as e:
            logger.error(f"Error loading test results from {latest_file}: {e}")
            return None
