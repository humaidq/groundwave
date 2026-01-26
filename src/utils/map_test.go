// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"os"
	"testing"
)

func TestCreateGridMap(t *testing.T) {
	config := MapConfig{
		Width:      400,
		Height:     300,
		Zoom:       3,
		OutputPath: "test_map.png",
	}

	myGrid := "FN31pr"
	theirGrid := "DM79hx"

	err := CreateGridMap(myGrid, theirGrid, config)
	if err != nil {
		t.Fatalf("CreateGridMap failed: %v", err)
	}

	if _, err := os.Stat(config.OutputPath); os.IsNotExist(err) {
		t.Fatalf("Output file %s was not created", config.OutputPath)
	}

	_ = os.Remove(config.OutputPath)
}

func TestCreateGridMapWithDistance(t *testing.T) {
	config := MapConfig{
		Width:      400,
		Height:     300,
		Zoom:       3,
		OutputPath: "test_map_distance.png",
	}

	myGrid := "FN31pr"
	theirGrid := "DM79hx"

	distance, err := CreateGridMapWithDistance(myGrid, theirGrid, config)
	if err != nil {
		t.Fatalf("CreateGridMapWithDistance failed: %v", err)
	}

	if distance <= 0 {
		t.Fatalf("Expected positive distance, got %f", distance)
	}

	t.Logf("Distance between %s and %s: %.2f km", myGrid, theirGrid, distance)

	if _, err := os.Stat(config.OutputPath); os.IsNotExist(err) {
		t.Fatalf("Output file %s was not created", config.OutputPath)
	}

	_ = os.Remove(config.OutputPath)
}
