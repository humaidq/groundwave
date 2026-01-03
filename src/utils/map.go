package utils

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	sm "github.com/flopp/go-staticmaps"
	"github.com/golang/geo/s2"
	"github.com/pd0mz/go-maidenhead"
)

type MapConfig struct {
	Width      int
	Height     int
	Zoom       int
	OutputPath string
}

func DefaultMapConfig() MapConfig {
	return MapConfig{
		Width:      800,
		Height:     600,
		Zoom:       4,
		OutputPath: "grid_map.png",
	}
}

func CreateGridMap(myGrid, theirGrid string, config MapConfig) error {
	ctx := sm.NewContext()
	ctx.SetSize(config.Width, config.Height)

	myPoint, err := maidenhead.ParseLocator(myGrid)
	if err != nil {
		return fmt.Errorf("failed to parse my grid locator %s: %w", myGrid, err)
	}

	theirPoint, err := maidenhead.ParseLocator(theirGrid)
	if err != nil {
		return fmt.Errorf("failed to parse their grid locator %s: %w", theirGrid, err)
	}

	myPos := s2.LatLngFromDegrees(myPoint.Latitude, myPoint.Longitude)
	theirPos := s2.LatLngFromDegrees(theirPoint.Latitude, theirPoint.Longitude)

	// Calculate bounding box and appropriate zoom level
	minLat := math.Min(myPoint.Latitude, theirPoint.Latitude)
	maxLat := math.Max(myPoint.Latitude, theirPoint.Latitude)
	minLon := math.Min(myPoint.Longitude, theirPoint.Longitude)
	maxLon := math.Max(myPoint.Longitude, theirPoint.Longitude)

	// Add padding (10% of the range)
	latRange := maxLat - minLat
	lonRange := maxLon - minLon
	
	// Ensure minimum range to avoid extreme zoom for very close locations
	if latRange < 1.0 {
		latRange = 1.0
	}
	if lonRange < 1.0 {
		lonRange = 1.0
	}
	
	padding := 0.1
	paddedMinLat := minLat - (latRange * padding)
	paddedMaxLat := maxLat + (latRange * padding)
	paddedMinLon := minLon - (lonRange * padding)
	paddedMaxLon := maxLon + (lonRange * padding)

	// Calculate zoom level based on the bounding box
	zoom := calculateZoomLevel(paddedMinLat, paddedMaxLat, paddedMinLon, paddedMaxLon, config.Width, config.Height)
	
	// Override zoom if specified in config (for manual control)
	if config.Zoom > 0 {
		zoom = config.Zoom
	}
	
	ctx.SetZoom(zoom)

	// Set center point
	centerLat := (myPoint.Latitude + theirPoint.Latitude) / 2
	centerLon := (myPoint.Longitude + theirPoint.Longitude) / 2
	ctx.SetCenter(s2.LatLngFromDegrees(centerLat, centerLon))

	// Add markers and path
	ctx.AddObject(sm.NewMarker(myPos, color.RGBA{255, 0, 0, 255}, 16.0))
	ctx.AddObject(sm.NewMarker(theirPos, color.RGBA{0, 0, 255, 255}, 16.0))

	path := sm.NewPath([]s2.LatLng{myPos, theirPos}, color.RGBA{0, 255, 0, 255}, 2)
	ctx.AddObject(path)

	// Get original attribution and create custom attribution
	originalAttribution := ctx.Attribution()
	customAttribution := fmt.Sprintf("QSL Map: %s <-> %s\n%s", myGrid, theirGrid, originalAttribution)
	ctx.OverrideAttribution(customAttribution)

	img, err := ctx.Render()
	if err != nil {
		return fmt.Errorf("failed to render map: %w", err)
	}

	return saveImage(img, config.OutputPath)
}

// calculateZoomLevel calculates appropriate zoom level to fit bounding box
func calculateZoomLevel(minLat, maxLat, minLon, maxLon float64, width, height int) int {
	// Web Mercator projection bounds
	latRange := maxLat - minLat
	lonRange := maxLon - minLon
	
	// Calculate zoom level needed for latitude
	latZoom := math.Log2(180.0 / latRange)
	
	// Calculate zoom level needed for longitude  
	lonZoom := math.Log2(360.0 / lonRange)
	
	// Use the more restrictive (lower) zoom level
	zoom := math.Min(latZoom, lonZoom)
	
	// Account for map dimensions (approximate adjustment)
	zoom = zoom + math.Log2(math.Min(float64(width)/256.0, float64(height)/256.0))
	
	// Clamp between 1 and 18
	if zoom < 1 {
		zoom = 1
	}
	if zoom > 18 {
		zoom = 18
	}
	
	return int(math.Floor(zoom))
}

func CreateGridMapWithDistance(myGrid, theirGrid string, config MapConfig) (float64, error) {
	myPoint, err := maidenhead.ParseLocator(myGrid)
	if err != nil {
		return 0, fmt.Errorf("failed to parse my grid locator %s: %w", myGrid, err)
	}

	theirPoint, err := maidenhead.ParseLocator(theirGrid)
	if err != nil {
		return 0, fmt.Errorf("failed to parse their grid locator %s: %w", theirGrid, err)
	}

	myPos := s2.LatLngFromDegrees(myPoint.Latitude, myPoint.Longitude)
	theirPos := s2.LatLngFromDegrees(theirPoint.Latitude, theirPoint.Longitude)

	distance := myPos.Distance(theirPos).Degrees() * 111.32

	// Use CreateGridMap which now includes custom attribution
	err = CreateGridMap(myGrid, theirGrid, config)
	if err != nil {
		return distance, err
	}

	return distance, nil
}

func saveImage(img image.Image, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}()

	err = png.Encode(file, img)
	if err != nil {
		return fmt.Errorf("failed to encode PNG: %w", err)
	}

	return nil
}
