// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/newrelic/newrelic-infra-operator/internal/cli"
)

const (
	testConfig = `
infraAgentInjection:
  customAttributes:
  - name: computeType
    defaultValue: serverless
`
)

//nolint:funlen,gocognit,cyclop
func Test_Options(t *testing.T) {
	t.Parallel()

	t.Run("configures_infra_agent_injection_with", func(t *testing.T) {
		t.Parallel()

		t.Run("configuration_from_config_file", func(t *testing.T) {
			t.Parallel()

			options, err := cli.Options(withTestConfigFile(t, testConfig))
			if err != nil {
				t.Fatalf("getting options: %v", err)
			}

			expectedName := "computeType"
			expectedDefaultValue := "serverless"

			if options.InfraAgentInjection.CustomAttributes == nil {
				t.Fatalf("expected agent config to not be empty")
			}

			if l := len(options.InfraAgentInjection.CustomAttributes); l != 1 {
				t.Fatalf("didn't find 1 customAttributes %d", l)
			}

			ca := options.InfraAgentInjection.CustomAttributes[0]
			if ca.Name != expectedName {
				t.Fatalf("expected name %q, got %q", expectedName, ca.Name)
			}

			if ca.DefaultValue != expectedDefaultValue {
				t.Fatalf("expected defaultValue %q, got %q", expectedDefaultValue, ca.DefaultValue)
			}
		})

		t.Run("license_key_from_environment_variable", func(t *testing.T) {
			t.Parallel()

			expectedLicenseKey := "foo"

			if err := os.Setenv(cli.EnvLicenseKey, expectedLicenseKey); err != nil {
				t.Fatalf("setting %q environment variable: %v", cli.EnvLicenseKey, err)
			}

			options, err := cli.Options(withTestConfigFile(t, testConfig))
			if err != nil {
				t.Fatalf("getting options: %v", err)
			}

			if licensekey := options.InfraAgentInjection.License; licensekey != expectedLicenseKey {
				t.Fatalf("expected license key %q, got %q", expectedLicenseKey, licensekey)
			}
		})

		t.Run("retains_cluster_name_when_set_in_configuration", func(t *testing.T) {
			expectedClusterName := "test-cluster"

			if err := os.Setenv(cli.EnvClusterName, "other-cluster-name"); err != nil {
				t.Fatalf("setting %q environment variable: %v", cli.EnvClusterName, err)
			}

			config := fmt.Sprintf(`
infraAgentInjection:
  clusterName: %s
`, expectedClusterName)

			options, err := cli.Options(withTestConfigFile(t, config))
			if err != nil {
				t.Fatalf("getting options: %v", err)
			}

			if clusterName := options.InfraAgentInjection.ClusterName; clusterName != expectedClusterName {
				t.Fatalf("expected cluster name %q, got %q", expectedClusterName, clusterName)
			}
		})

		t.Run("sets_cluster_name_from_environment_variable_if_not_set_in_configuration", func(t *testing.T) {
			expectedClusterName := "test-cluster"

			if err := os.Setenv(cli.EnvClusterName, expectedClusterName); err != nil {
				t.Fatalf("setting %q environment variable: %v", cli.EnvClusterName, err)
			}

			options, err := cli.Options(withTestConfigFile(t, ""))
			if err != nil {
				t.Fatalf("getting options: %v", err)
			}

			if clusterName := options.InfraAgentInjection.ClusterName; clusterName != expectedClusterName {
				t.Fatalf("expected cluster name %q, got %q", expectedClusterName, clusterName)
			}
		})
	})

	t.Run("returns_error_when", func(t *testing.T) {
		t.Parallel()

		t.Run("config_file_exists_but_it_is_not_readable", func(t *testing.T) {
			t.Parallel()

			configPath := withTestConfigFile(t, testConfig)

			if err := os.Chmod(configPath, 0o200); err != nil {
				t.Fatalf("changing config file %q permissions: %v", configPath, err)
			}

			options, err := cli.Options(configPath)
			if err == nil {
				t.Fatalf("expected error while getting options")
			}

			if options != nil {
				t.Fatalf("expected options to be empty when error occurs")
			}
		})

		t.Run("reading_malformed_file", func(t *testing.T) {
			t.Parallel()

			config := ":notvalidyaml"

			options, err := cli.Options(withTestConfigFile(t, config))
			if err == nil {
				t.Fatalf("expected error while getting options")
			}

			if options != nil {
				t.Fatalf("expected options to be empty when error occurs")
			}
		})

		t.Run("found_unknown_key_in_the_configuration", func(t *testing.T) {
			t.Parallel()

			config := `
nonExistingKey: foo
`
			options, err := cli.Options(withTestConfigFile(t, config))
			if err == nil {
				t.Fatalf("expected error while getting options")
			}

			if options != nil {
				t.Fatalf("expected options to be empty when error occurs")
			}
		})
	})
}

func withTestConfigFile(t *testing.T, config string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")

	if err := ioutil.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("writing test config file %q: %v", configPath, err)
	}

	return configPath
}
