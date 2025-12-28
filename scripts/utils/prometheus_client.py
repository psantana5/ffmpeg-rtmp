#!/usr/bin/env python3
"""
Prometheus client utility for querying metrics.

Provides a reusable client for querying Prometheus with proper error handling
and logging aligned with project style.
"""

import logging
from typing import Dict, List, Optional

import requests

logger = logging.getLogger(__name__)


class PrometheusClient:
    """Client for querying Prometheus API."""
    
    def __init__(self, base_url: str = 'http://localhost:9090'):
        """
        Initialize Prometheus client.
        
        Args:
            base_url: Prometheus server base URL
        """
        self.base_url = base_url.rstrip('/')
        logger.debug(f"PrometheusClient initialized with base_url={self.base_url}")
    
    def query(self, query: str, ts: Optional[float] = None) -> Optional[Dict]:
        """
        Execute instant query (optionally at a specific unix timestamp).
        
        Args:
            query: PromQL query string
            ts: Optional unix timestamp for point-in-time query
            
        Returns:
            Query response dict or None on error
        """
        url = f"{self.base_url}/api/v1/query"
        params = {'query': query}
        if ts is not None:
            params['time'] = int(ts)
        
        try:
            response = requests.get(url, params=params, timeout=30)
            response.raise_for_status()
            data = response.json()
            
            if data.get('status') != 'success':
                logger.error(f"Prometheus query failed: {data.get('error', 'Unknown error')}")
                return None
            
            return data
        
        except requests.exceptions.Timeout:
            logger.error(f"Query timeout: {query}")
            return None
        except requests.exceptions.RequestException as e:
            logger.error(f"Error querying Prometheus: {e}")
            return None
        except Exception as e:
            logger.error(f"Unexpected error during query: {e}")
            return None
    
    def query_range(
        self,
        query: str,
        start: float,
        end: float,
        step: str = '15s'
    ) -> Optional[Dict]:
        """
        Execute range query.
        
        Args:
            query: PromQL query string
            start: Start timestamp (unix time)
            end: End timestamp (unix time)
            step: Query resolution step
            
        Returns:
            Query response dict or None on error
        """
        url = f"{self.base_url}/api/v1/query_range"
        params = {
            'query': query,
            'start': int(start),
            'end': int(end),
            'step': step
        }
        
        try:
            response = requests.get(url, params=params, timeout=30)
            response.raise_for_status()
            data = response.json()
            
            if data.get('status') != 'success':
                logger.error(f"Prometheus range query failed: {data.get('error', 'Unknown error')}")
                return None
            
            return data
        
        except requests.exceptions.Timeout:
            logger.error(f"Range query timeout: {query}")
            return None
        except requests.exceptions.RequestException as e:
            logger.error(f"Error querying Prometheus range: {e}")
            return None
        except Exception as e:
            logger.error(f"Unexpected error during range query: {e}")
            return None
    
    def get_targets(self) -> Optional[List[Dict]]:
        """
        Get list of Prometheus scrape targets.
        
        Returns:
            List of target dicts or None on error
        """
        url = f"{self.base_url}/api/v1/targets"
        
        try:
            response = requests.get(url, timeout=10)
            response.raise_for_status()
            data = response.json()
            
            if data.get('status') != 'success':
                logger.error(f"Failed to get targets: {data.get('error', 'Unknown error')}")
                return None
            
            return data.get('data', {}).get('activeTargets', [])
        
        except requests.exceptions.RequestException as e:
            logger.error(f"Error getting Prometheus targets: {e}")
            return None
        except Exception as e:
            logger.error(f"Unexpected error getting targets: {e}")
            return None
    
    def health_check(self) -> bool:
        """
        Check if Prometheus server is healthy.
        
        Returns:
            True if server is reachable and healthy, False otherwise
        """
        url = f"{self.base_url}/-/healthy"
        
        try:
            response = requests.get(url, timeout=5)
            return response.status_code == 200
        except Exception:
            return False
    
    def extract_values(self, data: Optional[Dict]) -> List[float]:
        """
        Extract numeric values from query response.
        
        Args:
            data: Prometheus query response
            
        Returns:
            List of float values
        """
        if not data or 'data' not in data or 'result' not in data['data']:
            return []
        
        values = []
        results = data['data']['result']
        
        for result in results:
            if 'values' in result:
                # Range query
                for timestamp, value in result['values']:
                    try:
                        values.append(float(value))
                    except (ValueError, TypeError):
                        continue
            elif 'value' in result:
                # Instant query
                try:
                    values.append(float(result['value'][1]))
                except (ValueError, TypeError, IndexError):
                    continue
        
        return values
