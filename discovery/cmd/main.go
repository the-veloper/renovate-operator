package main

import (
	"context"
	"fmt"
	"os"

	renovatev1beta1 "github.com/thegeeklab/renovate-operator/api/v1beta1"
	"github.com/thegeeklab/renovate-operator/discovery"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme = runtime.NewScheme()

	ErrReadDiscoveryFile = fmt.Errorf("failed to read discovery file")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(renovatev1beta1.AddToScheme(scheme))
}

func main() {
	logf.SetLogger(zap.New(zap.JSONEncoder()))

	if err := run(context.Background()); err != nil {
		logf.Log.Error(err, "Failed to run discovery")
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctxLogger := logf.FromContext(ctx)

	d, err := discovery.New(scheme)
	if err != nil {
		return err
	}

	ctxLogger = ctxLogger.WithValues("namespace", d.Namespace, "name", d.Name)

	discoveredRepos, err := readDiscoveryFile(d.FilePath)
	if err != nil {
		return err
	}

	ctxLogger.Info("Repository list", "repositories", discoveredRepos)

	// Get renovator instance as owner ref
	renovator := &renovatev1beta1.Renovator{}

	renovatorName := types.NamespacedName{
		Namespace: d.Namespace,
		Name:      d.Name,
	}

	if err := d.Client.Get(ctx, renovatorName, renovator); err != nil {
		return err
	}

	// Create GitRepo CRs for discovered repos
	discoveredRepoMatcher := make(map[string]bool)

	for _, repo := range discoveredRepos {
		discoveredRepoMatcher[repo] = true

		r := discovery.CreateGitRepo(renovator, d.Namespace, repo)

		err := d.Client.Create(ctx, r)
		if err != nil && !errors.IsAlreadyExists(err) {
			ctxLogger.Error(err, "Failed to create GitRepo", "repo", repo)
		}
	}

	// Clean up removed repos
	existingRepos := &renovatev1beta1.GitRepoList{}
	if err := d.Client.List(ctx, existingRepos, client.InNamespace(d.Namespace)); err != nil {
		return err
	}

	for _, repo := range existingRepos.Items {
		if discoveredRepoMatcher[repo.Spec.Name] || !metav1.IsControlledBy(&repo, renovator) {
			continue
		}

		if err := d.Client.Delete(ctx, &repo); err != nil {
			ctxLogger.Error(err, "Failed to delete GitRepo", "repo", repo.Name)

			continue
		}

		ctxLogger.Info("Deleted GitRepo", "repo", repo.Name)
	}

	return nil
}

func readDiscoveryFile(path string) ([]string, error) {
	readBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var repos []string

	if err := json.Unmarshal(readBytes, &repos); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReadDiscoveryFile, err)
	}

	return repos, nil
}
