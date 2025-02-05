package main

import (
	"context"
	"fmt"
	"os"

	"github.com/thegeeklab/renovate-operator/dispatcher"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	ErrReadFile  = fmt.Errorf("failed to read file")
	ErrWriteFile = fmt.Errorf("failed to write file")
)

func main() {
	logf.SetLogger(zap.New(zap.JSONEncoder()))

	if err := run(context.Background()); err != nil {
		logf.Log.Error(err, "Failed to run dispatcher")
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctxLogger := logf.FromContext(ctx)

	d, err := dispatcher.New()
	if err != nil {
		return err
	}

	ctxLogger.Info("Dispatch batch")

	rawConfig, err := os.ReadFile(d.RawConfigFile)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrReadFile, d.RawConfigFile, err)
	}

	ctxLogger.V(1).Info("Read raw renovate config", "content", rawConfig)

	batchesConfig, err := os.ReadFile(d.BatchesFile)
	if err != nil {
		return fmt.Errorf("%w: %s, %w", ErrReadFile, d.BatchesFile, err)
	}

	ctxLogger.V(1).Info("Read batches config", "content", batchesConfig)

	mergedConfig, err := d.MergeConfig(rawConfig, batchesConfig, int(d.JobCompletionIndex))
	if err != nil {
		return err
	}

	err = os.WriteFile(d.ConfigFile, mergedConfig, 0o644) //nolint:gosec,mnd
	if err != nil {
		return fmt.Errorf("%w: %s, %w", ErrWriteFile, d.ConfigFile, err)
	}

	return nil
}
