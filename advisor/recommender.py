"""
Transcoding Configuration Recommender

Ranks transcoding configurations by energy efficiency and provides
actionable recommendations for selecting optimal pipelines.

This module bridges measurement and decision-making:
- Accepts analyzed scenario data
- Applies scoring algorithms
- Ranks configurations
- Returns best options with justification
- Supports output ladder grouping for fair comparison

Designed for:
- Single-machine optimization (developer workstation)
- Cloud/bare-metal deployment planning (future)
- Sustainable streaming infrastructure
"""

import logging
from collections import defaultdict
from typing import Dict, List, Optional

from .scoring import EnergyEfficiencyScorer

logger = logging.getLogger(__name__)


class TranscodingRecommender:
    """
    Recommends optimal transcoding configurations based on energy efficiency analysis.
    
    This class:
    1. Accepts analyzed scenarios (from ResultsAnalyzer)
    2. Computes efficiency scores using EnergyEfficiencyScorer
    3. Ranks scenarios by score
    4. Returns best configuration and top N alternatives
    
    Design principles:
    - Immutable inputs: does not modify scenario data in-place
    - Transparent ranking: provides justification for recommendations
    - Production-ready: handles missing data, edge cases gracefully
    """

    def __init__(self, scorer: Optional[EnergyEfficiencyScorer] = None):
        """
        Initialize the recommender.
        
        Args:
            scorer: EnergyEfficiencyScorer instance. If None, uses default scorer.
        """
        self.scorer = scorer or EnergyEfficiencyScorer()

    def analyze_and_rank(self, scenarios: List[Dict]) -> List[Dict]:
        """
        Compute efficiency scores and rank scenarios.
        
        Args:
            scenarios: List of scenario dicts (from ResultsAnalyzer.generate_report())
            
        Returns:
            List of scenarios sorted by efficiency_score (descending).
            Each scenario dict is augmented with:
                - 'efficiency_score': float or None
                - 'efficiency_rank': int (1 = best, only for scenarios with scores)
        
        Example:
            >>> recommender = TranscodingRecommender()
            >>> ranked = recommender.analyze_and_rank(scenarios)
            >>> best = ranked[0]  # Highest efficiency
            >>> print(f"Best config: {best['name']} (score: {best['efficiency_score']:.4f})")
        """
        # Compute scores for all scenarios
        scored_scenarios = []
        for scenario in scenarios:
            # Create a copy to avoid mutating input
            scenario_copy = scenario.copy()
            score = self.scorer.compute_score(scenario_copy)
            scenario_copy['efficiency_score'] = score
            scored_scenarios.append(scenario_copy)

        # Separate scenarios with valid scores from those without
        with_scores = [s for s in scored_scenarios if s['efficiency_score'] is not None]
        without_scores = [s for s in scored_scenarios if s['efficiency_score'] is None]

        # Sort by score (descending: higher efficiency is better)
        with_scores.sort(key=lambda s: s['efficiency_score'], reverse=True)

        # Assign ranks to scored scenarios
        for rank, scenario in enumerate(with_scores, start=1):
            scenario['efficiency_rank'] = rank

        # Combine: ranked scenarios first, then unscored ones
        return with_scores + without_scores

    def analyze_and_rank_by_ladder(self, scenarios: List[Dict]) -> Dict[str, List[Dict]]:
        """
        Compute efficiency scores and rank scenarios, grouped by output ladder.
        
        This method groups scenarios by their output ladder configuration and ranks
        within each group. Only scenarios with identical output ladders are compared.
        
        Args:
            scenarios: List of scenario dicts
            
        Returns:
            Dict mapping ladder identifier to list of ranked scenarios.
            Each scenario dict is augmented with:
                - 'efficiency_score': float or None
                - 'efficiency_rank': int (1 = best within ladder)
                - 'output_ladder': str (ladder identifier)
        
        Example:
            >>> recommender = TranscodingRecommender()
            >>> by_ladder = recommender.analyze_and_rank_by_ladder(scenarios)
            >>> for ladder, ranked_scenarios in by_ladder.items():
            >>>     print(f"Ladder: {ladder}")
            >>>     print(f"  Best: {ranked_scenarios[0]['name']}")
        """
        # Group scenarios by output ladder
        ladder_groups = defaultdict(list)

        for scenario in scenarios:
            # Create a copy to avoid mutating input
            scenario_copy = scenario.copy()

            # Get output ladder
            ladder = self.scorer.get_output_ladder(scenario_copy)
            scenario_copy['output_ladder'] = ladder

            # Compute efficiency score
            score = self.scorer.compute_score(scenario_copy)
            scenario_copy['efficiency_score'] = score

            # Group by ladder (None for scenarios without ladder)
            ladder_key = ladder if ladder else '_no_ladder_'
            ladder_groups[ladder_key].append(scenario_copy)

        # Rank within each ladder group
        ranked_by_ladder = {}
        for ladder_key, group_scenarios in ladder_groups.items():
            # Separate scenarios with valid scores
            with_scores = [s for s in group_scenarios if s['efficiency_score'] is not None]
            without_scores = [s for s in group_scenarios if s['efficiency_score'] is None]

            # Sort by score (descending: higher efficiency is better)
            with_scores.sort(key=lambda s: s['efficiency_score'], reverse=True)

            # Assign ranks within this ladder
            for rank, scenario in enumerate(with_scores, start=1):
                scenario['efficiency_rank'] = rank

            # Combine: ranked scenarios first, then unscored ones
            ranked_by_ladder[ladder_key] = with_scores + without_scores

        return ranked_by_ladder

    def get_best_per_ladder(self, scenarios: List[Dict]) -> Dict[str, Optional[Dict]]:
        """
        Get the best configuration for each output ladder.
        
        Args:
            scenarios: List of scenario dicts
            
        Returns:
            Dict mapping ladder identifier to best scenario for that ladder.
        
        Example:
            >>> recommender = TranscodingRecommender()
            >>> best_per_ladder = recommender.get_best_per_ladder(scenarios)
            >>> for ladder, best in best_per_ladder.items():
            >>>     if best:
            >>>         print(f"{ladder}: {best['name']} - {best['efficiency_score']:.4f}")
        """
        by_ladder = self.analyze_and_rank_by_ladder(scenarios)

        best_per_ladder = {}
        for ladder_key, ranked_scenarios in by_ladder.items():
            if ranked_scenarios and ranked_scenarios[0].get('efficiency_score') is not None:
                best_per_ladder[ladder_key] = ranked_scenarios[0]
            else:
                best_per_ladder[ladder_key] = None

        return best_per_ladder

    def get_best_configuration(self, scenarios: List[Dict]) -> Optional[Dict]:
        """
        Get the single best transcoding configuration.
        
        Args:
            scenarios: List of scenario dicts
            
        Returns:
            Best scenario dict (highest efficiency_score) or None if no valid scores.
            
        Example:
            >>> best = recommender.get_best_configuration(scenarios)
            >>> if best:
            >>>     print(f"Recommended: {best['name']}")
            >>>     print(f"Efficiency: {best['efficiency_score']:.4f} Mbps/W")
            >>>     print(f"Power: {best['power']['mean_watts']:.2f} W")
        """
        ranked = self.analyze_and_rank(scenarios)

        if not ranked or ranked[0]['efficiency_score'] is None:
            logger.warning("No valid configurations with efficiency scores found")
            return None

        return ranked[0]

    def get_top_n(self, scenarios: List[Dict], n: int = 5) -> List[Dict]:
        """
        Get top N transcoding configurations by efficiency.
        
        Args:
            scenarios: List of scenario dicts
            n: Number of top configurations to return
            
        Returns:
            List of top N scenarios sorted by efficiency (descending).
            May return fewer than N if insufficient data.
            
        Example:
            >>> top_5 = recommender.get_top_n(scenarios, n=5)
            >>> for i, config in enumerate(top_5, start=1):
            >>>     print(f"{i}. {config['name']}: {config['efficiency_score']:.4f} Mbps/W")
        """
        ranked = self.analyze_and_rank(scenarios)

        # Return only scenarios with valid scores
        with_scores = [s for s in ranked if s['efficiency_score'] is not None]

        return with_scores[:n]

    def get_recommendation_summary(self, scenarios: List[Dict]) -> Dict:
        """
        Generate a comprehensive recommendation summary.
        
        Args:
            scenarios: List of scenario dicts
            
        Returns:
            Summary dict containing:
                - 'best_config': Best configuration dict or None
                - 'top_5': List of top 5 configurations
                - 'total_analyzed': Total number of scenarios
                - 'scorable': Number of scenarios with efficiency scores
                - 'efficiency_range': (min, max) efficiency scores or None
                - 'power_range': (min, max) mean_watts or None
        
        Example:
            >>> summary = recommender.get_recommendation_summary(scenarios)
            >>> print(f"Analyzed {summary['total_analyzed']} configurations")
            >>> print(f"Best: {summary['best_config']['name']}")
            >>> print(f"Efficiency range: {summary['efficiency_range']}")
        """
        ranked = self.analyze_and_rank(scenarios)
        with_scores = [s for s in ranked if s['efficiency_score'] is not None]

        best_config = with_scores[0] if with_scores else None
        top_5 = with_scores[:5]

        efficiency_range = None
        if with_scores:
            efficiency_scores = [s['efficiency_score'] for s in with_scores]
            efficiency_range = (min(efficiency_scores), max(efficiency_scores))

        power_range = None
        with_power = [
            s for s in scenarios
            if s.get('power') and s['power'].get('mean_watts') is not None
        ]
        if with_power:
            power_values = [s['power']['mean_watts'] for s in with_power]
            power_range = (min(power_values), max(power_values))

        return {
            'best_config': best_config,
            'top_5': top_5,
            'total_analyzed': len(scenarios),
            'scorable': len(with_scores),
            'efficiency_range': efficiency_range,
            'power_range': power_range,
        }

    def compare_configurations(
        self, config_a: Dict, config_b: Dict
    ) -> Dict:
        """
        Compare two specific configurations head-to-head.
        
        Args:
            config_a: First scenario dict
            config_b: Second scenario dict
            
        Returns:
            Comparison dict with:
                - 'winner': 'config_a', 'config_b', or 'tie'
                - 'efficiency_diff': Absolute difference in efficiency scores
                - 'efficiency_pct_diff': Percentage difference
                - 'power_diff': Difference in mean_watts
                - 'justification': Human-readable explanation
        
        Example:
            >>> comparison = recommender.compare_configurations(scenario_a, scenario_b)
            >>> print(comparison['justification'])
        """
        score_a = self.scorer.compute_score(config_a)
        score_b = self.scorer.compute_score(config_b)

        if score_a is None or score_b is None:
            return {
                'winner': 'tie',
                'efficiency_diff': None,
                'efficiency_pct_diff': None,
                'power_diff': None,
                'justification': 'Cannot compare: missing efficiency data',
            }

        efficiency_diff = score_a - score_b

        # Avoid division by zero
        avg_score = (score_a + score_b) / 2
        efficiency_pct_diff = (efficiency_diff / avg_score) * 100 if avg_score > 0 else 0

        winner = 'config_a' if efficiency_diff > 0 else 'config_b' if efficiency_diff < 0 else 'tie'

        power_a = config_a.get('power', {}).get('mean_watts')
        power_b = config_b.get('power', {}).get('mean_watts')
        power_diff = None
        if power_a is not None and power_b is not None:
            power_diff = power_a - power_b

        # Generate justification
        if winner == 'tie':
            justification = f"Configurations are equally efficient (both {score_a:.4f} Mbps/W)"
        else:
            winner_name = config_a['name'] if winner == 'config_a' else config_b['name']
            winner_score = score_a if winner == 'config_a' else score_b
            loser_score = score_a if winner == 'config_b' else score_b
            justification = (
                f"{winner_name} is {abs(efficiency_pct_diff):.1f}% more efficient "
                f"({winner_score:.4f} Mbps/W vs {loser_score:.4f} Mbps/W)"
            )

        return {
            'winner': winner,
            'efficiency_diff': efficiency_diff,
            'efficiency_pct_diff': efficiency_pct_diff,
            'power_diff': power_diff,
            'justification': justification,
        }

    # ============================================================================
    # Placeholder for future CLI interface
    # ============================================================================

    def generate_cli_report(self, scenarios: List[Dict]) -> str:
        """
        PLACEHOLDER: Generate CLI-friendly recommendation report.
        
        Future implementation will provide:
        - Formatted ASCII table of top configurations
        - Energy savings estimates vs baseline
        - Hardware utilization summary
        - Actionable next steps
        
        Args:
            scenarios: List of scenario dicts
            
        Returns:
            Formatted report string (not implemented yet)
        """
        raise NotImplementedError(
            "CLI report generation will be implemented in future versions. "
            "For now, use get_recommendation_summary() and format manually."
        )
