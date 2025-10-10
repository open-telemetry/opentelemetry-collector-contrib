package newrelicsqlserverreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGranularFailoverClusterToggles tests the granular failover cluster metric toggles
func TestGranularFailoverClusterToggles(t *testing.T) {
	tests := []struct {
		name            string
		mainToggle      bool
		granularToggles map[string]bool
		expectedResults map[string]bool
		description     string
	}{
		{
			name:       "main_toggle_enabled_overrides_granular",
			mainToggle: true,
			granularToggles: map[string]bool{
				"replica":             false,
				"replica_state":       false,
				"node":                false,
				"ag_health":           false,
				"ag_config":           false,
				"performance_counter": false,
				"cluster_properties":  false,
			},
			expectedResults: map[string]bool{
				"replica":             true, // main toggle overrides
				"replica_state":       true, // main toggle overrides
				"node":                true, // main toggle overrides
				"ag_health":           true, // main toggle overrides
				"ag_config":           true, // main toggle overrides
				"performance_counter": true, // main toggle overrides
				"cluster_properties":  true, // main toggle overrides
			},
			description: "When main toggle is enabled, all granular metrics are enabled regardless of individual settings",
		},
		{
			name:       "main_toggle_disabled_uses_granular",
			mainToggle: false,
			granularToggles: map[string]bool{
				"replica":             true,
				"replica_state":       true,
				"node":                false,
				"ag_health":           true,
				"ag_config":           false,
				"performance_counter": false,
				"cluster_properties":  true,
			},
			expectedResults: map[string]bool{
				"replica":             true,
				"replica_state":       true,
				"node":                false,
				"ag_health":           true,
				"ag_config":           false,
				"performance_counter": false,
				"cluster_properties":  true,
			},
			description: "When main toggle is disabled, individual toggles control their respective metrics",
		},
		{
			name:       "all_disabled",
			mainToggle: false,
			granularToggles: map[string]bool{
				"replica":             false,
				"replica_state":       false,
				"node":                false,
				"ag_health":           false,
				"ag_config":           false,
				"performance_counter": false,
				"cluster_properties":  false,
			},
			expectedResults: map[string]bool{
				"replica":             false,
				"replica_state":       false,
				"node":                false,
				"ag_health":           false,
				"ag_config":           false,
				"performance_counter": false,
				"cluster_properties":  false,
			},
			description: "When all toggles are disabled, no metrics are enabled",
		},
		{
			name:       "selective_enabling",
			mainToggle: false,
			granularToggles: map[string]bool{
				"replica":             true,  // Critical for production
				"replica_state":       true,  // Critical for production
				"node":                true,  // Important for production
				"ag_health":           true,  // Critical for production
				"ag_config":           true,  // Important for production
				"performance_counter": false, // Optional for production
				"cluster_properties":  false, // Optional for production
			},
			expectedResults: map[string]bool{
				"replica":             true,
				"replica_state":       true,
				"node":                true,
				"ag_health":           true,
				"ag_config":           true,
				"performance_counter": false,
				"cluster_properties":  false,
			},
			description: "Production monitoring scenario: enable critical metrics, disable optional ones",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				EnableFailoverClusterMetrics:                        tt.mainToggle,
				EnableFailoverClusterReplicaMetrics:                 tt.granularToggles["replica"],
				EnableFailoverClusterReplicaStateMetrics:            tt.granularToggles["replica_state"],
				EnableFailoverClusterNodeMetrics:                    tt.granularToggles["node"],
				EnableFailoverClusterAvailabilityGroupHealthMetrics: tt.granularToggles["ag_health"],
				EnableFailoverClusterAvailabilityGroupMetrics:       tt.granularToggles["ag_config"],
				EnableFailoverClusterPerformanceCounterMetrics:      tt.granularToggles["performance_counter"],
				EnableFailoverClusterClusterPropertiesMetrics:       tt.granularToggles["cluster_properties"],
			}

			// Test all granular helper methods
			assert.Equal(t, tt.expectedResults["replica"], config.IsFailoverClusterReplicaMetricsEnabled(),
				"Replica metrics enabled check failed for test: %s", tt.description)

			assert.Equal(t, tt.expectedResults["replica_state"], config.IsFailoverClusterReplicaStateMetricsEnabled(),
				"Replica state metrics enabled check failed for test: %s", tt.description)

			assert.Equal(t, tt.expectedResults["node"], config.IsFailoverClusterNodeMetricsEnabled(),
				"Node metrics enabled check failed for test: %s", tt.description)

			assert.Equal(t, tt.expectedResults["ag_health"], config.IsFailoverClusterAvailabilityGroupHealthMetricsEnabled(),
				"AG health metrics enabled check failed for test: %s", tt.description)

			assert.Equal(t, tt.expectedResults["ag_config"], config.IsFailoverClusterAvailabilityGroupMetricsEnabled(),
				"AG config metrics enabled check failed for test: %s", tt.description)

			assert.Equal(t, tt.expectedResults["performance_counter"], config.IsFailoverClusterPerformanceCounterMetricsEnabled(),
				"Performance counter metrics enabled check failed for test: %s", tt.description)

			assert.Equal(t, tt.expectedResults["cluster_properties"], config.IsFailoverClusterClusterPropertiesMetricsEnabled(),
				"Cluster properties metrics enabled check failed for test: %s", tt.description)
		})
	}
}

// TestGranularToggleDefaults verifies that all granular toggles default to false
func TestGranularToggleDefaults(t *testing.T) {
	config := DefaultConfig().(*Config)

	// Main toggle should default to false
	assert.False(t, config.EnableFailoverClusterMetrics, "Main failover cluster toggle should default to false")

	// All granular toggles should default to false
	assert.False(t, config.EnableFailoverClusterReplicaMetrics, "Replica metrics toggle should default to false")
	assert.False(t, config.EnableFailoverClusterReplicaStateMetrics, "Replica state metrics toggle should default to false")
	assert.False(t, config.EnableFailoverClusterNodeMetrics, "Node metrics toggle should default to false")
	assert.False(t, config.EnableFailoverClusterAvailabilityGroupHealthMetrics, "AG health metrics toggle should default to false")
	assert.False(t, config.EnableFailoverClusterAvailabilityGroupMetrics, "AG config metrics toggle should default to false")
	assert.False(t, config.EnableFailoverClusterPerformanceCounterMetrics, "Performance counter metrics toggle should default to false")
	assert.False(t, config.EnableFailoverClusterClusterPropertiesMetrics, "Cluster properties metrics toggle should default to false")

	// With all defaults (false), helper methods should return false
	assert.False(t, config.IsFailoverClusterReplicaMetricsEnabled(), "Replica metrics should be disabled by default")
	assert.False(t, config.IsFailoverClusterReplicaStateMetricsEnabled(), "Replica state metrics should be disabled by default")
	assert.False(t, config.IsFailoverClusterNodeMetricsEnabled(), "Node metrics should be disabled by default")
	assert.False(t, config.IsFailoverClusterAvailabilityGroupHealthMetricsEnabled(), "AG health metrics should be disabled by default")
	assert.False(t, config.IsFailoverClusterAvailabilityGroupMetricsEnabled(), "AG config metrics should be disabled by default")
	assert.False(t, config.IsFailoverClusterPerformanceCounterMetricsEnabled(), "Performance counter metrics should be disabled by default")
	assert.False(t, config.IsFailoverClusterClusterPropertiesMetricsEnabled(), "Cluster properties metrics should be disabled by default")
}

// TestLogicalORBehavior specifically tests the logical OR behavior of toggles
func TestGranularDatabasePrincipalsToggles(t *testing.T) {
	tests := []struct {
		name                                     string
		enableDatabasePrincipalsMetrics          bool
		enableDatabasePrincipalsDetailsMetrics   bool
		enableDatabasePrincipalsSummaryMetrics   bool
		enableDatabasePrincipalsActivityMetrics  bool
		expectedDatabasePrincipalsDetailsResult  bool
		expectedDatabasePrincipalsSummaryResult  bool
		expectedDatabasePrincipalsActivityResult bool
	}{
		{
			name:                                     "All database principals metrics disabled",
			enableDatabasePrincipalsMetrics:          false,
			enableDatabasePrincipalsDetailsMetrics:   false,
			enableDatabasePrincipalsSummaryMetrics:   false,
			enableDatabasePrincipalsActivityMetrics:  false,
			expectedDatabasePrincipalsDetailsResult:  false,
			expectedDatabasePrincipalsSummaryResult:  false,
			expectedDatabasePrincipalsActivityResult: false,
		},
		{
			name:                                     "Main database principals toggle enabled only",
			enableDatabasePrincipalsMetrics:          true,
			enableDatabasePrincipalsDetailsMetrics:   false,
			enableDatabasePrincipalsSummaryMetrics:   false,
			enableDatabasePrincipalsActivityMetrics:  false,
			expectedDatabasePrincipalsDetailsResult:  true,
			expectedDatabasePrincipalsSummaryResult:  true,
			expectedDatabasePrincipalsActivityResult: true,
		},
		{
			name:                                     "Only database principals details enabled individually",
			enableDatabasePrincipalsMetrics:          false,
			enableDatabasePrincipalsDetailsMetrics:   true,
			enableDatabasePrincipalsSummaryMetrics:   false,
			enableDatabasePrincipalsActivityMetrics:  false,
			expectedDatabasePrincipalsDetailsResult:  true,
			expectedDatabasePrincipalsSummaryResult:  false,
			expectedDatabasePrincipalsActivityResult: false,
		},
		{
			name:                                     "Only database principals summary enabled individually",
			enableDatabasePrincipalsMetrics:          false,
			enableDatabasePrincipalsDetailsMetrics:   false,
			enableDatabasePrincipalsSummaryMetrics:   true,
			enableDatabasePrincipalsActivityMetrics:  false,
			expectedDatabasePrincipalsDetailsResult:  false,
			expectedDatabasePrincipalsSummaryResult:  true,
			expectedDatabasePrincipalsActivityResult: false,
		},
		{
			name:                                     "Only database principals activity enabled individually",
			enableDatabasePrincipalsMetrics:          false,
			enableDatabasePrincipalsDetailsMetrics:   false,
			enableDatabasePrincipalsSummaryMetrics:   false,
			enableDatabasePrincipalsActivityMetrics:  true,
			expectedDatabasePrincipalsDetailsResult:  false,
			expectedDatabasePrincipalsSummaryResult:  false,
			expectedDatabasePrincipalsActivityResult: true,
		},
		{
			name:                                     "Main toggle enabled with some individual toggles also enabled",
			enableDatabasePrincipalsMetrics:          true,
			enableDatabasePrincipalsDetailsMetrics:   true,
			enableDatabasePrincipalsSummaryMetrics:   false,
			enableDatabasePrincipalsActivityMetrics:  true,
			expectedDatabasePrincipalsDetailsResult:  true,
			expectedDatabasePrincipalsSummaryResult:  true,
			expectedDatabasePrincipalsActivityResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				EnableDatabasePrincipalsMetrics:         tt.enableDatabasePrincipalsMetrics,
				EnableDatabasePrincipalsDetailsMetrics:  tt.enableDatabasePrincipalsDetailsMetrics,
				EnableDatabasePrincipalsSummaryMetrics:  tt.enableDatabasePrincipalsSummaryMetrics,
				EnableDatabasePrincipalsActivityMetrics: tt.enableDatabasePrincipalsActivityMetrics,
			}

			assert.Equal(t, tt.expectedDatabasePrincipalsDetailsResult, cfg.IsDatabasePrincipalsDetailsMetricsEnabled())
			assert.Equal(t, tt.expectedDatabasePrincipalsSummaryResult, cfg.IsDatabasePrincipalsSummaryMetricsEnabled())
			assert.Equal(t, tt.expectedDatabasePrincipalsActivityResult, cfg.IsDatabasePrincipalsActivityMetricsEnabled())
		})
	}
}

func TestGranularDatabaseRoleMembershipToggles(t *testing.T) {
	tests := []struct {
		name                                        string
		enableDatabaseRoleMembershipMetrics         bool
		enableDatabaseRoleMembershipDetailsMetrics  bool
		enableDatabaseRoleMembershipSummaryMetrics  bool
		enableDatabaseRoleHierarchyMetrics          bool
		enableDatabaseRoleActivityMetrics           bool
		enableDatabaseRolePermissionMatrixMetrics   bool
		expectedDatabaseRoleMembershipDetailsResult bool
		expectedDatabaseRoleMembershipSummaryResult bool
		expectedDatabaseRoleHierarchyResult         bool
		expectedDatabaseRoleActivityResult          bool
		expectedDatabaseRolePermissionMatrixResult  bool
	}{
		{
			name:                                "All database role membership metrics disabled",
			enableDatabaseRoleMembershipMetrics: false,
			enableDatabaseRoleMembershipDetailsMetrics:  false,
			enableDatabaseRoleMembershipSummaryMetrics:  false,
			enableDatabaseRoleHierarchyMetrics:          false,
			enableDatabaseRoleActivityMetrics:           false,
			enableDatabaseRolePermissionMatrixMetrics:   false,
			expectedDatabaseRoleMembershipDetailsResult: false,
			expectedDatabaseRoleMembershipSummaryResult: false,
			expectedDatabaseRoleHierarchyResult:         false,
			expectedDatabaseRoleActivityResult:          false,
			expectedDatabaseRolePermissionMatrixResult:  false,
		},
		{
			name:                                "Main database role membership toggle enabled only",
			enableDatabaseRoleMembershipMetrics: true,
			enableDatabaseRoleMembershipDetailsMetrics:  false,
			enableDatabaseRoleMembershipSummaryMetrics:  false,
			enableDatabaseRoleHierarchyMetrics:          false,
			enableDatabaseRoleActivityMetrics:           false,
			enableDatabaseRolePermissionMatrixMetrics:   false,
			expectedDatabaseRoleMembershipDetailsResult: true,
			expectedDatabaseRoleMembershipSummaryResult: true,
			expectedDatabaseRoleHierarchyResult:         true,
			expectedDatabaseRoleActivityResult:          true,
			expectedDatabaseRolePermissionMatrixResult:  true,
		},
		{
			name:                                "Only database role membership details enabled individually",
			enableDatabaseRoleMembershipMetrics: false,
			enableDatabaseRoleMembershipDetailsMetrics:  true,
			enableDatabaseRoleMembershipSummaryMetrics:  false,
			enableDatabaseRoleHierarchyMetrics:          false,
			enableDatabaseRoleActivityMetrics:           false,
			enableDatabaseRolePermissionMatrixMetrics:   false,
			expectedDatabaseRoleMembershipDetailsResult: true,
			expectedDatabaseRoleMembershipSummaryResult: false,
			expectedDatabaseRoleHierarchyResult:         false,
			expectedDatabaseRoleActivityResult:          false,
			expectedDatabaseRolePermissionMatrixResult:  false,
		},
		{
			name:                                "Mixed individual toggles enabled",
			enableDatabaseRoleMembershipMetrics: false,
			enableDatabaseRoleMembershipDetailsMetrics:  true,
			enableDatabaseRoleMembershipSummaryMetrics:  false,
			enableDatabaseRoleHierarchyMetrics:          true,
			enableDatabaseRoleActivityMetrics:           false,
			enableDatabaseRolePermissionMatrixMetrics:   true,
			expectedDatabaseRoleMembershipDetailsResult: true,
			expectedDatabaseRoleMembershipSummaryResult: false,
			expectedDatabaseRoleHierarchyResult:         true,
			expectedDatabaseRoleActivityResult:          false,
			expectedDatabaseRolePermissionMatrixResult:  true,
		},
		{
			name:                                "Main toggle enabled with some individual toggles also enabled",
			enableDatabaseRoleMembershipMetrics: true,
			enableDatabaseRoleMembershipDetailsMetrics:  false,
			enableDatabaseRoleMembershipSummaryMetrics:  true,
			enableDatabaseRoleHierarchyMetrics:          false,
			enableDatabaseRoleActivityMetrics:           true,
			enableDatabaseRolePermissionMatrixMetrics:   false,
			expectedDatabaseRoleMembershipDetailsResult: true,
			expectedDatabaseRoleMembershipSummaryResult: true,
			expectedDatabaseRoleHierarchyResult:         true,
			expectedDatabaseRoleActivityResult:          true,
			expectedDatabaseRolePermissionMatrixResult:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				EnableDatabaseRoleMembershipMetrics:        tt.enableDatabaseRoleMembershipMetrics,
				EnableDatabaseRoleMembershipDetailsMetrics: tt.enableDatabaseRoleMembershipDetailsMetrics,
				EnableDatabaseRoleMembershipSummaryMetrics: tt.enableDatabaseRoleMembershipSummaryMetrics,
				EnableDatabaseRoleHierarchyMetrics:         tt.enableDatabaseRoleHierarchyMetrics,
				EnableDatabaseRoleActivityMetrics:          tt.enableDatabaseRoleActivityMetrics,
				EnableDatabaseRolePermissionMatrixMetrics:  tt.enableDatabaseRolePermissionMatrixMetrics,
			}

			assert.Equal(t, tt.expectedDatabaseRoleMembershipDetailsResult, cfg.IsDatabaseRoleMembershipDetailsMetricsEnabled())
			assert.Equal(t, tt.expectedDatabaseRoleMembershipSummaryResult, cfg.IsDatabaseRoleMembershipSummaryMetricsEnabled())
			assert.Equal(t, tt.expectedDatabaseRoleHierarchyResult, cfg.IsDatabaseRoleHierarchyMetricsEnabled())
			assert.Equal(t, tt.expectedDatabaseRoleActivityResult, cfg.IsDatabaseRoleActivityMetricsEnabled())
			assert.Equal(t, tt.expectedDatabaseRolePermissionMatrixResult, cfg.IsDatabaseRolePermissionMatrixMetricsEnabled())
		})
	}
}

func TestLogicalORBehavior(t *testing.T) {
	testCases := []struct {
		name        string
		mainToggle  bool
		individual  bool
		expected    bool
		description string
	}{
		{
			name:        "both_false",
			mainToggle:  false,
			individual:  false,
			expected:    false,
			description: "false OR false = false",
		},
		{
			name:        "main_true_individual_false",
			mainToggle:  true,
			individual:  false,
			expected:    true,
			description: "true OR false = true",
		},
		{
			name:        "main_false_individual_true",
			mainToggle:  false,
			individual:  true,
			expected:    true,
			description: "false OR true = true",
		},
		{
			name:        "both_true",
			mainToggle:  true,
			individual:  true,
			expected:    true,
			description: "true OR true = true",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &Config{
				EnableFailoverClusterMetrics:        tc.mainToggle,
				EnableFailoverClusterReplicaMetrics: tc.individual,
			}

			result := config.IsFailoverClusterReplicaMetricsEnabled()
			assert.Equal(t, tc.expected, result, "Logical OR behavior failed: %s", tc.description)
		})
	}
}

// TestProductionUseCaseScenarios tests real-world usage scenarios
func TestProductionUseCaseScenarios(t *testing.T) {
	t.Run("comprehensive_monitoring", func(t *testing.T) {
		config := &Config{
			EnableFailoverClusterMetrics: true, // Enable everything
		}

		// All metrics should be enabled when main toggle is true
		assert.True(t, config.IsFailoverClusterReplicaMetricsEnabled())
		assert.True(t, config.IsFailoverClusterReplicaStateMetricsEnabled())
		assert.True(t, config.IsFailoverClusterNodeMetricsEnabled())
		assert.True(t, config.IsFailoverClusterAvailabilityGroupHealthMetricsEnabled())
		assert.True(t, config.IsFailoverClusterAvailabilityGroupMetricsEnabled())
		assert.True(t, config.IsFailoverClusterPerformanceCounterMetricsEnabled())
		assert.True(t, config.IsFailoverClusterClusterPropertiesMetricsEnabled())
	})

	t.Run("production_critical_only", func(t *testing.T) {
		config := &Config{
			EnableFailoverClusterMetrics:                        false, // Use granular control
			EnableFailoverClusterReplicaMetrics:                 true,  // Critical
			EnableFailoverClusterReplicaStateMetrics:            true,  // Critical
			EnableFailoverClusterAvailabilityGroupHealthMetrics: true,  // Critical
			EnableFailoverClusterAvailabilityGroupMetrics:       true,  // Important
			EnableFailoverClusterNodeMetrics:                    true,  // Important
			EnableFailoverClusterPerformanceCounterMetrics:      false, // Optional
			EnableFailoverClusterClusterPropertiesMetrics:       false, // Optional
		}

		// Critical metrics enabled
		assert.True(t, config.IsFailoverClusterReplicaMetricsEnabled())
		assert.True(t, config.IsFailoverClusterReplicaStateMetricsEnabled())
		assert.True(t, config.IsFailoverClusterAvailabilityGroupHealthMetricsEnabled())
		assert.True(t, config.IsFailoverClusterAvailabilityGroupMetricsEnabled())
		assert.True(t, config.IsFailoverClusterNodeMetricsEnabled())

		// Optional metrics disabled
		assert.False(t, config.IsFailoverClusterPerformanceCounterMetricsEnabled())
		assert.False(t, config.IsFailoverClusterClusterPropertiesMetricsEnabled())
	})

	t.Run("development_lightweight", func(t *testing.T) {
		config := &Config{
			EnableFailoverClusterMetrics: false, // All granular toggles default to false
		}

		// All metrics should be disabled for lightweight development monitoring
		assert.False(t, config.IsFailoverClusterReplicaMetricsEnabled())
		assert.False(t, config.IsFailoverClusterReplicaStateMetricsEnabled())
		assert.False(t, config.IsFailoverClusterNodeMetricsEnabled())
		assert.False(t, config.IsFailoverClusterAvailabilityGroupHealthMetricsEnabled())
		assert.False(t, config.IsFailoverClusterAvailabilityGroupMetricsEnabled())
		assert.False(t, config.IsFailoverClusterPerformanceCounterMetricsEnabled())
		assert.False(t, config.IsFailoverClusterClusterPropertiesMetricsEnabled())
	})

	t.Run("single_metric_debugging", func(t *testing.T) {
		config := &Config{
			EnableFailoverClusterMetrics:        false, // Use granular control
			EnableFailoverClusterReplicaMetrics: true,  // Enable only replica metrics for debugging
		}

		// Only replica metrics enabled
		assert.True(t, config.IsFailoverClusterReplicaMetricsEnabled())

		// All others disabled
		assert.False(t, config.IsFailoverClusterReplicaStateMetricsEnabled())
		assert.False(t, config.IsFailoverClusterNodeMetricsEnabled())
		assert.False(t, config.IsFailoverClusterAvailabilityGroupHealthMetricsEnabled())
		assert.False(t, config.IsFailoverClusterAvailabilityGroupMetricsEnabled())
		assert.False(t, config.IsFailoverClusterPerformanceCounterMetricsEnabled())
		assert.False(t, config.IsFailoverClusterClusterPropertiesMetricsEnabled())
	})
}
