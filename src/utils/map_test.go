// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sm "github.com/flopp/go-staticmaps"
	"github.com/golang/geo/s2"
)

type stubMapContext struct {
	width       int
	height      int
	zoom        int
	center      s2.LatLng
	objects     []sm.MapObject
	baseAttrib  string
	overrideAtt string
	renderImage image.Image
	renderErr   error
}

func (s *stubMapContext) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *stubMapContext) SetZoom(zoom int) {
	s.zoom = zoom
}

func (s *stubMapContext) SetCenter(center s2.LatLng) {
	s.center = center
}

func (s *stubMapContext) AddObject(object sm.MapObject) {
	s.objects = append(s.objects, object)
}

func (s *stubMapContext) Attribution() string {
	if s.overrideAtt != "" {
		return s.overrideAtt
	}
	return s.baseAttrib
}

func (s *stubMapContext) OverrideAttribution(attribution string) {
	s.overrideAtt = attribution
}

func (s *stubMapContext) Render() (image.Image, error) {
	if s.renderErr != nil {
		return nil, s.renderErr
	}
	if s.renderImage != nil {
		return s.renderImage, nil
	}
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
}

type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (nopWriteCloser) Close() error {
	return nil
}

type closeErrorWriter struct{}

func (closeErrorWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (closeErrorWriter) Close() error {
	return errors.New("close failed")
}

func TestDefaultMapConfig(t *testing.T) {
	config := DefaultMapConfig()
	if config.Width != 800 || config.Height != 600 || config.Zoom != 4 || config.OutputPath != "grid_map.png" {
		t.Fatalf("unexpected default config: %+v", config)
	}
}

func TestNewMapContextDefault(t *testing.T) {
	ctx := newMapContext()
	if ctx == nil {
		t.Fatalf("expected default map context")
	}
}

func TestCalculateZoomLevel(t *testing.T) {
	if got := calculateZoomLevel(-80, 80, -170, 170, 256, 256); got != 1 {
		t.Fatalf("expected low clamp 1, got %d", got)
	}
	if got := calculateZoomLevel(0, 0.0001, 0, 0.0001, 256, 256); got != 18 {
		t.Fatalf("expected high clamp 18, got %d", got)
	}
	if got := calculateZoomLevel(0, 10, 0, 10, 800, 600); got != 5 {
		t.Fatalf("expected mid zoom 5, got %d", got)
	}
}

func TestCreateGridMapUsesStubContext(t *testing.T) {
	origNewMapContext := newMapContext
	stub := &stubMapContext{baseAttrib: "Base Attribution"}
	newMapContext = func() mapContext {
		return stub
	}
	defer func() {
		newMapContext = origNewMapContext
	}()

	outputPath := filepath.Join(t.TempDir(), "test_map.png")
	config := MapConfig{
		Width:      400,
		Height:     300,
		Zoom:       7,
		OutputPath: outputPath,
	}

	myGrid := "FN31pr"
	theirGrid := "FN31pr"

	if err := CreateGridMap(myGrid, theirGrid, config); err != nil {
		t.Fatalf("CreateGridMap failed: %v", err)
	}

	if stub.width != config.Width || stub.height != config.Height {
		t.Fatalf("expected size %dx%d, got %dx%d", config.Width, config.Height, stub.width, stub.height)
	}
	if stub.zoom != config.Zoom {
		t.Fatalf("expected zoom %d, got %d", config.Zoom, stub.zoom)
	}
	if len(stub.objects) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(stub.objects))
	}
	expectedAttr := fmt.Sprintf("QSL Map: %s <-> %s\n%s", myGrid, theirGrid, stub.baseAttrib)
	if stub.overrideAtt != expectedAttr {
		t.Fatalf("unexpected attribution override: %q", stub.overrideAtt)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
}

func TestCreateGridMapInvalidMyGrid(t *testing.T) {
	origNewMapContext := newMapContext
	newMapContext = func() mapContext {
		return &stubMapContext{}
	}
	defer func() {
		newMapContext = origNewMapContext
	}()

	outputPath := filepath.Join(t.TempDir(), "map.png")
	config := MapConfig{Width: 1, Height: 1, OutputPath: outputPath}

	err := CreateGridMap("invalid", "FN31pr", config)
	if err == nil || !strings.Contains(err.Error(), "failed to parse my grid locator") {
		t.Fatalf("expected my grid parse error, got %v", err)
	}
}

func TestCreateGridMapInvalidTheirGrid(t *testing.T) {
	origNewMapContext := newMapContext
	newMapContext = func() mapContext {
		return &stubMapContext{}
	}
	defer func() {
		newMapContext = origNewMapContext
	}()

	outputPath := filepath.Join(t.TempDir(), "map.png")
	config := MapConfig{Width: 1, Height: 1, OutputPath: outputPath}

	err := CreateGridMap("FN31pr", "invalid", config)
	if err == nil || !strings.Contains(err.Error(), "failed to parse their grid locator") {
		t.Fatalf("expected their grid parse error, got %v", err)
	}
}

func TestCreateGridMapRenderError(t *testing.T) {
	origNewMapContext := newMapContext
	newMapContext = func() mapContext {
		return &stubMapContext{renderErr: errors.New("render failed")}
	}
	defer func() {
		newMapContext = origNewMapContext
	}()

	outputPath := filepath.Join(t.TempDir(), "map.png")
	config := MapConfig{Width: 100, Height: 100, OutputPath: outputPath}

	err := CreateGridMap("FN31pr", "DM79hx", config)
	if err == nil || !strings.Contains(err.Error(), "failed to render map") {
		t.Fatalf("expected render error, got %v", err)
	}
}

func TestCreateGridMapSaveError(t *testing.T) {
	origNewMapContext := newMapContext
	origCreateFile := createFile
	newMapContext = func() mapContext {
		return &stubMapContext{renderImage: image.NewRGBA(image.Rect(0, 0, 1, 1))}
	}
	createFile = func(_ string) (io.WriteCloser, error) {
		return nil, errors.New("create failed")
	}
	defer func() {
		newMapContext = origNewMapContext
		createFile = origCreateFile
	}()

	outputPath := filepath.Join(t.TempDir(), "map.png")
	config := MapConfig{Width: 100, Height: 100, OutputPath: outputPath}

	err := CreateGridMap("FN31pr", "DM79hx", config)
	if err == nil || !strings.Contains(err.Error(), "failed to create file") {
		t.Fatalf("expected save error, got %v", err)
	}
}

func TestCreateGridMapWithDistanceSuccess(t *testing.T) {
	origNewMapContext := newMapContext
	stub := &stubMapContext{renderImage: image.NewRGBA(image.Rect(0, 0, 1, 1))}
	newMapContext = func() mapContext {
		return stub
	}
	defer func() {
		newMapContext = origNewMapContext
	}()

	outputPath := filepath.Join(t.TempDir(), "test_map_distance.png")
	config := MapConfig{
		Width:      400,
		Height:     300,
		Zoom:       3,
		OutputPath: outputPath,
	}

	distance, err := CreateGridMapWithDistance("FN31pr", "DM79hx", config)
	if err != nil {
		t.Fatalf("CreateGridMapWithDistance failed: %v", err)
	}
	if distance <= 0 {
		t.Fatalf("expected positive distance, got %f", distance)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
}

func TestCreateGridMapWithDistanceRenderError(t *testing.T) {
	origNewMapContext := newMapContext
	newMapContext = func() mapContext {
		return &stubMapContext{renderErr: errors.New("render failed")}
	}
	defer func() {
		newMapContext = origNewMapContext
	}()

	outputPath := filepath.Join(t.TempDir(), "map.png")
	config := MapConfig{Width: 100, Height: 100, OutputPath: outputPath}

	distance, err := CreateGridMapWithDistance("FN31pr", "DM79hx", config)
	if err == nil {
		t.Fatalf("expected render error")
	}
	if distance <= 0 {
		t.Fatalf("expected distance to be computed even on error")
	}
}

func TestCreateGridMapWithDistanceInvalidMyGrid(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "map.png")
	config := MapConfig{Width: 100, Height: 100, OutputPath: outputPath}

	if _, err := CreateGridMapWithDistance("invalid", "DM79hx", config); err == nil {
		t.Fatalf("expected error for invalid my grid")
	}
}

func TestCreateGridMapWithDistanceInvalidTheirGrid(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "map.png")
	config := MapConfig{Width: 100, Height: 100, OutputPath: outputPath}

	if _, err := CreateGridMapWithDistance("FN31pr", "invalid", config); err == nil {
		t.Fatalf("expected error for invalid their grid")
	}
}

func TestSaveImageCreateError(t *testing.T) {
	origCreateFile := createFile
	createFile = func(_ string) (io.WriteCloser, error) {
		return nil, errors.New("create failed")
	}
	defer func() {
		createFile = origCreateFile
	}()

	if err := saveImage(image.NewRGBA(image.Rect(0, 0, 1, 1)), "ignored"); err == nil {
		t.Fatalf("expected create file error")
	}
}

func TestSaveImageEncodeError(t *testing.T) {
	origCreateFile := createFile
	origEncodePNG := encodePNG
	createFile = func(_ string) (io.WriteCloser, error) {
		return nopWriteCloser{}, nil
	}
	encodePNG = func(_ io.Writer, _ image.Image) error {
		return errors.New("encode failed")
	}
	defer func() {
		createFile = origCreateFile
		encodePNG = origEncodePNG
	}()

	if err := saveImage(image.NewRGBA(image.Rect(0, 0, 1, 1)), "ignored"); err == nil {
		t.Fatalf("expected encode error")
	}
}

func TestSaveImageCloseError(t *testing.T) {
	origCreateFile := createFile
	createFile = func(_ string) (io.WriteCloser, error) {
		return closeErrorWriter{}, nil
	}
	defer func() {
		createFile = origCreateFile
	}()

	if err := saveImage(image.NewRGBA(image.Rect(0, 0, 1, 1)), "ignored"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
