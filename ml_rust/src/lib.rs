//! FFmpeg ML Library - Rust implementation of ML models for power prediction
//!
//! This library provides high-performance ML models and cost calculations
//! for FFmpeg transcoding power optimization.

use serde::{Deserialize, Serialize};
use std::ffi::CStr;
use std::os::raw::c_char;

/// Linear regression model for power prediction
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LinearPredictor {
    intercept: f64,
    slope: f64,
    r2: f64,
}

impl LinearPredictor {
    /// Create new predictor
    pub fn new() -> Self {
        Self {
            intercept: 0.0,
            slope: 0.0,
            r2: 0.0,
        }
    }

    /// Train model using ordinary least squares
    pub fn fit(&mut self, x: &[f64], y: &[f64]) -> Result<(), String> {
        if x.len() != y.len() || x.is_empty() {
            return Err("Invalid input".to_string());
        }

        let n = x.len() as f64;
        let mean_x = x.iter().sum::<f64>() / n;
        let mean_y = y.iter().sum::<f64>() / n;

        let mut numerator = 0.0;
        let mut denominator = 0.0;
        for i in 0..x.len() {
            let dx = x[i] - mean_x;
            let dy = y[i] - mean_y;
            numerator += dx * dy;
            denominator += dx * dx;
        }

        self.slope = if denominator != 0.0 {
            numerator / denominator
        } else {
            0.0
        };
        self.intercept = mean_y - self.slope * mean_x;

        // Calculate R²
        let mut ss_tot = 0.0;
        let mut ss_res = 0.0;
        for i in 0..x.len() {
            let pred = self.intercept + self.slope * x[i];
            ss_tot += (y[i] - mean_y).powi(2);
            ss_res += (y[i] - pred).powi(2);
        }
        self.r2 = if ss_tot != 0.0 {
            1.0 - ss_res / ss_tot
        } else {
            0.0
        };

        Ok(())
    }

    /// Predict power for given streams
    pub fn predict(&self, x: f64) -> f64 {
        (self.intercept + self.slope * x).max(0.0)
    }

    /// Get R² score
    pub fn r2_score(&self) -> f64 {
        self.r2
    }
}

/// Cost model for transcoding
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CostModel {
    pub energy_cost_per_kwh: f64,
    pub cpu_cost_per_hour: f64,
}

impl CostModel {
    pub fn new(energy_cost_per_kwh: f64, cpu_cost_per_hour: f64) -> Self {
        Self {
            energy_cost_per_kwh,
            cpu_cost_per_hour,
        }
    }

    /// Compute energy cost from joules
    pub fn compute_energy_cost(&self, energy_joules: f64) -> f64 {
        let energy_kwh = energy_joules / 3_600_000.0;
        energy_kwh * self.energy_cost_per_kwh
    }

    /// Compute total cost
    pub fn compute_total_cost(&self, energy_joules: f64, duration_hours: f64) -> f64 {
        let energy_cost = self.compute_energy_cost(energy_joules);
        let compute_cost = duration_hours * self.cpu_cost_per_hour;
        energy_cost + compute_cost
    }
}

/// Regional pricing
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RegionalPricing {
    pub region: String,
    pub electricity_price: f64,
    pub carbon_intensity: f64,
}

impl RegionalPricing {
    pub fn new(region: &str) -> Self {
        let (price, intensity) = match region {
            "us-east-1" => (0.13, 0.45),
            "us-west-2" => (0.10, 0.30),
            "eu-west-1" => (0.20, 0.28),
            "eu-north-1" => (0.08, 0.12),
            _ => (0.12, 0.50),
        };

        Self {
            region: region.to_string(),
            electricity_price: price,
            carbon_intensity: intensity,
        }
    }

    pub fn compute_co2_emissions(&self, energy_kwh: f64) -> f64 {
        energy_kwh * self.carbon_intensity
    }
}

// ============================================================================
// C FFI for Go integration
// ============================================================================

#[repr(C)]
pub struct CLinearPredictor {
    inner: LinearPredictor,
}

#[no_mangle]
pub extern "C" fn linear_predictor_new() -> *mut CLinearPredictor {
    Box::into_raw(Box::new(CLinearPredictor {
        inner: LinearPredictor::new(),
    }))
}

#[no_mangle]
pub extern "C" fn linear_predictor_fit(
    ptr: *mut CLinearPredictor,
    x: *const f64,
    y: *const f64,
    n: usize,
) -> i32 {
    if ptr.is_null() || x.is_null() || y.is_null() {
        return -1;
    }

    unsafe {
        let predictor = &mut (*ptr).inner;
        let x_slice = std::slice::from_raw_parts(x, n);
        let y_slice = std::slice::from_raw_parts(y, n);

        match predictor.fit(x_slice, y_slice) {
            Ok(_) => 0,
            Err(_) => -1,
        }
    }
}

#[no_mangle]
pub extern "C" fn linear_predictor_predict(ptr: *const CLinearPredictor, x: f64) -> f64 {
    if ptr.is_null() {
        return 0.0;
    }
    unsafe { (*ptr).inner.predict(x) }
}

#[no_mangle]
pub extern "C" fn linear_predictor_r2(ptr: *const CLinearPredictor) -> f64 {
    if ptr.is_null() {
        return 0.0;
    }
    unsafe { (*ptr).inner.r2_score() }
}

#[no_mangle]
pub extern "C" fn linear_predictor_free(ptr: *mut CLinearPredictor) {
    if !ptr.is_null() {
        unsafe {
            drop(Box::from_raw(ptr));
        }
    }
}

#[repr(C)]
pub struct CCostModel {
    inner: CostModel,
}

#[no_mangle]
pub extern "C" fn cost_model_new(energy_cost_per_kwh: f64, cpu_cost_per_hour: f64) -> *mut CCostModel {
    Box::into_raw(Box::new(CCostModel {
        inner: CostModel::new(energy_cost_per_kwh, cpu_cost_per_hour),
    }))
}

#[no_mangle]
pub extern "C" fn cost_model_compute_total_cost(
    ptr: *const CCostModel,
    energy_joules: f64,
    duration_hours: f64,
) -> f64 {
    if ptr.is_null() {
        return 0.0;
    }
    unsafe { (*ptr).inner.compute_total_cost(energy_joules, duration_hours) }
}

#[no_mangle]
pub extern "C" fn cost_model_free(ptr: *mut CCostModel) {
    if !ptr.is_null() {
        unsafe {
            drop(Box::from_raw(ptr));
        }
    }
}

#[repr(C)]
pub struct CRegionalPricing {
    inner: RegionalPricing,
}

#[no_mangle]
pub extern "C" fn regional_pricing_new(region: *const c_char) -> *mut CRegionalPricing {
    let region_str = if region.is_null() {
        "default"
    } else {
        unsafe {
            CStr::from_ptr(region).to_str().unwrap_or("default")
        }
    };

    Box::into_raw(Box::new(CRegionalPricing {
        inner: RegionalPricing::new(region_str),
    }))
}

#[no_mangle]
pub extern "C" fn regional_pricing_get_electricity_price(ptr: *const CRegionalPricing) -> f64 {
    if ptr.is_null() {
        return 0.0;
    }
    unsafe { (*ptr).inner.electricity_price }
}

#[no_mangle]
pub extern "C" fn regional_pricing_compute_co2(ptr: *const CRegionalPricing, energy_kwh: f64) -> f64 {
    if ptr.is_null() {
        return 0.0;
    }
    unsafe { (*ptr).inner.compute_co2_emissions(energy_kwh) }
}

#[no_mangle]
pub extern "C" fn regional_pricing_free(ptr: *mut CRegionalPricing) {
    if !ptr.is_null() {
        unsafe {
            drop(Box::from_raw(ptr));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_linear_predictor() {
        let mut predictor = LinearPredictor::new();
        let x = vec![1.0, 2.0, 3.0, 4.0, 5.0];
        let y = vec![50.0, 75.0, 100.0, 125.0, 150.0];

        predictor.fit(&x, &y).unwrap();
        
        assert!(predictor.r2_score() > 0.99);
        assert!((predictor.predict(6.0) - 175.0).abs() < 1.0);
    }

    #[test]
    fn test_cost_model() {
        let model = CostModel::new(0.12, 0.50);
        let cost = model.compute_total_cost(3_600_000.0, 1.0);
        assert!((cost - 0.62).abs() < 0.01);
    }

    #[test]
    fn test_regional_pricing() {
        let pricing = RegionalPricing::new("us-east-1");
        assert_eq!(pricing.electricity_price, 0.13);
        
        let co2 = pricing.compute_co2_emissions(10.0);
        assert!((co2 - 4.5).abs() < 0.01);
    }
}

