# Trained ML Models

This directory contains trained machine learning models for power prediction.

Models are organized by hardware ID:
- `models/<hardware_id>/power_predictor_latest.pkl` - Simple power predictor
- `models/<hardware_id>/multivariate_predictor_latest.pkl` - Advanced multivariate predictor
- `models/<hardware_id>/metadata.json` - Model metadata

## Retraining Models

To retrain models from test results:

```bash
make retrain-models
```

Or manually:

```bash
python3 scripts/retrain_models.py --results-dir ./test_results --models-dir ./models
```

## Model Versioning

Models are timestamped and versioned. The `_latest` symlinks always point to the most recent models.

