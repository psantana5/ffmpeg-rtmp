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
            energy_cost_per_kwh=0.12, cpu_cost_per_hour=0.50, gpu_cost_per_hour=1.20, currency='EUR'
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
            'duration': 3600,  # 1 hour
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
            'duration': 3600,  # 1 hour
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
            'duration': 1800,  # 30 minutes = 0.5 hours
        }

        cost = model.compute_energy_cost(scenario)

        # Expected: (200 / 1000) * (1800 / 3600) * 0.10 = 0.2 * 0.5 * 0.10 = 0.01
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.01

    def test_compute_energy_cost_zero_price(self):
        """Test energy cost returns zero when price is zero."""
        model = CostModel(energy_cost_per_kwh=0.0)

        scenario = {'name': 'Zero Price', 'power': {'mean_watts': 100.0}, 'duration': 3600}

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
            'duration': 3600,  # 1 hour
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
            'duration': 7200,  # 2 hours
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
        model = CostModel(energy_cost_per_kwh=0.12, cpu_cost_per_hour=0.50, gpu_cost_per_hour=1.00)

        scenario = {
            'name': 'Total Cost Test',
            'power': {'mean_watts': 100.0},
            'duration': 3600,  # 1 hour
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
            'fps': 30,
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
                {'resolution': '1280x720', 'fps': 30},
            ],
        }

        cost_per_pixel = model.compute_cost_per_pixel(scenario)

        # Total pixels: (1920*1080 + 1280*720) * 30 * 60
        total_pixels = (1920 * 1080 + 1280 * 720) * 30 * 60
        energy_cost = (100 / 1000) * (60 / 3600) * 0.10
        expected = energy_cost / total_pixels

        assert cost_per_pixel is not None
        assert pytest.approx(cost_per_pixel, rel=1e-2) == expected

    def test_compute_cost_per_pixel_missing_data(self):
        """Test cost per pixel returns None when data is missing."""
        model = CostModel(energy_cost_per_kwh=0.10)

        # Missing resolution
        scenario1 = {'name': 'No Resolution', 'power': {'mean_watts': 100.0}, 'duration': 60}
        assert model.compute_cost_per_pixel(scenario1) is None

        # Missing cost
        scenario2 = {'name': 'No Cost', 'duration': 60, 'resolution': '1920x1080', 'fps': 30}
        # Should return None because there's no cost (no power data)
        result = model.compute_cost_per_pixel(scenario2)
        assert result is None

    def test_compute_cost_per_watch_hour_single_viewer(self):
        """Test cost per watch hour with single viewer."""
        model = CostModel(energy_cost_per_kwh=0.10)

        scenario = {
            'name': 'Watch Hour Test',
            'power': {'mean_watts': 100.0},
            'duration': 3600,  # 1 hour
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
            'duration': 3600,  # 1 hour
        }

        cost_per_watch_hour = model.compute_cost_per_watch_hour(scenario, viewers=100)

        # Energy cost: 0.01
        # Watch hours: 1 * 100 = 100
        # Cost per watch hour: 0.01 / 100 = 0.0001
        assert cost_per_watch_hour is not None
        assert pytest.approx(cost_per_watch_hour, rel=1e-6) == 0.0001

    def test_compute_cost_per_watch_hour_invalid_viewers(self):
        """Test cost per watch hour with invalid viewer count."""
        model = CostModel(energy_cost_per_kwh=0.10)

        scenario = {'name': 'Invalid Viewers', 'power': {'mean_watts': 100.0}, 'duration': 3600}

        # Zero viewers
        assert model.compute_cost_per_watch_hour(scenario, viewers=0) is None

        # Negative viewers
        assert model.compute_cost_per_watch_hour(scenario, viewers=-1) is None

    def test_get_pricing_info(self):
        """Test getting pricing information."""
        model = CostModel(
            energy_cost_per_kwh=0.12, cpu_cost_per_hour=0.50, gpu_cost_per_hour=1.20, currency='EUR'
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
            energy_cost_per_kwh=0.12, cpu_cost_per_hour=0.50, gpu_cost_per_hour=1.00, currency='USD'
        )

        scenario = {
            'name': 'Comprehensive Test',
            'power': {'mean_watts': 150.0},
            'gpu_power': {'mean_watts': 50.0},
            'duration': 1800,  # 30 minutes
            'resolution': '1920x1080',
            'fps': 30,
        }

        # Compute all metrics
        total_cost = model.compute_total_cost(scenario)
        energy_cost = model.compute_energy_cost(scenario)
        compute_cost = model.compute_compute_cost(scenario)
        cost_per_pixel = model.compute_cost_per_pixel(scenario)
        cost_per_watch_hour = model.compute_cost_per_watch_hour(scenario, viewers=10)

        # Verify all metrics are computed
        assert total_cost is not None
        assert energy_cost is not None
        assert compute_cost is not None
        assert cost_per_pixel is not None
        assert cost_per_watch_hour is not None

        # Verify total cost is sum of components
        assert pytest.approx(total_cost, rel=1e-6) == energy_cost + compute_cost


class TestCostModelLoadAware:
    """Tests for load-aware cost calculation methods."""

    def test_load_aware_pricing_initialization(self):
        """Test load-aware pricing parameter initialization."""
        # Explicit load-aware pricing
        model = CostModel(
            price_per_core_second=0.000138889,  # $0.50/hour / 3600
            price_per_joule=3.33e-8,  # $0.12/kWh / 3.6e6
        )
        assert pytest.approx(model.price_per_core_second, rel=1e-6) == 0.000138889
        assert pytest.approx(model.price_per_joule, rel=1e-6) == 3.33e-8

    def test_load_aware_pricing_auto_derivation(self):
        """Test automatic derivation of load-aware pricing from hourly rates."""
        model = CostModel(energy_cost_per_kwh=0.12, cpu_cost_per_hour=0.50)
        # Should auto-derive from hourly rates
        expected_core_second = 0.50 / 3600.0
        expected_joule = 0.12 / 3_600_000.0

        assert pytest.approx(model.price_per_core_second, rel=1e-6) == expected_core_second
        assert pytest.approx(model.price_per_joule, rel=1e-6) == expected_joule

    def test_compute_cost_load_aware(self):
        """Test load-aware compute cost calculation."""
        model = CostModel(
            price_per_core_second=0.0001  # Simple value for testing
        )

        scenario = {
            'name': 'Load Aware Test',
            'cpu_usage_cores': [2.0, 2.5, 3.0, 2.8, 2.2],  # 5 measurements
            'step_seconds': 10,  # 10 seconds between measurements
        }

        cost = model.compute_compute_cost_load_aware(scenario)

        # Expected (trapezoidal): (10/2) * [2.0 + 2*(2.5+3.0+2.8) + 2.2] * 0.0001
        # = 5 * 20.8 * 0.0001 = 0.0104
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.0104

    def test_compute_cost_load_aware_with_gpu(self):
        """Test load-aware compute cost with GPU usage."""
        model = CostModel(price_per_core_second=0.0001)

        scenario = {
            'name': 'GPU Test',
            'cpu_usage_cores': [2.0, 2.0, 2.0],  # 3 measurements
            'gpu_usage_cores': [1.0, 1.0, 1.0],  # 3 measurements
            'step_seconds': 10,
        }

        cost = model.compute_compute_cost_load_aware(scenario)

        # Expected (trapezoidal):
        # CPU: (10/2)*[2.0 + 2*(2.0) + 2.0] = 5*8 = 40
        # GPU: (10/2)*[1.0 + 2*(1.0) + 1.0] = 5*4 = 20
        # Total: (40+20)*0.0001 = 0.006
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.006

    def test_energy_cost_load_aware(self):
        """Test load-aware energy cost calculation from integrated power."""
        model = CostModel(
            price_per_joule=1e-7  # Simple value for testing
        )

        scenario = {
            'name': 'Energy Test',
            'power_watts': [100.0, 150.0, 200.0, 180.0, 120.0],  # 5 measurements
            'step_seconds': 10,
        }

        cost = model.compute_energy_cost_load_aware(scenario)

        # Expected (trapezoidal): (10/2) * [100 + 2*(150+200+180) + 120] * 1e-7
        # = 5 * 1280 * 1e-7 = 0.00064
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.00064

    def test_total_cost_load_aware(self):
        """Test total cost using load-aware calculation."""
        model = CostModel(price_per_core_second=0.0001, price_per_joule=1e-7)

        scenario = {
            'name': 'Total Cost Test',
            'cpu_usage_cores': [2.0, 2.5, 3.0],
            'power_watts': [100.0, 150.0, 200.0],
            'step_seconds': 10,
        }

        cost = model.compute_total_cost_load_aware(scenario)

        # Compute (trapezoidal): (10/2) * [2.0 + 2*(2.5) + 3.0] * 0.0001 = 5 * 10.0 * 0.0001 = 0.005
        # Energy (trapezoidal): (10/2) * [100 + 2*(150) + 200] * 1e-7 = 5 * 600 * 1e-7 = 0.0003
        # Total: 0.005 + 0.0003 = 0.0053
        assert cost is not None
        assert pytest.approx(cost, rel=1e-6) == 0.0053

    def test_load_aware_scales_with_streams(self):
        """Test that cost increases with more streams (higher CPU usage)."""
        model = CostModel(price_per_core_second=0.0001, price_per_joule=1e-7)

        # Single stream scenario
        scenario_1_stream = {
            'name': '1 stream',
            'cpu_usage_cores': [1.0, 1.0, 1.0, 1.0, 1.0],
            'power_watts': [100.0, 100.0, 100.0, 100.0, 100.0],
            'step_seconds': 10,
        }

        # Multiple streams scenario (higher CPU and power)
        scenario_4_streams = {
            'name': '4 streams',
            'cpu_usage_cores': [4.0, 4.0, 4.0, 4.0, 4.0],
            'power_watts': [200.0, 200.0, 200.0, 200.0, 200.0],
            'step_seconds': 10,
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
        model = CostModel(price_per_core_second=0.0001, price_per_joule=1e-7)

        # Low bitrate scenario
        scenario_low_bitrate = {
            'name': 'Low bitrate',
            'cpu_usage_cores': [1.5, 1.5, 1.5, 1.5],
            'power_watts': [110.0, 110.0, 110.0, 110.0],
            'step_seconds': 10,
        }

        # High bitrate scenario (higher CPU and power)
        scenario_high_bitrate = {
            'name': 'High bitrate',
            'cpu_usage_cores': [3.0, 3.0, 3.0, 3.0],
            'power_watts': [150.0, 150.0, 150.0, 150.0],
            'step_seconds': 10,
        }

        cost_low = model.compute_total_cost_load_aware(scenario_low_bitrate)
        cost_high = model.compute_total_cost_load_aware(scenario_high_bitrate)

        assert cost_low is not None
        assert cost_high is not None
        assert cost_high > cost_low

    def test_load_aware_idle_baseline_lowest_cost(self):
        """Test that idle baseline has lowest cost."""
        model = CostModel(price_per_core_second=0.0001, price_per_joule=1e-7)

        # Idle baseline
        scenario_idle = {
            'name': 'Idle',
            'cpu_usage_cores': [0.1, 0.1, 0.1, 0.1, 0.1],
            'power_watts': [50.0, 50.0, 50.0, 50.0, 50.0],
            'step_seconds': 10,
        }

        # Active workload
        scenario_active = {
            'name': 'Active',
            'cpu_usage_cores': [2.0, 2.0, 2.0, 2.0, 2.0],
            'power_watts': [120.0, 120.0, 120.0, 120.0, 120.0],
            'step_seconds': 10,
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
        model = CostModel(price_per_core_second=0.0001, price_per_joule=1e-7)

        scenario = {
            'name': 'No Viewers',
            'cpu_usage_cores': [2.0, 2.0],
            'power_watts': [100.0, 100.0],
            'step_seconds': 10,
            'duration': 20,
            # No 'viewers' field
        }

        # Should return None when no viewer count is provided
        result = model.compute_cost_per_watch_hour_load_aware(scenario)
        assert result is None

        # Should work when viewer count is provided as parameter
        result_with_viewers = model.compute_cost_per_watch_hour_load_aware(scenario, viewers=100)
        assert result_with_viewers is not None

    def test_cost_per_watch_hour_from_scenario(self):
        """Test cost per watch hour using viewer count from scenario."""
        model = CostModel(price_per_core_second=0.0001, price_per_joule=1e-7)

        scenario = {
            'name': 'With Viewers',
            'cpu_usage_cores': [2.0, 2.0],
            'power_watts': [100.0, 100.0],
            'step_seconds': 10,
            'duration': 20,  # 20 seconds
            'viewers': 10,  # 10 viewers
        }

        result = model.compute_cost_per_watch_hour_load_aware(scenario)
        assert result is not None
        assert result > 0.0

    def test_cost_per_pixel_load_aware(self):
        """Test cost per pixel with load-aware calculation."""
        model = CostModel(price_per_core_second=0.0001, price_per_joule=1e-7)

        scenario = {
            'name': 'Pixel Cost Test',
            'cpu_usage_cores': [2.0, 2.0],
            'power_watts': [100.0, 100.0],
            'step_seconds': 10,
            'duration': 20,
            'resolution': '1920x1080',
            'fps': 30,
        }

        cost_per_pixel = model.compute_cost_per_pixel_load_aware(scenario)

        # Should compute cost per pixel
        assert cost_per_pixel is not None
        assert cost_per_pixel > 0.0

    def test_missing_data_returns_none(self):
        """Test that missing data returns None gracefully."""
        model = CostModel(price_per_core_second=0.0001, price_per_joule=1e-7)

        # Missing CPU data
        scenario1 = {'name': 'No CPU', 'power_watts': [100.0], 'step_seconds': 10}
        assert model.compute_compute_cost_load_aware(scenario1) is None

        # Missing power data
        scenario2 = {'name': 'No Power', 'cpu_usage_cores': [2.0], 'step_seconds': 10}
        assert model.compute_energy_cost_load_aware(scenario2) is None

        # Missing step_seconds
        scenario3 = {'name': 'No Step', 'cpu_usage_cores': [2.0], 'power_watts': [100.0]}
        assert model.compute_compute_cost_load_aware(scenario3) is None

    def test_get_pricing_info_includes_load_aware(self):
        """Test that pricing info includes load-aware parameters."""
        model = CostModel(
            energy_cost_per_kwh=0.12,
            cpu_cost_per_hour=0.50,
            price_per_core_second=0.000138889,
            price_per_joule=3.33e-8,
        )

        info = model.get_pricing_info()

        assert 'price_per_core_second' in info
        assert 'price_per_joule' in info
        assert info['price_per_core_second'] == 0.000138889
        assert info['price_per_joule'] == 3.33e-8
