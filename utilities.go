package main

// convertCSQToDBm converts CSQ value to dBm
func convertCSQToDBm(csq int) float64 {
	if csq == 0 {
		return -113.0
	} else if csq == 1 {
		return -111.0
	} else if csq >= 2 && csq <= 30 {
		return -109.0 + 2.0*(float64(csq)-2.0)
	} else if csq == 31 {
		return -51.0
	}
	return -113.0
}

// convertLTEValues converts LTE parameter values to actual measurements
func convertLTEValues(rsrp, sinr, rsrq int) (float64, float64, float64) {
	// RSRP conversion (typically -140dBm to -44dBm)
	rsrp_dBm := -140.0 + float64(rsrp)

	// SINR conversion (typically -20dB to 50dB)
	sinr_dB := -20.0 + 0.5*float64(sinr)

	// RSRQ conversion (typically -20dB to -3dB)
	rsrq_dB := -20.0 + 0.5*float64(rsrq)

	return rsrp_dBm, sinr_dB, rsrq_dB
}

// analyzeSignalQuality determines overall signal quality
func (sa *SignalAnalyzer) analyzeSignalQuality(status *ModemStatus) string {
	score := 0

	// RSRP scoring
	if status.RSRP_dBm >= -85 {
		score += 3
	} else if status.RSRP_dBm >= -95 {
		score += 2
	} else if status.RSRP_dBm >= -105 {
		score += 1
	}

	// SINR scoring
	if status.SINR_dB >= 20 {
		score += 3
	} else if status.SINR_dB >= 13 {
		score += 2
	} else if status.SINR_dB >= 0 {
		score += 1
	}

	// RSRQ scoring
	if status.RSRQ_dB >= -7 {
		score += 2
	} else if status.RSRQ_dB >= -10 {
		score += 1
	}

	switch {
	case score >= 7:
		return "Excellent"
	case score >= 5:
		return "Good"
	case score >= 3:
		return "Fair"
	default:
		return "Poor"
	}
}

// analyzeHealthStatus determines modem health status
func (sa *SignalAnalyzer) analyzeHealthStatus(status *ModemStatus) string {
	if status.RSRP_dBm < -120 || status.SINR_dB < -5 {
		return "Critical"
	} else if status.RSRP_dBm < -110 || status.SINR_dB < 0 {
		return "Degraded"
	} else if status.RSRP_dBm < -100 || status.SINR_dB < 10 {
		return "Stable"
	}
	return "Optimal"
}
