"""
Tests for data quality improvements: outlier detection, hardware metadata, etc.
"""

import numpy as np

from analyze_results import filter_outliers_iqr, get_hardware_metadata


class TestOutlierDetection:
    """Test outlier detection and filtering"""
    
    def test_filter_outliers_iqr_no_outliers(self):
        """Test that clean data is not filtered"""
        values = [10.0, 11.0, 12.0, 13.0, 14.0, 15.0]
        filtered, removed = filter_outliers_iqr(values)
        
        assert removed == 0
        assert len(filtered) == len(values)
    
    def test_filter_outliers_iqr_with_outliers(self):
        """Test that outliers are correctly identified and removed"""
        # Normal distribution around 100W with outliers
        values = [98.0, 99.0, 100.0, 101.0, 102.0, 103.0, 200.0, 5.0]
        filtered, removed = filter_outliers_iqr(values)
        
        assert removed == 2  # Should remove 200.0 and 5.0
        assert 200.0 not in filtered
        assert 5.0 not in filtered
        assert 100.0 in filtered
    
    def test_filter_outliers_iqr_insufficient_data(self):
        """Test that insufficient data is not filtered"""
        values = [10.0, 11.0, 12.0]  # Less than 4 values
        filtered, removed = filter_outliers_iqr(values)
        
        assert removed == 0
        assert len(filtered) == len(values)
    
    def test_filter_outliers_iqr_custom_factor(self):
        """Test outlier detection with custom IQR factor"""
        values = [98.0, 99.0, 100.0, 101.0, 102.0, 120.0]
        
        # With default factor (1.5), 120 might be an outlier
        filtered_default, removed_default = filter_outliers_iqr(values, factor=1.5)
        
        # With more permissive factor (3.0), 120 should not be an outlier
        filtered_permissive, removed_permissive = filter_outliers_iqr(values, factor=3.0)
        
        assert removed_permissive <= removed_default
    
    def test_filter_outliers_iqr_power_measurement_scenario(self):
        """Test realistic power measurement scenario with RAPL noise"""
        # Simulate power measurements with occasional spikes
        base_power = 150.0
        normal_values = [base_power + np.random.normal(0, 2) for _ in range(50)]
        # Add some spikes (RAPL read errors, thermal throttling, etc.)
        noisy_values = normal_values + [200.0, 250.0, 90.0]
        
        filtered, removed = filter_outliers_iqr(noisy_values)
        
        # Should remove the spikes
        assert removed >= 2
        assert all(140 < v < 160 for v in filtered)
    
    def test_filter_outliers_preserves_order(self):
        """Test that filtering preserves relative order of values"""
        values = [10.0, 11.0, 100.0, 12.0, 13.0, 14.0]  # 100.0 is outlier
        filtered, removed = filter_outliers_iqr(values)
        
        # Check that non-outlier values maintain their relative order
        non_outliers = [v for v in values if v != 100.0]
        assert filtered == sorted(non_outliers)  # Should be in ascending order


class TestHardwareMetadata:
    """Test hardware metadata capture"""
    
    def test_get_hardware_metadata_returns_dict(self):
        """Test that hardware metadata returns a dictionary"""
        metadata = get_hardware_metadata()
        
        assert isinstance(metadata, dict)
    
    def test_get_hardware_metadata_has_required_fields(self):
        """Test that metadata contains required fields"""
        metadata = get_hardware_metadata()
        
        required_fields = [
            'cpu_model',
            'cpu_count',
            'cpu_freq_mhz',
            'ffmpeg_version',
            'kernel_version'
        ]
        
        for field in required_fields:
            assert field in metadata
    
    def test_get_hardware_metadata_cpu_model(self):
        """Test CPU model extraction"""
        metadata = get_hardware_metadata()
        
        # Should at least not be empty
        assert metadata['cpu_model'] is not None
        # Should be string
        assert isinstance(metadata['cpu_model'], str)
    
    def test_get_hardware_metadata_cpu_count(self):
        """Test CPU count is reasonable"""
        metadata = get_hardware_metadata()
        
        if metadata['cpu_count'] is not None:
            assert isinstance(metadata['cpu_count'], int)
            assert metadata['cpu_count'] > 0
            assert metadata['cpu_count'] <= 256  # Reasonable upper bound
    
    def test_get_hardware_metadata_graceful_failures(self):
        """Test that metadata capture handles missing information gracefully"""
        metadata = get_hardware_metadata()
        
        # All fields should be present, even if they're 'unknown' or None
        assert 'cpu_model' in metadata
        assert 'ffmpeg_version' in metadata
        assert 'kernel_version' in metadata
        
        # These should have reasonable values or None
        for key, value in metadata.items():
            assert value is not None or key in ['cpu_freq_mhz', 'cpu_count']
