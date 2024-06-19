package parser

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	evictionapi "k8s.io/kubernetes/pkg/kubelet/eviction/api"
	"strconv"
	"strings"
)

// ParseThresholdStatement parses a threshold statement and returns a threshold,
// or nil if the threshold should be ignored.
func ParseThresholdStatement(signal evictionapi.Signal, val string) (*evictionapi.Threshold, error) {
	operator := evictionapi.OpForSignal[signal]
	if strings.HasSuffix(val, "%") {
		// ignore 0% and 100%
		if val == "0%" || val == "100%" {
			return nil, nil
		}
		percentage, err := parsePercentage(val)
		if err != nil {
			return nil, err
		}
		if percentage < 0 {
			return nil, fmt.Errorf("eviction percentage threshold %v must be >= 0%%: %s", signal, val)
		}
		// percentage is a float and should not be greater than 1 (100%)
		if percentage > 1 {
			return nil, fmt.Errorf("eviction percentage threshold %v must be <= 100%%: %s", signal, val)
		}
		return &evictionapi.Threshold{
			Signal:   signal,
			Operator: operator,
			Value: evictionapi.ThresholdValue{
				Percentage: percentage,
			},
		}, nil
	}
	quantity, err := resource.ParseQuantity(val)
	if err != nil {
		return nil, err
	}
	if quantity.Sign() < 0 || quantity.IsZero() {
		return nil, fmt.Errorf("eviction threshold %v must be positive: %s", signal, &quantity)
	}
	return &evictionapi.Threshold{
		Signal:   signal,
		Operator: operator,
		Value: evictionapi.ThresholdValue{
			Quantity: &quantity,
		},
	}, nil
}

// parsePercentage parses a string representing a percentage value
func parsePercentage(input string) (float32, error) {
	value, err := strconv.ParseFloat(strings.TrimRight(input, "%"), 32)
	if err != nil {
		return 0, err
	}
	return float32(value) / 100, nil
}
