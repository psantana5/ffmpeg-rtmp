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
import pickle
import re
from pathlib import Path
from typing import Dict, List, Optional, Tuple

import numpy as np
from sklearn.ensemble import GradientBoostingRegressor, RandomForestRegressor
from sklearn.linear_model import LinearRegression
from sklearn.model_selection import cross_val_score
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import PolynomialFeatures, StandardScaler

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


class MultivariatePredictor:
    """
    Advanced multivariate power predictor with ensemble models and confidence intervals.
    
    This class extends the basic PowerPredictor with:
    - Multiple input features (stream_count, bitrate, pixels, cpu_usage, encoder_type, etc.)
    - Ensemble of regression models (Linear, Polynomial, RandomForest, GradientBoosting)
    - Automatic model selection via cross-validation
    - Prediction uncertainty estimates (confidence intervals)
    - Hardware-aware model storage and versioning
    - Multiple prediction targets (power, energy, efficiency)
    
    Features:
    ---------
    Input Features:
        - stream_count: Number of concurrent transcoding streams
        - bitrate_mbps: Bitrate in megabits per second
        - total_pixels: Sum of width × height × fps across all outputs
        - cpu_usage_pct: Mean CPU usage percentage during scenario
        - encoder_type: One-hot encoded (x264, NVENC, etc.)
        - hardware_cpu_model: Hashed or one-hot encoded CPU model
        - container_cpu_pct: Docker container CPU overhead percentage
    
    Prediction Targets:
        - mean_power_watts: Mean power consumption
        - total_energy_joules: Total energy consumed
        - efficiency_score: Direct efficiency prediction
    
    Model Selection:
        - Linear Regression: Baseline model
        - Polynomial Regression (degree=2,3): Non-linear relationships
        - RandomForestRegressor: Handles complex interactions
        - GradientBoostingRegressor: State-of-the-art performance
    
    Confidence Intervals:
        - Bootstrapped prediction intervals
        - Configurable confidence level (default: 95%)
    
    Hardware Awareness:
        - Per-hardware model storage
        - Automatic hardware fingerprinting
        - Fallback to universal model if hardware unknown
    
    Example:
        >>> predictor = MultivariatePredictor()
        >>> predictor.fit(scenarios, target='mean_power_watts')
        >>> prediction = predictor.predict({
        ...     'stream_count': 4,
        ...     'bitrate_mbps': 2.5,
        ...     'total_pixels': 1920*1080*30,
        ...     'cpu_usage_pct': 50.0,
        ...     'encoder_type': 'x264',
        ...     'hardware_cpu_model': 'Intel_i7_9700K',
        ...     'container_cpu_pct': 5.0
        ... })
        >>> print(f"Predicted power: {prediction['mean']} ± {prediction['ci_width']} W")
    """

    def __init__(
        self,
        models: Optional[List[str]] = None,
        confidence_level: float = 0.95,
        n_bootstrap: int = 100,
        cv_folds: int = 5,
    ):
        """
        Initialize the multivariate predictor.
        
        Args:
            models: List of model types to train. If None, uses all available.
                   Options: 'linear', 'poly2', 'poly3', 'rf', 'gbm'
            confidence_level: Confidence level for prediction intervals (0-1)
            n_bootstrap: Number of bootstrap samples for confidence intervals
            cv_folds: Number of cross-validation folds for model selection
        """
        if models is None:
            models = ['linear', 'poly2', 'poly3', 'rf', 'gbm']

        self.models = models
        self.confidence_level = confidence_level
        self.n_bootstrap = n_bootstrap
        self.cv_folds = cv_folds

        # Model storage
        self.pipelines = {}  # Dict of model_name -> sklearn Pipeline
        self.best_model_name = None
        self.best_model_score = None
        self.model_scores = {}  # Dict of model_name -> {'r2': float, 'rmse': float}

        # Feature and target information
        self.feature_names = []
        self.target_name = None
        self.encoder_categories = {}  # For one-hot encoding
        self.hardware_model = None

        # Training data storage for bootstrap
        self.X_train = None
        self.y_train = None

        # Model version
        self.version = '1.0'

    def _extract_features(self, scenario: Dict) -> Optional[Dict]:
        """
        Extract feature values from a scenario dictionary.
        
        Args:
            scenario: Scenario dict from ResultsAnalyzer with test data
            
        Returns:
            Dict of feature_name -> value, or None if missing critical data
        """
        features = {}

        # stream_count (required)
        stream_count = self._infer_stream_count(scenario.get('name', ''))
        if stream_count is None:
            return None
        features['stream_count'] = stream_count

        # bitrate_mbps
        bitrate_str = scenario.get('bitrate', '0k')
        features['bitrate_mbps'] = self._parse_bitrate_to_mbps(bitrate_str)

        # total_pixels
        features['total_pixels'] = self._compute_total_pixels(scenario)

        # cpu_usage_pct
        container_usage = scenario.get('container_usage', {})
        features['cpu_usage_pct'] = container_usage.get('cpu_percent', 0.0)

        # encoder_type (categorical)
        encoder_type = scenario.get('encoder_type', 'x264')
        features['encoder_type'] = encoder_type

        # hardware_cpu_model (categorical)
        hardware = scenario.get('hardware', {})
        cpu_model = hardware.get('cpu_model', 'unknown')
        features['hardware_cpu_model'] = cpu_model

        # container_cpu_pct (Docker overhead)
        docker_overhead = scenario.get('docker_overhead', {})
        features['container_cpu_pct'] = docker_overhead.get('cpu_percent', 0.0)

        return features

    def _infer_stream_count(self, scenario_name: str) -> Optional[int]:
        """Infer number of streams from scenario name (reuse logic from PowerPredictor)."""
        name_lower = scenario_name.lower()
        match = re.search(r'(\d+)\s*[-\s]*streams?', name_lower)
        if match:
            return int(match.group(1))
        if 'single' in name_lower and 'stream' in name_lower:
            return 1
        match = re.match(r'^(\d+)\s+', scenario_name)
        if match:
            return int(match.group(1))
        return None

    def _parse_bitrate_to_mbps(self, bitrate: str) -> float:
        """Parse bitrate string to Mbps."""
        value = bitrate.strip().upper()
        if not value or value == "N/A":
            return 0.0
        try:
            if value.endswith('M'):
                return float(value[:-1])
            elif value.endswith('K'):
                return float(value[:-1]) / 1000.0
            else:
                return float(value) / 1000.0
        except ValueError:
            return 0.0

    def _compute_total_pixels(self, scenario: Dict) -> float:
        """Compute total pixels delivered (width * height * fps * duration)."""
        duration = scenario.get('duration', 0)
        if not duration or duration <= 0:
            return 0.0

        outputs = scenario.get('outputs')
        total_pixels = 0.0

        if outputs and isinstance(outputs, list) and len(outputs) > 0:
            for output in outputs:
                resolution = output.get('resolution')
                fps = output.get('fps')
                if not resolution or not fps:
                    continue
                width, height = self._parse_resolution(resolution)
                if width is None or height is None:
                    continue
                total_pixels += width * height * fps * duration
        else:
            resolution = scenario.get('resolution')
            fps = scenario.get('fps')
            if resolution and resolution != 'N/A' and fps and fps != 'N/A':
                width, height = self._parse_resolution(resolution)
                if width is not None and height is not None:
                    total_pixels = width * height * fps * duration

        return total_pixels

    def _parse_resolution(self, resolution: str) -> Tuple[Optional[int], Optional[int]]:
        """Parse resolution string to (width, height)."""
        if not resolution or resolution == 'N/A':
            return (None, None)
        try:
            parts = resolution.lower().split('x')
            if len(parts) == 2:
                width = int(parts[0].strip())
                height = int(parts[1].strip())
                return (width, height)
        except (ValueError, AttributeError):
            pass
        return (None, None)

    def _extract_target(self, scenario: Dict, target_name: str) -> Optional[float]:
        """
        Extract target value from scenario.
        
        Args:
            scenario: Scenario dict
            target_name: One of 'mean_power_watts', 'total_energy_joules', 'efficiency_score'
            
        Returns:
            Target value or None if not available
        """
        if target_name == 'mean_power_watts':
            power = scenario.get('power', {})
            return power.get('mean_watts')
        elif target_name == 'total_energy_joules':
            power = scenario.get('power', {})
            return power.get('total_energy_joules')
        elif target_name == 'efficiency_score':
            return scenario.get('efficiency_score')
        return None

    def _encode_features(self, features_list: List[Dict]) -> np.ndarray:
        """
        Encode features into numpy array with one-hot encoding for categorical variables.
        
        Args:
            features_list: List of feature dicts
            
        Returns:
            numpy array of shape (n_samples, n_features)
        """
        if not features_list:
            return np.array([])

        # Build feature matrix
        rows = []
        for features in features_list:
            row = []

            # Numeric features
            row.append(features.get('stream_count', 0))
            row.append(features.get('bitrate_mbps', 0.0))
            row.append(features.get('total_pixels', 0.0))
            row.append(features.get('cpu_usage_pct', 0.0))
            row.append(features.get('container_cpu_pct', 0.0))

            # Categorical: encoder_type
            encoder_type = features.get('encoder_type', 'x264')
            for category in self.encoder_categories.get('encoder_type', []):
                row.append(1.0 if encoder_type == category else 0.0)

            # Categorical: hardware_cpu_model (hash to reduce dimensionality)
            cpu_model = features.get('hardware_cpu_model', 'unknown')
            # Simple hash encoding (could be improved with feature hashing)
            cpu_hash = hash(cpu_model) % 10  # Map to 0-9
            row.append(float(cpu_hash))

            rows.append(row)

        return np.array(rows)

    def fit(
        self,
        scenarios: List[Dict],
        target: str = 'mean_power_watts',
        hardware_id: Optional[str] = None,
    ) -> bool:
        """
        Train ensemble of models on scenario data.
        
        Args:
            scenarios: List of scenario dicts from ResultsAnalyzer
            target: Target variable to predict
            hardware_id: Hardware identifier for per-hardware models
            
        Returns:
            True if training successful, False otherwise
        """
        self.target_name = target
        self.hardware_model = hardware_id

        # Extract features and targets
        features_list = []
        targets_list = []

        for scenario in scenarios:
            features = self._extract_features(scenario)
            if features is None:
                continue

            target_value = self._extract_target(scenario, target)
            if target_value is None:
                continue

            features_list.append(features)
            targets_list.append(target_value)

        if len(features_list) < 2:
            logger.warning(f"Insufficient data for multivariate predictor: {len(features_list)} samples")
            return False

        # Build encoder categories from training data
        encoder_types = set(f.get('encoder_type', 'x264') for f in features_list)
        self.encoder_categories['encoder_type'] = sorted(encoder_types)

        # Encode features
        X = self._encode_features(features_list)
        y = np.array(targets_list)

        # Store for bootstrap
        self.X_train = X
        self.y_train = y

        # Build feature names
        self.feature_names = [
            'stream_count', 'bitrate_mbps', 'total_pixels',
            'cpu_usage_pct', 'container_cpu_pct'
        ]
        for enc_type in self.encoder_categories['encoder_type']:
            self.feature_names.append(f'encoder_{enc_type}')
        self.feature_names.append('hardware_hash')

        # Train ensemble of models
        logger.info(f"Training {len(self.models)} models on {len(X)} samples...")

        for model_name in self.models:
            pipeline = self._create_pipeline(model_name)

            try:
                # Train model
                pipeline.fit(X, y)

                # Cross-validation score
                if len(X) >= self.cv_folds:
                    cv_scores = cross_val_score(
                        pipeline, X, y,
                        cv=min(self.cv_folds, len(X)),
                        scoring='r2'
                    )
                    r2_score = cv_scores.mean()
                else:
                    # Not enough data for CV, use training score
                    r2_score = pipeline.score(X, y)

                # Calculate RMSE
                y_pred = pipeline.predict(X)
                rmse = np.sqrt(np.mean((y - y_pred) ** 2))

                self.pipelines[model_name] = pipeline
                self.model_scores[model_name] = {'r2': r2_score, 'rmse': rmse}

                logger.info(f"  {model_name}: R²={r2_score:.4f}, RMSE={rmse:.2f}")

            except Exception as e:
                logger.warning(f"Failed to train {model_name}: {e}")
                continue

        if not self.pipelines:
            logger.error("No models trained successfully")
            return False

        # Select best model
        self.best_model_name = max(
            self.model_scores.keys(),
            key=lambda k: self.model_scores[k]['r2']
        )
        self.best_model_score = self.model_scores[self.best_model_name]

        logger.info(
            f"Best model: {self.best_model_name} "
            f"(R²={self.best_model_score['r2']:.4f}, "
            f"RMSE={self.best_model_score['rmse']:.2f})"
        )

        return True

    def _create_pipeline(self, model_name: str) -> Pipeline:
        """
        Create sklearn pipeline for a given model type.
        
        Args:
            model_name: One of 'linear', 'poly2', 'poly3', 'rf', 'gbm'
            
        Returns:
            sklearn Pipeline with scaler and model
        """
        steps = [('scaler', StandardScaler())]

        if model_name == 'linear':
            steps.append(('model', LinearRegression()))

        elif model_name == 'poly2':
            steps.append(('poly', PolynomialFeatures(degree=2)))
            steps.append(('model', LinearRegression()))

        elif model_name == 'poly3':
            steps.append(('poly', PolynomialFeatures(degree=3)))
            steps.append(('model', LinearRegression()))

        elif model_name == 'rf':
            steps.append(('model', RandomForestRegressor(
                n_estimators=100,
                max_depth=10,
                random_state=42,
                n_jobs=-1
            )))

        elif model_name == 'gbm':
            # Try XGBoost first, fall back to sklearn GradientBoosting
            try:
                from xgboost import XGBRegressor
                steps.append(('model', XGBRegressor(
                    n_estimators=100,
                    max_depth=5,
                    learning_rate=0.1,
                    random_state=42,
                    n_jobs=-1
                )))
            except ImportError:
                logger.debug("XGBoost not available, using sklearn GradientBoostingRegressor")
                steps.append(('model', GradientBoostingRegressor(
                    n_estimators=100,
                    max_depth=5,
                    learning_rate=0.1,
                    random_state=42
                )))

        else:
            raise ValueError(f"Unknown model type: {model_name}")

        return Pipeline(steps)

    def predict(
        self,
        features: Dict,
        return_confidence: bool = True,
        model_name: Optional[str] = None
    ) -> Dict:
        """
        Predict target value with optional confidence intervals.
        
        Args:
            features: Dict of feature_name -> value
            return_confidence: Whether to compute confidence intervals
            model_name: Specific model to use (default: best model)
            
        Returns:
            Dict with keys:
                - 'mean': Predicted value
                - 'ci_low': Lower confidence bound (if return_confidence=True)
                - 'ci_high': Upper confidence bound (if return_confidence=True)
                - 'ci_width': Width of confidence interval (if return_confidence=True)
                - 'model': Model name used
        """
        if not self.pipelines:
            logger.warning("No models trained")
            return {'mean': None, 'model': None}

        # Select model
        if model_name is None:
            model_name = self.best_model_name

        if model_name not in self.pipelines:
            logger.warning(f"Model {model_name} not available, using {self.best_model_name}")
            model_name = self.best_model_name

        pipeline = self.pipelines[model_name]

        # Encode features
        X = self._encode_features([features])

        # Predict
        mean_prediction = pipeline.predict(X)[0]

        result = {
            'mean': float(max(0.0, mean_prediction)),  # Clamp to non-negative
            'model': model_name
        }

        if return_confidence:
            ci_low, ci_high = self._bootstrap_confidence_interval(X[0], model_name)
            result['ci_low'] = float(max(0.0, ci_low))
            result['ci_high'] = float(max(0.0, ci_high))
            result['ci_width'] = result['ci_high'] - result['ci_low']

        return result

    def _bootstrap_confidence_interval(
        self,
        x: np.ndarray,
        model_name: str
    ) -> Tuple[float, float]:
        """
        Compute bootstrapped confidence interval for a prediction.
        
        Args:
            x: Feature vector (1D array)
            model_name: Model to use for prediction
            
        Returns:
            Tuple of (lower_bound, upper_bound)
        """
        if self.X_train is None or self.y_train is None:
            # Cannot compute CI without training data
            return (0.0, 0.0)

        predictions = []
        n_samples = len(self.X_train)

        for _ in range(self.n_bootstrap):
            # Bootstrap resample
            indices = np.random.choice(n_samples, size=n_samples, replace=True)
            X_boot = self.X_train[indices]
            y_boot = self.y_train[indices]

            # Train model on bootstrap sample
            pipeline = self._create_pipeline(model_name)
            try:
                pipeline.fit(X_boot, y_boot)
                pred = pipeline.predict(x.reshape(1, -1))[0]
                predictions.append(pred)
            except Exception:
                continue

        if not predictions:
            return (0.0, 0.0)

        # Compute percentiles
        alpha = 1 - self.confidence_level
        lower_percentile = (alpha / 2) * 100
        upper_percentile = (1 - alpha / 2) * 100

        ci_low = np.percentile(predictions, lower_percentile)
        ci_high = np.percentile(predictions, upper_percentile)

        return (ci_low, ci_high)

    def predict_batch(
        self,
        features_list: List[Dict],
        return_confidence: bool = False
    ) -> List[Dict]:
        """
        Predict for multiple feature sets efficiently.
        
        Args:
            features_list: List of feature dicts
            return_confidence: Whether to compute confidence intervals
            
        Returns:
            List of prediction dicts
        """
        if not self.pipelines:
            return [{'mean': None, 'model': None}] * len(features_list)

        # Encode all features at once
        X = self._encode_features(features_list)

        # Predict with best model
        pipeline = self.pipelines[self.best_model_name]
        predictions = pipeline.predict(X)

        results = []
        for i, pred in enumerate(predictions):
            result = {
                'mean': float(max(0.0, pred)),
                'model': self.best_model_name
            }

            if return_confidence:
                ci_low, ci_high = self._bootstrap_confidence_interval(
                    X[i], self.best_model_name
                )
                result['ci_low'] = float(max(0.0, ci_low))
                result['ci_high'] = float(max(0.0, ci_high))
                result['ci_width'] = result['ci_high'] - result['ci_low']

            results.append(result)

        return results

    def get_model_info(self) -> Dict:
        """
        Get information about trained models.
        
        Returns:
            Dict with model metadata
        """
        if not self.pipelines:
            return {
                'trained': False,
                'best_model': None,
                'models': {},
                'n_samples': 0,
                'target': None,
                'hardware': None,
                'version': self.version
            }

        return {
            'trained': True,
            'best_model': self.best_model_name,
            'best_score': self.best_model_score,
            'models': self.model_scores,
            'n_samples': len(self.X_train) if self.X_train is not None else 0,
            'n_features': len(self.feature_names),
            'feature_names': self.feature_names,
            'target': self.target_name,
            'hardware': self.hardware_model,
            'version': self.version,
            'confidence_level': self.confidence_level
        }

    def save(self, filepath: Path):
        """
        Save trained model to disk.
        
        Args:
            filepath: Path to save model (will create parent directories)
        """
        filepath.parent.mkdir(parents=True, exist_ok=True)

        model_data = {
            'version': self.version,
            'pipelines': self.pipelines,
            'best_model_name': self.best_model_name,
            'best_model_score': self.best_model_score,
            'model_scores': self.model_scores,
            'feature_names': self.feature_names,
            'target_name': self.target_name,
            'encoder_categories': self.encoder_categories,
            'hardware_model': self.hardware_model,
            'confidence_level': self.confidence_level,
            'n_bootstrap': self.n_bootstrap,
            'cv_folds': self.cv_folds,
            'X_train': self.X_train,
            'y_train': self.y_train,
        }

        with open(filepath, 'wb') as f:
            pickle.dump(model_data, f)

        logger.info(f"Model saved to {filepath}")

    @classmethod
    def load(cls, filepath: Path) -> 'MultivariatePredictor':
        """
        Load trained model from disk.
        
        Args:
            filepath: Path to saved model
            
        Returns:
            Loaded MultivariatePredictor instance
        """
        with open(filepath, 'rb') as f:
            model_data = pickle.load(f)

        # Create instance
        predictor = cls(
            confidence_level=model_data.get('confidence_level', 0.95),
            n_bootstrap=model_data.get('n_bootstrap', 100),
            cv_folds=model_data.get('cv_folds', 5),
        )

        # Restore state
        predictor.version = model_data.get('version', '1.0')
        predictor.pipelines = model_data.get('pipelines', {})
        predictor.best_model_name = model_data.get('best_model_name')
        predictor.best_model_score = model_data.get('best_model_score')
        predictor.model_scores = model_data.get('model_scores', {})
        predictor.feature_names = model_data.get('feature_names', [])
        predictor.target_name = model_data.get('target_name')
        predictor.encoder_categories = model_data.get('encoder_categories', {})
        predictor.hardware_model = model_data.get('hardware_model')
        predictor.X_train = model_data.get('X_train')
        predictor.y_train = model_data.get('y_train')

        logger.info(f"Model loaded from {filepath}")

        return predictor
