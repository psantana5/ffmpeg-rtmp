"""
Energy-Aware Scalability Prediction Model

This module provides predictive modeling for power consumption based on
transcoding workload characteristics (e.g., number of concurrent streams).

The PowerPredictor class trains regression models on historical measurements
and predicts power consumption for untested stream counts, enabling capacity
planning and energy-aware scaling decisions.

Mathematical Model
==================

Linear Regression (< 6 unique stream counts):
    Power(streams) = β₀ + β₁ × streams
    
    Where:
    - β₀ (intercept): Baseline power consumption (idle/overhead)
    - β₁ (coefficient): Incremental power per additional stream
    - streams: Number of concurrent transcoding streams

Polynomial Regression (≥ 6 unique stream counts):
    Power(streams) = β₀ + β₁ × streams + β₂ × streams²
    
    Where:
    - β₀ (intercept): Baseline power consumption
    - β₁ (linear coefficient): Linear component of power scaling
    - β₂ (quadratic coefficient): Captures non-linear scaling effects
    - streams: Number of concurrent transcoding streams
    
    The quadratic term captures effects like:
    - Thermal throttling at high loads
    - Cache contention and memory bandwidth saturation
    - CPU frequency scaling behavior

Data Requirements
=================

Input Data (from ResultsAnalyzer scenarios):
    - Scenario name: String containing stream count information
      Examples: "4 Streams @ 2500k", "8 streams @ 1080p"
    - Power data: Dictionary with 'mean_watts' key
      Example: {'mean_watts': 150.0}

Training Data Structure:
    - List of (streams: int, power: float) tuples
    - Minimum: 1 data point (will train, but predictions may be poor)
    - Recommended: 4+ data points for linear, 7+ for polynomial
    - Missing power data is automatically filtered out

Model Selection Logic:
    - If unique_stream_counts < 6: Use Linear Regression
      Rationale: Insufficient data for reliable polynomial fitting
    - If unique_stream_counts ≥ 6: Use Polynomial Regression (degree=2)
      Rationale: Enough data to capture non-linear power scaling

Prediction Methodology
======================

The model uses scikit-learn's LinearRegression with optional polynomial
feature transformation:

1. Training Phase (fit method):
   - Parse scenario names to extract stream counts
   - Filter scenarios with valid power measurements
   - Create feature matrix X (stream counts) and target vector y (power)
   - For polynomial: Transform X using PolynomialFeatures(degree=2)
   - Fit LinearRegression to transformed features
   - Calculate R² score to assess model quality

2. Prediction Phase (predict method):
   - Accept arbitrary stream count as input
   - For polynomial: Apply same feature transformation
   - Use trained model to predict power consumption
   - Clamp predictions to non-negative values (physical constraint)

Limitations and Caveats
========================

1. Assumes consistent hardware and configuration across measurements
2. Does not account for:
   - Different video codecs (H.264 vs H.265 vs AV1)
   - Different resolutions or bitrates per stream
   - Ambient temperature effects on thermal throttling
   - Power management settings (governor, turbo boost)
3. Extrapolation beyond training range may be unreliable
4. Small datasets (< 3 points) will have poor prediction quality
5. Model assumes power scales primarily with stream count, not other factors

Use Cases
=========

1. Capacity Planning: Predict power for N concurrent streams
2. Cost Estimation: Estimate energy costs for different workload sizes
3. Thermal Management: Identify safe operating limits before testing
4. Infrastructure Sizing: Determine power requirements for target throughput
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
    
    This class implements a machine learning model for predicting system power
    consumption as a function of transcoding workload (measured in concurrent streams).
    
    Architecture:
    1. Extracts stream counts from scenario names (e.g., "4 Streams @ 2500k" → 4)
    2. Trains a regression model on measured power vs stream count
    3. Automatically switches to polynomial regression (degree=2) if enough data
    4. Predicts power for arbitrary stream counts
    
    Model Selection:
    - Linear regression: Used when < 6 unique stream counts (simpler, more stable)
    - Polynomial regression (degree=2): Used when ≥ 6 unique stream counts
      (captures non-linear effects like thermal throttling)
    
    Design principles:
    - Graceful degradation: falls back to linear if insufficient data
    - Robust parsing: handles various scenario name formats
    - Production-ready: handles edge cases and missing data
    - Physical constraints: predictions are clamped to non-negative values
    
    Attributes:
        model: sklearn LinearRegression model (None until trained)
        poly_features: sklearn PolynomialFeatures transformer (None if linear)
        is_polynomial: bool indicating if polynomial regression is used
        training_data: List of (streams, power) tuples used for training
    
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
        """
        Initialize the power predictor.
        
        Creates an untrained predictor with default settings.
        Call fit() with training data before making predictions.
        """
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
        
        Training Algorithm:
        -------------------
        1. Data Extraction:
           - Parse each scenario name to infer stream count
           - Extract mean_watts from power measurements
           - Filter out scenarios missing either value
        
        2. Feature Engineering:
           - X (features): Stream counts as 1D array [n_samples, 1]
           - y (target): Mean power measurements as 1D array [n_samples]
        
        3. Model Selection:
           - Count unique stream values in training data
           - If unique_streams < 6: Use Linear Regression
             Formula: Power = β₀ + β₁ × streams
           - If unique_streams ≥ 6: Use Polynomial Regression (degree=2)
             Formula: Power = β₀ + β₁ × streams + β₂ × streams²
        
        4. Model Training:
           - For polynomial: Transform X to [1, streams, streams²]
           - Fit LinearRegression using ordinary least squares (OLS)
           - OLS minimizes: Σ(y_true - y_pred)²
        
        5. Model Validation:
           - Calculate R² score: R² = 1 - (SS_res / SS_tot)
             Where SS_res = Σ(y_true - y_pred)²
                   SS_tot = Σ(y_true - y_mean)²
           - R² ranges from -∞ to 1 (1 = perfect fit)
           - Log R² for model quality assessment
        
        Data Requirements:
        ------------------
        - Minimum: 1 scenario with valid stream count and power data
        - Recommended: 4+ for linear, 7+ for polynomial
        - Each scenario dict must have:
          * 'name': String with stream count information
          * 'power': Dict with 'mean_watts' key (float)
        
        Edge Cases Handled:
        -------------------
        - Missing 'power' key: Scenario skipped
        - None or missing 'mean_watts': Scenario skipped
        - Cannot infer stream count: Scenario skipped, debug logged
        - Zero valid scenarios: Returns False, warning logged
        
        Args:
            scenarios: List of scenario dicts from ResultsAnalyzer.
                      Each dict should contain:
                      - 'name': str (e.g., "4 Streams @ 2500k")
                      - 'power': {'mean_watts': float}
            
        Returns:
            True if model was trained successfully (>0 valid data points),
            False if no valid training data found.
            
        Side Effects:
            - Sets self.model to trained LinearRegression
            - Sets self.poly_features (if polynomial) or None (if linear)
            - Sets self.is_polynomial flag
            - Stores training_data as list of (streams, power) tuples
            - Logs training progress and R² score
            
        Example:
            >>> predictor = PowerPredictor()
            >>> scenarios = [
            ...     {'name': '2 Streams @ 2500k', 'power': {'mean_watts': 80.0}},
            ...     {'name': '4 Streams @ 2500k', 'power': {'mean_watts': 150.0}},
            ...     {'name': '8 Streams @ 2500k', 'power': {'mean_watts': 280.0}},
            ... ]
            >>> success = predictor.fit(scenarios)
            >>> if success:
            ...     print(f"Model trained on {len(predictor.training_data)} data points")
            ...     info = predictor.get_model_info()
            ...     print(f"Model type: {info['model_type']}")
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
        
        Prediction Algorithm:
        ---------------------
        1. Input Validation:
           - Check if model is trained (self.model is not None)
           - Return None if untrained
        
        2. Feature Preparation:
           - Create feature array: X = [[streams]]
           - For polynomial model: Transform to [1, streams, streams²]
             Using PolynomialFeatures.transform()
           - For linear model: Use raw stream count
        
        3. Prediction:
           - Linear: Power = β₀ + β₁ × streams
           - Polynomial: Power = β₀ + β₁ × streams + β₂ × streams²
           - Where β coefficients were learned during training
        
        4. Post-Processing:
           - Clamp prediction to non-negative values: max(0, prediction)
           - Rationale: Physical constraint (power cannot be negative)
           - This handles edge cases like predicting for 0 streams
        
        Interpolation vs Extrapolation:
        --------------------------------
        - Interpolation (within training range): Generally reliable
          Example: Trained on [2, 4, 8] streams, predict for 6 streams
        
        - Extrapolation (outside training range): Use with caution
          Example: Trained on [2, 4, 8] streams, predict for 16 streams
          
          Risks:
          * Linear model: Assumes constant power per stream (may diverge)
          * Polynomial model: Can diverge rapidly outside training range
          * Real systems: May have thermal limits, throttling not in model
        
        Recommended Usage:
        ------------------
        - Best: Predict within or near training range
        - Acceptable: Predict within 2x max training stream count
        - Caution: Predict beyond 2x max training stream count
        
        Physical Interpretation:
        ------------------------
        The predicted power represents the expected system-wide power
        consumption (CPU package + DRAM) when transcoding N concurrent
        streams with similar characteristics to training data.
        
        This does NOT account for:
        - Different codec settings (preset, tune, etc.)
        - Different resolutions or bitrates per stream
        - Ambient temperature effects
        - Power management policy changes
        
        Args:
            streams: Number of concurrent transcoding streams (integer).
                    Can be any non-negative integer, though extrapolation
                    beyond training range is less reliable.
            
        Returns:
            Predicted mean power consumption in watts (float), or None if
            model has not been trained. Predictions are guaranteed to be
            non-negative due to physical constraint clamping.
            
        Raises:
            No exceptions raised. Returns None for untrained model.
            
        Example:
            >>> predictor = PowerPredictor()
            >>> scenarios = [...]  # Training data
            >>> predictor.fit(scenarios)
            >>> 
            >>> # Interpolation (reliable)
            >>> power_6 = predictor.predict(6)
            >>> print(f"6 streams: {power_6:.2f} W")
            >>> 
            >>> # Extrapolation (use with caution)
            >>> power_16 = predictor.predict(16)
            >>> print(f"16 streams (extrapolated): {power_16:.2f} W")
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
