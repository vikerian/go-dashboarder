package domain

import "fmt"

// Validate kontroluje, zda jsou IoT data kompletní před odesláním do DB.
func (i IoTData) Validate() error {
	if i.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}
	if i.Metric == "" {
		return fmt.Errorf("metric name is required")
	}
	return nil
}
