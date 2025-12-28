"""Tests for scripts.utils.prometheus_client module."""

import pytest
from unittest.mock import Mock, patch

from scripts.utils.prometheus_client import PrometheusClient


class TestPrometheusClient:
    """Tests for PrometheusClient class."""
    
    def test_initialization(self):
        """Test PrometheusClient initialization."""
        client = PrometheusClient('http://localhost:9090')
        assert client.base_url == 'http://localhost:9090'
    
    def test_initialization_strips_trailing_slash(self):
        """Test that trailing slash is removed from base_url."""
        client = PrometheusClient('http://localhost:9090/')
        assert client.base_url == 'http://localhost:9090'
    
    @patch('scripts.utils.prometheus_client.requests.get')
    def test_query_success(self, mock_get):
        """Test successful instant query."""
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            'status': 'success',
            'data': {
                'result': [
                    {'metric': {}, 'value': [1234567890, '42.0']}
                ]
            }
        }
        mock_get.return_value = mock_response
        
        client = PrometheusClient()
        result = client.query('up')
        
        assert result is not None
        assert result['status'] == 'success'
        mock_get.assert_called_once()
    
    @patch('scripts.utils.prometheus_client.requests.get')
    def test_query_with_timestamp(self, mock_get):
        """Test instant query with timestamp."""
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            'status': 'success',
            'data': {'result': []}
        }
        mock_get.return_value = mock_response
        
        client = PrometheusClient()
        client.query('up', ts=1234567890.0)
        
        call_kwargs = mock_get.call_args[1]
        assert call_kwargs['params']['time'] == 1234567890
    
    @patch('scripts.utils.prometheus_client.requests.get')
    def test_query_failure(self, mock_get):
        """Test query with error response."""
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            'status': 'error',
            'error': 'query parse error'
        }
        mock_get.return_value = mock_response
        
        client = PrometheusClient()
        result = client.query('invalid{query')
        
        assert result is None
    
    @patch('scripts.utils.prometheus_client.requests.get')
    def test_query_timeout(self, mock_get):
        """Test query timeout handling."""
        import requests
        mock_get.side_effect = requests.exceptions.Timeout()
        
        client = PrometheusClient()
        result = client.query('up')
        
        assert result is None
    
    @patch('scripts.utils.prometheus_client.requests.get')
    def test_query_range_success(self, mock_get):
        """Test successful range query."""
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            'status': 'success',
            'data': {
                'result': [
                    {
                        'metric': {},
                        'values': [
                            [1234567890, '42.0'],
                            [1234567900, '43.0']
                        ]
                    }
                ]
            }
        }
        mock_get.return_value = mock_response
        
        client = PrometheusClient()
        result = client.query_range('up', start=1234567890, end=1234567900)
        
        assert result is not None
        assert result['status'] == 'success'
        mock_get.assert_called_once()
    
    @patch('scripts.utils.prometheus_client.requests.get')
    def test_get_targets_success(self, mock_get):
        """Test getting targets list."""
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            'status': 'success',
            'data': {
                'activeTargets': [
                    {'scrapeUrl': 'http://localhost:8080/metrics', 'health': 'up'}
                ]
            }
        }
        mock_get.return_value = mock_response
        
        client = PrometheusClient()
        targets = client.get_targets()
        
        assert targets is not None
        assert len(targets) == 1
        assert targets[0]['health'] == 'up'
    
    @patch('scripts.utils.prometheus_client.requests.get')
    def test_health_check_success(self, mock_get):
        """Test health check when server is healthy."""
        mock_response = Mock()
        mock_response.status_code = 200
        mock_get.return_value = mock_response
        
        client = PrometheusClient()
        is_healthy = client.health_check()
        
        assert is_healthy is True
    
    @patch('scripts.utils.prometheus_client.requests.get')
    def test_health_check_failure(self, mock_get):
        """Test health check when server is unhealthy."""
        import requests
        mock_get.side_effect = requests.exceptions.ConnectionError()
        
        client = PrometheusClient()
        is_healthy = client.health_check()
        
        assert is_healthy is False
    
    def test_extract_values_instant_query(self):
        """Test extracting values from instant query response."""
        client = PrometheusClient()
        
        data = {
            'data': {
                'result': [
                    {'value': [1234567890, '42.0']},
                    {'value': [1234567890, '43.5']}
                ]
            }
        }
        
        values = client.extract_values(data)
        
        assert len(values) == 2
        assert values[0] == 42.0
        assert values[1] == 43.5
    
    def test_extract_values_range_query(self):
        """Test extracting values from range query response."""
        client = PrometheusClient()
        
        data = {
            'data': {
                'result': [
                    {
                        'values': [
                            [1234567890, '42.0'],
                            [1234567900, '43.5'],
                            [1234567910, '44.0']
                        ]
                    }
                ]
            }
        }
        
        values = client.extract_values(data)
        
        assert len(values) == 3
        assert values[0] == 42.0
        assert values[1] == 43.5
        assert values[2] == 44.0
    
    def test_extract_values_empty_response(self):
        """Test extracting values from empty response."""
        client = PrometheusClient()
        
        data = {'data': {'result': []}}
        values = client.extract_values(data)
        
        assert len(values) == 0
    
    def test_extract_values_none(self):
        """Test extracting values from None."""
        client = PrometheusClient()
        values = client.extract_values(None)
        
        assert len(values) == 0
    
    def test_extract_values_invalid_format(self):
        """Test extracting values with invalid value format."""
        client = PrometheusClient()
        
        data = {
            'data': {
                'result': [
                    {'value': [1234567890, 'invalid']},  # Invalid number
                    {'value': [1234567890, '42.0']}      # Valid
                ]
            }
        }
        
        values = client.extract_values(data)
        
        # Should skip invalid and return valid
        assert len(values) == 1
        assert values[0] == 42.0
