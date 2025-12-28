"""
Energy-Aware Scalability Prediction Model

This module provides predictive modeling for power consumption based on
transcoding workload characteristics (e.g., number of concurrent streams).

The PowerPredictor class trains regression models on historical measurements
and predicts power consumption for untested stream counts, enabling capacity
planning and energy-aware scaling decisions.
"""

import logging
import re
from typing import Dict, List, Optional

import numpy as np
from sklearn.linear_model import LinearRegression
from sklearn.preprocessing import PolynomialFeatures

logger = logging.getLogger(__name__)


class PowerPredictor:
    """
    Predicts power consumption based on number of concurrent streams.
    
    This class:
    1. Extracts stream counts from scenario names (e.g., "4 Streams @ 2500k" → 4)
    2. Trains a regression model on measured power vs stream count
    3. Automatically switches to polynomial regression (degree=2) if enough data
    4. Predicts power for arbitrary stream counts
    
    Design principles:
    - Graceful degradation: falls back to linear if insufficient data
    - Robust parsing: handles various scenario name formats
    - Production-ready: handles edge cases and missing data
    
    Example:
        >>> predictor = PowerPredictor()
        >>> scenarios = [
        ...     {'name': '2 Streams @ 2500k', 'power': {'mean_watts': 80.0}},
        ...     {'name': '4 Streams @ 2500k', 'power': {'mean_watts': 150.0}},
        ... ]
        >>> predictor.fit(scenarios)
        >>> power = predictor.predict(8)  # Predict power for 8 streams
        >>> print(f"Predicted power for 8 streams: {power:.2f} W")
    """
    
    def __init__(self):
        """Initialize the power predictor."""
        self.model = None
        self.poly_features = None
        self.is_polynomial = False
        self.training_data = []  # Store (streams, power) tuples
        
    def _infer_stream_count(self, scenario_name: str) -> Optional[int]:
        """
        Infer number of streams from scenario name.
        
        Supports patterns like:
        - "4 Streams @ 2500k" → 4
        - "8 streams @ 1080p" → 8
        - "Single Stream" → 1
        - "2-stream test" → 2
        
        Args:
            scenario_name: Name of the scenario
            
        Returns:
            Number of streams, or None if cannot be inferred
        """
        name_lower = scenario_name.lower()
        
        # Pattern 1: "N stream(s)" or "N-stream"
        match = re.search(r'(\d+)\s*[-\s]*streams?', name_lower)
        if match:
            return int(match.group(1))
        
        # Pattern 2: "single stream" → 1
        if 'single' in name_lower and 'stream' in name_lower:
            return 1
        
        # Pattern 3: "multi stream" without number → None (ambiguous)
        if 'multi' in name_lower:
            return None
        
        # Pattern 4: Check if scenario name starts with a number
        match = re.match(r'^(\d+)\s+', scenario_name)
        if match:
            return int(match.group(1))
        
        return None
    
    def fit(self, scenarios: List[Dict]) -> bool:
        """
        Train the power prediction model on scenario data.
        
        This method:
        1. Extracts (streams, power) pairs from scenarios
        2. Filters out scenarios without power data or stream counts
        3. Trains linear regression by default
        4. Switches to polynomial (degree=2) if >= 6 unique stream counts
        
        Args:
            scenarios: List of scenario dicts from ResultsAnalyzer
            
        Returns:
            True if model was trained successfully, False otherwise
            
        Example:
            >>> predictor = PowerPredictor()
            >>> success = predictor.fit(scenarios)
            >>> if success:
            ...     print(f"Model trained on {len(predictor.training_data)} data points")
        """
        # Extract (streams, power) pairs
        training_pairs = []
        
        for scenario in scenarios:
            # Skip scenarios without power data
            if 'power' not in scenario:
                continue
            
            mean_power = scenario['power'].get('mean_watts')
            if mean_power is None:
                continue
            
            # Try to infer stream count from name
            streams = self._infer_stream_count(scenario['name'])
            if streams is None:
                logger.debug(f"Could not infer stream count from: {scenario['name']}")
                continue
            
            training_pairs.append((streams, mean_power))
        
        if not training_pairs:
            logger.warning("No valid training data for PowerPredictor")
            return False
        
        # Store training data
        self.training_data = training_pairs
        
        # Prepare feature and target arrays
        X = np.array([streams for streams, _ in training_pairs]).reshape(-1, 1)
        y = np.array([power for _, power in training_pairs])
        
        # Count unique stream counts
        unique_streams = len(set(streams for streams, _ in training_pairs))
        
        # Decide on model type
        if unique_streams >= 6:
            # Use polynomial regression (degree=2) for richer model
            logger.info(
                f"Using polynomial regression (degree=2) with "
                f"{unique_streams} unique stream counts"
            )
            self.is_polynomial = True
            self.poly_features = PolynomialFeatures(degree=2)
            X_poly = self.poly_features.fit_transform(X)
            self.model = LinearRegression()
            self.model.fit(X_poly, y)
        else:
            # Use linear regression for small datasets
            logger.info(f"Using linear regression with {unique_streams} unique stream counts")
            self.is_polynomial = False
            self.poly_features = None
            self.model = LinearRegression()
            self.model.fit(X, y)
        
        # Log model statistics
        if self.is_polynomial:
            X_transformed = self.poly_features.transform(X)
            y_pred = self.model.predict(X_transformed)
        else:
            y_pred = self.model.predict(X)
        
        # Calculate R² score
        ss_res = np.sum((y - y_pred) ** 2)
        ss_tot = np.sum((y - np.mean(y)) ** 2)
        r2 = 1 - (ss_res / ss_tot) if ss_tot > 0 else 0
        
        logger.info(f"PowerPredictor trained on {len(training_pairs)} data points, R² = {r2:.4f}")
        
        return True
    
    def predict(self, streams: int) -> Optional[float]:
        """
        Predict power consumption for a given number of streams.
        
        Args:
            streams: Number of concurrent streams
            
        Returns:
            Predicted mean power in watts, or None if model not trained
            
        Example:
            >>> power = predictor.predict(8)
            >>> if power:
            ...     print(f"8 streams: {power:.2f} W")
        """
        if self.model is None:
            logger.warning("PowerPredictor not trained yet")
            return None
        
        X = np.array([[streams]])
        
        if self.is_polynomial:
            X = self.poly_features.transform(X)
        
        prediction = self.model.predict(X)[0]
        
        # Ensure non-negative prediction
        return max(0.0, float(prediction))
    
    def get_model_info(self) -> Dict:
        """
        Get information about the trained model.
        
        Returns:
            Dict with model metadata:
                - 'trained': bool
                - 'model_type': 'linear' or 'polynomial'
                - 'n_samples': number of training samples
                - 'stream_range': (min, max) stream counts in training data
        """
        if self.model is None:
            return {
                'trained': False,
                'model_type': None,
                'n_samples': 0,
                'stream_range': None,
            }
        
        streams = [s for s, _ in self.training_data]
        
        return {
            'trained': True,
            'model_type': 'polynomial' if self.is_polynomial else 'linear',
            'n_samples': len(self.training_data),
            'stream_range': (min(streams), max(streams)) if streams else None,
        }
