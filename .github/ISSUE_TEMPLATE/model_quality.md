---
name: Model Quality Issue
about: Report issues with power prediction model accuracy or performance
title: "[MODEL] "
labels: model, quality
assignees: ''

---

## Model Issue Type
- [ ] Poor prediction accuracy
- [ ] Model training failure
- [ ] Cross-validation issues
- [ ] Feature engineering suggestion
- [ ] Other: ___________

## Dataset Information
- Number of scenarios: ___
- Stream count range: ___ to ___
- Power range: ___ to ___ W
- Model type used: Linear / Polynomial

## Model Metrics
If applicable, provide the model quality metrics:
- R² score: ___
- RMSE: ___ W
- MAE: ___ W
- CV RMSE: ___ ± ___ W

## Issue Description
Describe the model quality issue in detail.

## Test Results File
If possible, attach or link to the test_results_*.json file.

## Hardware Configuration
- CPU: [e.g., Intel i7-12700K]
- Number of cores: ___
- Base/Max frequency: ___
- Power limits: ___

## Expected vs Actual
What predictions did you expect vs what the model produced?

## Suggestions
Do you have ideas on how to improve the model for this scenario?

## Additional Context
Any other relevant information about the workload or environment.
