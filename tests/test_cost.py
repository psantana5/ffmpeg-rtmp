"""Tests for the advisor.cost module (load-aware only)."""

import pytest

from advisor.cost import CostModel


class TestCostModelLoadAware:
    """Tests for load-aware CostModel class."""
    
    # Common pricing constants for tests
    # Derived from typical cloud pricing:
    #   CPU: $0.50/hour → 0.000138889 $/core-second
    #   Energy: $0.12/kWh → 3.33e-8 $/joule
    PRICE_PER_CORE_SECOND_TYPICAL = 0.000138889
    PRICE_PER_JOULE_TYPICAL = 3.33e-8
    
    # Simple test values for validation
    PRICE_PER_CORE_SECOND_TEST = 0.0001
    PRICE_PER_JOULE_TEST = 1e-7
    
    def test_initialization_default(self):
        """Test default initialization."""
        model = CostModel()
        assert model.price_per_core_second == 0.0
        assert model.price_per_joule == 0.0
        assert model.currency == 'USD'
    
    def test_initialization_with_values(self):
        """Test initialization with custom values."""
        model = CostModel(
            price_per_core_second=self.PRICE_PER_CORE_SECOND_TYPICAL,
            price_per_joule=self.PRICE_PER_JOULE_TYPICAL,
            currency='EUR'
        )
        assert pytest.approx(model.price_per_core_second, rel=1e-6) == self.PRICE_PER_CORE_SECOND_TYPICAL
        assert pytest.approx(model.price_per_joule, rel=1e-6) == self.PRICE_PER_JOULE_TYPICAL
        assert model.currency == 'EUR'
    
    def test_compute_cost_load_aware(self):
        """Test load-aware compute cost calculation."""
        model = CostModel(
            price_per_core_second=self.PRICE_PER_CORE_SECOND_TEST
        )
        
        scenario = {
            'name': 'Load Aware Test',
            'cpu_usage_cores': [2.0, 2.5, 3.0, 2.8, 2.2],  # 5 measurements
            'step_seconds': 10  # 10 seconds between measurements
        }
        
        cost = model.compute_compute_cost_load_aware(scenario)
        
        # Expected: (2.0 + 2.5 + 3.0 + 2.8 + 2.2) * 10 * 0.0001 = 12.5 * 10 * 0.0001 = 0.0125
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.0125
    
    def test_compute_cost_load_aware_with_gpu(self):
        """Test load-aware compute cost with GPU usage."""
        model = CostModel(
            price_per_core_second=self.PRICE_PER_CORE_SECOND_TEST
        )
        
        scenario = {
            'name': 'GPU Test',
            'cpu_usage_cores': [2.0, 2.0, 2.0],  # 3 measurements
            'gpu_usage_cores': [1.0, 1.0, 1.0],  # 3 measurements
            'step_seconds': 10
        }
        
        cost = model.compute_compute_cost_load_aware(scenario)
        
        # Expected: (2.0+2.0+2.0 + 1.0+1.0+1.0) * 10 * 0.0001 = 9.0 * 10 * 0.0001 = 0.009
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.009
    
    def test_energy_cost_load_aware(self):
        """Test load-aware energy cost calculation from integrated power."""
        model = CostModel(
            price_per_joule=self.PRICE_PER_JOULE_TEST  # Simple value for testing
        )
        
        scenario = {
            'name': 'Energy Test',
            'power_watts': [100.0, 150.0, 200.0, 180.0, 120.0],  # 5 measurements
            'step_seconds': 10
        }
        
        cost = model.compute_energy_cost_load_aware(scenario)
        
        # Expected: (100 + 150 + 200 + 180 + 120) * 10 * 1e-7 = 750 * 10 * 1e-7 = 0.00075
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.00075
    
    def test_total_cost_load_aware(self):
        """Test total cost using load-aware calculation."""
        model = CostModel(
            price_per_core_second=self.PRICE_PER_CORE_SECOND_TEST,
            price_per_joule=self.PRICE_PER_JOULE_TEST
        )
        
        scenario = {
            'name': 'Total Cost Test',
            'cpu_usage_cores': [2.0, 2.5, 3.0],
            'power_watts': [100.0, 150.0, 200.0],
            'step_seconds': 10
        }
        
        cost = model.compute_total_cost_load_aware(scenario)
        
        # Compute: (2.0 + 2.5 + 3.0) * 10 * 0.0001 = 0.0075
        # Energy: (100 + 150 + 200) * 10 * 1e-7 = 0.00045
        # Total: 0.0075 + 0.00045 = 0.00795
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.00795
    
    def test_load_aware_scales_with_streams(self):
        """Test that cost increases with more streams (higher CPU usage)."""
        model = CostModel(
            price_per_core_second=self.PRICE_PER_CORE_SECOND_TEST,
            price_per_joule=self.PRICE_PER_JOULE_TEST
        )
        
        # Single stream scenario
        scenario_1_stream = {
            'name': '1 stream',
            'cpu_usage_cores': [1.0, 1.0, 1.0, 1.0, 1.0],
            'power_watts': [100.0, 100.0, 100.0, 100.0, 100.0],
            'step_seconds': 10
        }
        
        # Multiple streams scenario (higher CPU and power)
        scenario_4_streams = {
            'name': '4 streams',
            'cpu_usage_cores': [4.0, 4.0, 4.0, 4.0, 4.0],
            'power_watts': [200.0, 200.0, 200.0, 200.0, 200.0],
            'step_seconds': 10
        }
        
        cost_1 = model.compute_total_cost_load_aware(scenario_1_stream)
        cost_4 = model.compute_total_cost_load_aware(scenario_4_streams)
        
        assert cost_1 is not None
        assert cost_4 is not None
        assert cost_4 > cost_1
        # Cost should scale roughly with stream count
        assert cost_4 / cost_1 > 3.0  # At least 3x more expensive
    
    def test_load_aware_scales_with_bitrate(self):
        """Test that cost increases with higher bitrate (higher CPU usage)."""
        model = CostModel(
            price_per_core_second=self.PRICE_PER_CORE_SECOND_TEST,
            price_per_joule=self.PRICE_PER_JOULE_TEST
        )
        
        # Low bitrate scenario
        scenario_low_bitrate = {
            'name': 'Low bitrate',
            'cpu_usage_cores': [1.5, 1.5, 1.5, 1.5],
            'power_watts': [110.0, 110.0, 110.0, 110.0],
            'step_seconds': 10
        }
        
        # High bitrate scenario (higher CPU and power)
        scenario_high_bitrate = {
            'name': 'High bitrate',
            'cpu_usage_cores': [3.0, 3.0, 3.0, 3.0],
            'power_watts': [150.0, 150.0, 150.0, 150.0],
            'step_seconds': 10
        }
        
        cost_low = model.compute_total_cost_load_aware(scenario_low_bitrate)
        cost_high = model.compute_total_cost_load_aware(scenario_high_bitrate)
        
        assert cost_low is not None
        assert cost_high is not None
        assert cost_high > cost_low
    
    def test_load_aware_idle_baseline_lowest_cost(self):
        """Test that idle baseline has lowest cost."""
        model = CostModel(
            price_per_core_second=self.PRICE_PER_CORE_SECOND_TEST,
            price_per_joule=self.PRICE_PER_JOULE_TEST
        )
        
        # Idle baseline
        scenario_idle = {
            'name': 'Idle',
            'cpu_usage_cores': [0.1, 0.1, 0.1, 0.1, 0.1],
            'power_watts': [50.0, 50.0, 50.0, 50.0, 50.0],
            'step_seconds': 10
        }
        
        # Active workload
        scenario_active = {
            'name': 'Active',
            'cpu_usage_cores': [2.0, 2.0, 2.0, 2.0, 2.0],
            'power_watts': [120.0, 120.0, 120.0, 120.0, 120.0],
            'step_seconds': 10
        }
        
        cost_idle = model.compute_total_cost_load_aware(scenario_idle)
        cost_active = model.compute_total_cost_load_aware(scenario_active)
        
        assert cost_idle is not None
        assert cost_active is not None
        assert cost_idle < cost_active
        # Idle should be significantly cheaper
        assert cost_idle / cost_active < 0.5
    
    def test_cost_per_watch_hour_no_hardcoded_viewers(self):
        """Test that cost per watch hour requires viewer count (not hardcoded)."""
        model = CostModel(
            price_per_core_second=self.PRICE_PER_CORE_SECOND_TEST,
            price_per_joule=self.PRICE_PER_JOULE_TEST
        )
        
        scenario = {
            'name': 'No Viewers',
            'cpu_usage_cores': [2.0, 2.0],
            'power_watts': [100.0, 100.0],
            'step_seconds': 10,
            'duration': 20
            # No 'viewers' field
        }
        
        # Should return None when no viewer count is provided
        result = model.compute_cost_per_watch_hour_load_aware(scenario)
        assert result is None
        
        # Should work when viewer count is provided as parameter
        result_with_viewers = model.compute_cost_per_watch_hour_load_aware(
            scenario, viewers=100
        )
        assert result_with_viewers is not None
    
    def test_cost_per_watch_hour_from_scenario(self):
        """Test cost per watch hour using viewer count from scenario."""
        model = CostModel(
            price_per_core_second=self.PRICE_PER_CORE_SECOND_TEST,
            price_per_joule=self.PRICE_PER_JOULE_TEST
        )
        
        scenario = {
            'name': 'With Viewers',
            'cpu_usage_cores': [2.0, 2.0],
            'power_watts': [100.0, 100.0],
            'step_seconds': 10,
            'duration': 20,  # 20 seconds
            'viewers': 10  # 10 viewers
        }
        
        result = model.compute_cost_per_watch_hour_load_aware(scenario)
        assert result is not None
        assert result > 0.0
    
    def test_cost_per_pixel_load_aware(self):
        """Test cost per pixel with load-aware calculation."""
        model = CostModel(
            price_per_core_second=self.PRICE_PER_CORE_SECOND_TEST,
            price_per_joule=self.PRICE_PER_JOULE_TEST
        )
        
        scenario = {
            'name': 'Pixel Cost Test',
            'cpu_usage_cores': [2.0, 2.0],
            'power_watts': [100.0, 100.0],
            'step_seconds': 10,
            'duration': 20,
            'resolution': '1920x1080',
            'fps': 30
        }
        
        cost_per_pixel = model.compute_cost_per_pixel_load_aware(scenario)
        
        # Should compute cost per pixel
        assert cost_per_pixel is not None
        assert cost_per_pixel > 0.0
    
    def test_missing_data_returns_none(self):
        """Test that missing data returns None gracefully."""
        model = CostModel(
            price_per_core_second=self.PRICE_PER_CORE_SECOND_TEST,
            price_per_joule=self.PRICE_PER_JOULE_TEST
        )
        
        # Missing CPU data
        scenario1 = {
            'name': 'No CPU',
            'power_watts': [100.0],
            'step_seconds': 10
        }
        assert model.compute_compute_cost_load_aware(scenario1) is None
        
        # Missing power data
        scenario2 = {
            'name': 'No Power',
            'cpu_usage_cores': [2.0],
            'step_seconds': 10
        }
        assert model.compute_energy_cost_load_aware(scenario2) is None
        
        # Missing step_seconds
        scenario3 = {
            'name': 'No Step',
            'cpu_usage_cores': [2.0],
            'power_watts': [100.0]
        }
        assert model.compute_compute_cost_load_aware(scenario3) is None
    
    def test_get_pricing_info_includes_load_aware(self):
        """Test that pricing info includes load-aware parameters."""
        model = CostModel(
            price_per_core_second=0.000138889,
            price_per_joule=3.33e-8,
            currency='USD'
        )
        
        info = model.get_pricing_info()
        
        assert 'price_per_core_second' in info
        assert 'price_per_joule' in info
        assert 'currency' in info
        assert info['price_per_core_second'] == 0.000138889
        assert info['price_per_joule'] == 3.33e-8
        assert info['currency'] == 'USD'
    
    def test_parse_resolution(self):
        """Test resolution parsing."""
        model = CostModel()
        
        # Valid resolutions
        assert model._parse_resolution('1920x1080') == (1920, 1080)
        assert model._parse_resolution('1280x720') == (1280, 720)
        assert model._parse_resolution('854x480') == (854, 480)
        
        # Invalid resolutions
        assert model._parse_resolution('N/A') == (None, None)
        assert model._parse_resolution('invalid') == (None, None)
        assert model._parse_resolution('') == (None, None)


class TestCostModelIntegration:
    """Integration tests for load-aware cost modeling."""
    
    def test_comprehensive_scenario_analysis(self):
        """Test comprehensive cost analysis for a transcoding scenario."""
        # Use typical cloud pricing: $0.50/hour CPU, $0.12/kWh energy
        model = CostModel(
            price_per_core_second=0.000138889,  # $0.50/hour / 3600
            price_per_joule=3.33e-8,  # $0.12/kWh / 3.6e6
            currency='USD'
        )
        
        scenario = {
            'name': 'Comprehensive Test',
            'cpu_usage_cores': [2.5, 2.8, 3.0, 2.7, 2.6],
            'power_watts': [150.0, 155.0, 160.0, 158.0, 152.0],
            'step_seconds': 5,
            'duration': 25,  # 5 measurements * 5 seconds
            'resolution': '1920x1080',
            'fps': 30
        }
        
        # Compute all metrics
        total_cost = model.compute_total_cost_load_aware(scenario)
        energy_cost = model.compute_energy_cost_load_aware(scenario)
        compute_cost = model.compute_compute_cost_load_aware(scenario)
        cost_per_pixel = model.compute_cost_per_pixel_load_aware(scenario)
        cost_per_watch_hour = model.compute_cost_per_watch_hour_load_aware(
            scenario, viewers=10
        )
        
        # Verify all metrics are computed
        assert total_cost is not None
        assert energy_cost is not None
        assert compute_cost is not None
        assert cost_per_pixel is not None
        assert cost_per_watch_hour is not None
        
        # Verify total cost is sum of components
        assert pytest.approx(total_cost, rel=1e-6) == energy_cost + compute_cost
