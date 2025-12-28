"""Tests for the advisor.cost module."""

import pytest

from advisor.cost import CostModel


class TestCostModel:
    """Tests for CostModel class."""
    
    def test_initialization_default(self):
        """Test default initialization."""
        model = CostModel()
        assert model.energy_cost_per_kwh == 0.0
        assert model.cpu_cost_per_hour == 0.0
        assert model.gpu_cost_per_hour == 0.0
        assert model.currency == 'USD'
    
    def test_initialization_with_values(self):
        """Test initialization with custom values."""
        model = CostModel(
            energy_cost_per_kwh=0.12,
            cpu_cost_per_hour=0.50,
            gpu_cost_per_hour=1.20,
            currency='EUR'
        )
        assert model.energy_cost_per_kwh == 0.12
        assert model.cpu_cost_per_hour == 0.50
        assert model.gpu_cost_per_hour == 1.20
        assert model.currency == 'EUR'
    
    def test_compute_energy_cost(self):
        """Test energy cost computation."""
        model = CostModel(energy_cost_per_kwh=0.12)
        
        scenario = {
            'name': 'Test Scenario',
            'power': {'mean_watts': 100.0},
            'duration': 3600  # 1 hour
        }
        
        cost = model.compute_energy_cost(scenario)
        
        # Expected: (100 / 1000) * (3600 / 3600) * 0.12 = 0.1 * 1 * 0.12 = 0.012
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.012
    
    def test_compute_energy_cost_with_gpu(self):
        """Test energy cost with GPU power included."""
        model = CostModel(energy_cost_per_kwh=0.15)
        
        scenario = {
            'name': 'GPU Scenario',
            'power': {'mean_watts': 80.0},
            'gpu_power': {'mean_watts': 20.0},
            'duration': 3600  # 1 hour
        }
        
        cost = model.compute_energy_cost(scenario)
        
        # Expected: (100 / 1000) * (3600 / 3600) * 0.15 = 0.1 * 1 * 0.15 = 0.015
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.015
    
    def test_compute_energy_cost_partial_hour(self):
        """Test energy cost for partial hour duration."""
        model = CostModel(energy_cost_per_kwh=0.10)
        
        scenario = {
            'name': 'Partial Hour',
            'power': {'mean_watts': 200.0},
            'duration': 1800  # 30 minutes = 0.5 hours
        }
        
        cost = model.compute_energy_cost(scenario)
        
        # Expected: (200 / 1000) * (1800 / 3600) * 0.10 = 0.2 * 0.5 * 0.10 = 0.01
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.01
    
    def test_compute_energy_cost_zero_price(self):
        """Test energy cost returns zero when price is zero."""
        model = CostModel(energy_cost_per_kwh=0.0)
        
        scenario = {
            'name': 'Zero Price',
            'power': {'mean_watts': 100.0},
            'duration': 3600
        }
        
        cost = model.compute_energy_cost(scenario)
        assert cost == 0.0
    
    def test_compute_energy_cost_missing_data(self):
        """Test energy cost returns None when data is missing."""
        model = CostModel(energy_cost_per_kwh=0.12)
        
        # Missing power
        scenario1 = {'name': 'No Power', 'duration': 3600}
        assert model.compute_energy_cost(scenario1) is None
        
        # Missing duration
        scenario2 = {'name': 'No Duration', 'power': {'mean_watts': 100.0}}
        assert model.compute_energy_cost(scenario2) is None
    
    def test_compute_compute_cost(self):
        """Test compute cost calculation."""
        model = CostModel(cpu_cost_per_hour=0.50, gpu_cost_per_hour=1.20)
        
        scenario = {
            'name': 'Compute Test',
            'duration': 3600  # 1 hour
        }
        
        cost = model.compute_compute_cost(scenario)
        
        # Expected: 1 hour * (0.50 + 1.20) = 1.70
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 1.70
    
    def test_compute_compute_cost_cpu_only(self):
        """Test compute cost with CPU only."""
        model = CostModel(cpu_cost_per_hour=0.50)
        
        scenario = {
            'name': 'CPU Only',
            'duration': 7200  # 2 hours
        }
        
        cost = model.compute_compute_cost(scenario)
        
        # Expected: 2 hours * 0.50 = 1.00
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 1.00
    
    def test_compute_compute_cost_zero_price(self):
        """Test compute cost returns zero when prices are zero."""
        model = CostModel(cpu_cost_per_hour=0.0, gpu_cost_per_hour=0.0)
        
        scenario = {'name': 'Zero Price', 'duration': 3600}
        
        cost = model.compute_compute_cost(scenario)
        assert cost == 0.0
    
    def test_compute_total_cost(self):
        """Test total cost computation."""
        model = CostModel(
            energy_cost_per_kwh=0.12,
            cpu_cost_per_hour=0.50,
            gpu_cost_per_hour=1.00
        )
        
        scenario = {
            'name': 'Total Cost Test',
            'power': {'mean_watts': 100.0},
            'duration': 3600  # 1 hour
        }
        
        cost = model.compute_total_cost(scenario)
        
        # Energy: (100/1000) * 1 * 0.12 = 0.012
        # Compute: 1 * (0.50 + 1.00) = 1.50
        # Total: 0.012 + 1.50 = 1.512
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 1.512
    
    def test_compute_cost_per_pixel_single_resolution(self):
        """Test cost per pixel with single resolution."""
        model = CostModel(energy_cost_per_kwh=0.10)
        
        scenario = {
            'name': 'Pixel Cost Test',
            'power': {'mean_watts': 100.0},
            'duration': 60,  # 1 minute
            'resolution': '1920x1080',
            'fps': 30
        }
        
        cost_per_pixel = model.compute_cost_per_pixel(scenario)
        
        # Energy cost: (100/1000) * (60/3600) * 0.10 = 0.1 * 0.01667 * 0.10
        # = 0.0001667
        # Total pixels: 1920 * 1080 * 30 * 60 = 3,732,480,000
        # Cost per pixel: 0.0001667 / 3732480000 = 4.466e-14
        assert cost_per_pixel is not None
        assert pytest.approx(cost_per_pixel, rel=1e-2) == 4.466e-14
    
    def test_compute_cost_per_pixel_multi_output(self):
        """Test cost per pixel with multiple outputs."""
        model = CostModel(energy_cost_per_kwh=0.10)
        
        scenario = {
            'name': 'Multi Output',
            'power': {'mean_watts': 100.0},
            'duration': 60,
            'outputs': [
                {'resolution': '1920x1080', 'fps': 30},
                {'resolution': '1280x720', 'fps': 30}
            ]
        }
        
        cost_per_pixel = model.compute_cost_per_pixel(scenario)
        
        # Total pixels: (1920*1080 + 1280*720) * 30 * 60
        total_pixels = (1920*1080 + 1280*720) * 30 * 60
        energy_cost = (100/1000) * (60/3600) * 0.10
        expected = energy_cost / total_pixels
        
        assert cost_per_pixel is not None
        assert pytest.approx(cost_per_pixel, rel=1e-2) == expected
    
    def test_compute_cost_per_pixel_missing_data(self):
        """Test cost per pixel returns None when data is missing."""
        model = CostModel(energy_cost_per_kwh=0.10)
        
        # Missing resolution
        scenario1 = {
            'name': 'No Resolution',
            'power': {'mean_watts': 100.0},
            'duration': 60
        }
        assert model.compute_cost_per_pixel(scenario1) is None
        
        # Missing cost
        scenario2 = {
            'name': 'No Cost',
            'duration': 60,
            'resolution': '1920x1080',
            'fps': 30
        }
        # Should return None because there's no cost (no power data)
        result = model.compute_cost_per_pixel(scenario2)
        assert result is None
    
    def test_compute_cost_per_watch_hour_single_viewer(self):
        """Test cost per watch hour with single viewer."""
        model = CostModel(energy_cost_per_kwh=0.10)
        
        scenario = {
            'name': 'Watch Hour Test',
            'power': {'mean_watts': 100.0},
            'duration': 3600  # 1 hour
        }
        
        cost_per_watch_hour = model.compute_cost_per_watch_hour(scenario, viewers=1)
        
        # Energy cost: (100/1000) * 1 * 0.10 = 0.01
        # Watch hours: 1 * 1 = 1
        # Cost per watch hour: 0.01 / 1 = 0.01
        assert cost_per_watch_hour is not None
        assert pytest.approx(cost_per_watch_hour, rel=1e-6) == 0.01
    
    def test_compute_cost_per_watch_hour_multiple_viewers(self):
        """Test cost per watch hour with multiple viewers."""
        model = CostModel(energy_cost_per_kwh=0.10)
        
        scenario = {
            'name': 'Multi Viewer Test',
            'power': {'mean_watts': 100.0},
            'duration': 3600  # 1 hour
        }
        
        cost_per_watch_hour = model.compute_cost_per_watch_hour(
            scenario, viewers=100
        )
        
        # Energy cost: 0.01
        # Watch hours: 1 * 100 = 100
        # Cost per watch hour: 0.01 / 100 = 0.0001
        assert cost_per_watch_hour is not None
        assert pytest.approx(cost_per_watch_hour, rel=1e-6) == 0.0001
    
    def test_compute_cost_per_watch_hour_invalid_viewers(self):
        """Test cost per watch hour with invalid viewer count."""
        model = CostModel(energy_cost_per_kwh=0.10)
        
        scenario = {
            'name': 'Invalid Viewers',
            'power': {'mean_watts': 100.0},
            'duration': 3600
        }
        
        # Zero viewers
        assert model.compute_cost_per_watch_hour(scenario, viewers=0) is None
        
        # Negative viewers
        assert model.compute_cost_per_watch_hour(scenario, viewers=-1) is None
    
    def test_get_pricing_info(self):
        """Test getting pricing information."""
        model = CostModel(
            energy_cost_per_kwh=0.12,
            cpu_cost_per_hour=0.50,
            gpu_cost_per_hour=1.20,
            currency='EUR'
        )
        
        info = model.get_pricing_info()
        
        assert info['energy_cost_per_kwh'] == 0.12
        assert info['cpu_cost_per_hour'] == 0.50
        assert info['gpu_cost_per_hour'] == 1.20
        assert info['currency'] == 'EUR'
    
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
    """Integration tests for cost modeling."""
    
    def test_comprehensive_scenario_analysis(self):
        """Test comprehensive cost analysis for a transcoding scenario."""
        model = CostModel(
            energy_cost_per_kwh=0.12,
            cpu_cost_per_hour=0.50,
            gpu_cost_per_hour=1.00,
            currency='USD'
        )
        
        scenario = {
            'name': 'Comprehensive Test',
            'power': {'mean_watts': 150.0},
            'gpu_power': {'mean_watts': 50.0},
            'duration': 1800,  # 30 minutes
            'resolution': '1920x1080',
            'fps': 30
        }
        
        # Compute all metrics
        total_cost = model.compute_total_cost(scenario)
        energy_cost = model.compute_energy_cost(scenario)
        compute_cost = model.compute_compute_cost(scenario)
        cost_per_pixel = model.compute_cost_per_pixel(scenario)
        cost_per_watch_hour = model.compute_cost_per_watch_hour(
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
