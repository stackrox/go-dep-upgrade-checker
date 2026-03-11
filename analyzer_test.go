package main

import (
	"testing"
)

func TestExtractComponent(t *testing.T) {
	repoRoot := "/home/user/stackrox"

	tests := []struct {
		filePath string
		expected string
	}{
		{
			filePath: "/home/user/stackrox/central/deployment/manager.go",
			expected: "central",
		},
		{
			filePath: "/home/user/stackrox/sensor/kubernetes/client.go",
			expected: "sensor",
		},
		{
			filePath: "/home/user/stackrox/roxctl/main.go",
			expected: "roxctl",
		},
		{
			filePath: "/home/user/stackrox/pkg/utils/helpers.go",
			expected: "pkg",
		},
		{
			filePath: "/home/user/stackrox/scanner/scanner.go",
			expected: "scanner",
		},
		{
			filePath: "/home/user/stackrox/operator/apis/v1alpha1/types.go",
			expected: "operator",
		},
		{
			filePath: "/home/user/stackrox/migrator/migration.go",
			expected: "migrator",
		},
		{
			filePath: "/home/user/stackrox/ui/src/app.tsx",
			expected: "ui",
		},
		{
			filePath: "/home/user/stackrox/generated/proto/api.pb.go",
			expected: "generated",
		},
		{
			filePath: "/home/user/stackrox/tools/scripts/helper.sh",
			expected: "tools",
		},
		{
			filePath: "/home/user/stackrox/qa-tests-backend/tests/test.go",
			expected: "qa-tests-backend",
		},
		{
			filePath: "/home/user/stackrox/unknown/file.go",
			expected: "other",
		},
		{
			filePath: "/home/user/stackrox/docs/readme.md",
			expected: "other",
		},
	}

	for _, tt := range tests {
		result := extractComponent(tt.filePath, repoRoot)
		if result != tt.expected {
			t.Errorf("extractComponent(%q, %q) = %q, want %q", tt.filePath, repoRoot, result, tt.expected)
		}
	}
}

func TestIsCritical(t *testing.T) {
	tests := []struct {
		components []string
		expected   bool
	}{
		{
			components: []string{"central"},
			expected:   true,
		},
		{
			components: []string{"sensor"},
			expected:   true,
		},
		{
			components: []string{"central", "sensor"},
			expected:   true,
		},
		{
			components: []string{"roxctl", "pkg"},
			expected:   false,
		},
		{
			components: []string{"scanner"},
			expected:   false,
		},
		{
			components: []string{},
			expected:   false,
		},
	}

	for _, tt := range tests {
		ia := &ImpactAnalysis{Components: tt.components}
		result := ia.IsCritical()
		if result != tt.expected {
			t.Errorf("IsCritical(%v) = %v, want %v", tt.components, result, tt.expected)
		}
	}
}
