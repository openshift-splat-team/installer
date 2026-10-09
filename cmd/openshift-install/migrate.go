package main

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/installer/pkg/asset/installconfig/vsphere"
)

// newMigrateCmd creates the migrate command for vSphere brownfield migration.
func newMigrateCmd() *cobra.Command {
	migrateCmd := &cobra.Command{
		Use:   "vsphere",
		Short: "Commands for vSphere specific operations",
	}

	migrateToPerComponentCmd := &cobra.Command{
		Use:   "migrate-to-per-component",
		Short: "Migrate existing passthrough-mode cluster to per-component credential mode",
		Long: `Migrate an existing OpenShift cluster from legacy passthrough credential mode
to per-component credential mode.

This command:
  1. Reads per-component credentials from the credentials file
  2. Validates each component's credentials have required privileges
  3. Creates backup of original passthrough secret
  4. Creates component-specific secrets in appropriate namespaces
  5. Updates CCO configuration to per-component mode
  6. Restarts component operators to pick up new credentials
  7. Verifies each component reconnects successfully
  8. On failure: rolls back to original passthrough-mode configuration

Example:
  openshift-install vsphere migrate-to-per-component \
    --kubeconfig=/path/to/kubeconfig \
    --credentials-file=~/.vsphere/credentials`,
		RunE: runMigrateToPerComponent,
	}

	migrateToPerComponentCmd.Flags().String("kubeconfig", "", "Path to kubeconfig file")
	migrateToPerComponentCmd.Flags().String("credentials-file", "", "Path to vSphere credentials file (default: ~/.vsphere/credentials)")
	migrateToPerComponentCmd.Flags().Bool("validate-privileges", true, "Validate component privileges before migration")
	migrateToPerComponentCmd.MarkFlagRequired("kubeconfig")

	migrateCmd.AddCommand(migrateToPerComponentCmd)
	return migrateCmd
}

// runMigrateToPerComponent executes the migration from passthrough to per-component mode.
func runMigrateToPerComponent(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Parse flags
	kubeconfigPath, _ := cmd.Flags().GetString("kubeconfig")
	credentialsFilePath, _ := cmd.Flags().GetString("credentials-file")
	validatePrivileges, _ := cmd.Flags().GetBool("validate-privileges")

	logrus.Info("Starting migration from passthrough to per-component credential mode")

	// 1. Load kubeconfig and create Kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// 2. Load credentials file
	credsFile, err := vsphere.LoadCredentialsFile(credentialsFilePath)
	if err != nil {
		return fmt.Errorf("failed to load credentials file: %w", err)
	}

	if credsFile == nil {
		return fmt.Errorf("credentials file not found or empty")
	}

	// 3. Create migrator
	migrator := vsphere.NewBrownfieldMigrator(clientset, validatePrivileges)

	// 4. Execute migration
	if err := migrator.Migrate(ctx, credsFile); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	logrus.Info("Migration completed successfully")
	return nil
}

func init() {
	// Add migrate command to root command
	// Note: In a real implementation, this would be added to rootCmd in main.go
	// For testing purposes, we export the command constructor
}
