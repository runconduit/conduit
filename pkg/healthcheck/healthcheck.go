package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/linkerd/linkerd2/controller/api/public"
	healthcheckPb "github.com/linkerd/linkerd2/controller/gen/common/healthcheck"
	pb "github.com/linkerd/linkerd2/controller/gen/public"
	"github.com/linkerd/linkerd2/pkg/k8s"
	"github.com/linkerd/linkerd2/pkg/version"
	k8sVersion "k8s.io/apimachinery/pkg/version"
)

const (
	KubernetesAPICategory  = "kubernetes-api"
	LinkerdAPICategory     = "linkerd-api"
	LinkerdVersionCategory = "linkerd-version"
)

type checker struct {
	category    string
	description string
	fatal       bool
	check       func() error
	checkRPC    func() (*healthcheckPb.SelfCheckResponse, error)
}

type checkObserver func(string, string, error)

type HealthChecker struct {
	checkers      []*checker
	kubeAPI       *k8s.KubernetesAPI
	httpClient    *http.Client
	kubeVersion   *k8sVersion.Info
	apiClient     pb.ApiClient
	latestVersion string
}

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checkers: make([]*checker, 0),
	}
}

// AddKubernetesAPIChecks adds a series of checks to validate that the caller is
// configured to interact with a working Kubernetes cluster and that the cluster
// meets the minimum version requirement.
func (hc *HealthChecker) AddKubernetesAPIChecks(kubeconfigPath string) {
	hc.checkers = append(hc.checkers, &checker{
		category:    KubernetesAPICategory,
		description: "can initialize the client",
		fatal:       true,
		check: func() (err error) {
			hc.kubeAPI, err = k8s.NewAPI(kubeconfigPath)
			return
		},
	})

	hc.checkers = append(hc.checkers, &checker{
		category:    KubernetesAPICategory,
		description: "can query the Kubernetes API",
		fatal:       true,
		check: func() (err error) {
			hc.httpClient, err = hc.kubeAPI.NewClient()
			if err != nil {
				return
			}
			hc.kubeVersion, err = hc.kubeAPI.GetVersionInfo(hc.httpClient)
			return
		},
	})

	hc.checkers = append(hc.checkers, &checker{
		category:    KubernetesAPICategory,
		description: "is running the minimum Kubernetes API version",
		fatal:       false,
		check: func() error {
			return hc.kubeAPI.CheckVersion(hc.kubeVersion)
		},
	})
}

// AddLinkerdAPIChecks adds a series of checks to validate that the control
// plane namespace exists and that it's successfully serving the public API.
// These checks are dependent on the output of AddKubernetesAPIChecks, so those
// checks must be added first.
func (hc *HealthChecker) AddLinkerdAPIChecks(apiAddr, controlPlaneNamespace string) {
	hc.checkers = append(hc.checkers, &checker{
		category:    LinkerdAPICategory,
		description: "control plane namespace exists",
		fatal:       true,
		check: func() error {
			return hc.kubeAPI.CheckNamespaceExists(hc.httpClient, controlPlaneNamespace)
		},
	})

	hc.checkers = append(hc.checkers, &checker{
		category:    LinkerdAPICategory,
		description: "can initialize the client",
		fatal:       true,
		check: func() (err error) {
			if apiAddr != "" {
				hc.apiClient, err = public.NewInternalClient(controlPlaneNamespace, apiAddr)
			} else {
				hc.apiClient, err = public.NewExternalClient(controlPlaneNamespace, hc.kubeAPI)
			}
			return
		},
	})

	hc.checkers = append(hc.checkers, &checker{
		category:    LinkerdAPICategory,
		description: "can query the control plane API",
		fatal:       true,
		checkRPC: func() (*healthcheckPb.SelfCheckResponse, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return hc.apiClient.SelfCheck(ctx, &healthcheckPb.SelfCheckRequest{})
		},
	})
}

// AddLinkerdVersionChecks adds a series of checks to validate that the CLI and
// control plane are running the latest available version. These checks are
// dependent on the output of AddLinkerdAPIChecks, so those checks must be added
// first.
func (hc *HealthChecker) AddLinkerdVersionChecks(versionOverride string) {
	hc.checkers = append(hc.checkers, &checker{
		category:    LinkerdVersionCategory,
		description: "can get the latest version",
		fatal:       true,
		check: func() (err error) {
			if versionOverride != "" {
				hc.latestVersion = versionOverride
			} else {
				hc.latestVersion, err = version.GetLatestVersion()
			}
			return
		},
	})

	hc.checkers = append(hc.checkers, &checker{
		category:    LinkerdVersionCategory,
		description: "cli is up-to-date",
		fatal:       false,
		check: func() error {
			return version.CheckClientVersion(hc.latestVersion)
		},
	})

	hc.checkers = append(hc.checkers, &checker{
		category:    LinkerdVersionCategory,
		description: "control plane is up-to-date",
		fatal:       false,
		check: func() error {
			return version.CheckServerVersion(hc.apiClient, hc.latestVersion)
		},
	})
}

// Add adds an arbitrary checker. This should only be used for testing. For
// production code, add sets of checkers using the `Add*` functions above.
func (hc *HealthChecker) Add(category, description string, check func() error) {
	hc.checkers = append(hc.checkers, &checker{
		category:    category,
		description: description,
		check:       check,
	})
}

// RunChecks runs all configured checkers, and passes the results of each
// check to the observer. If a check fails and is marked as fatal, then all
// remaining checks are skipped. If at least one check fails, RunChecks returns
// false; if all checks passed, RunChecks returns true.
func (hc *HealthChecker) RunChecks(observer checkObserver) bool {
	success := true

	for _, checker := range hc.checkers {
		if checker.check != nil {
			err := checker.check()
			observer(checker.category, checker.description, err)
			if err != nil {
				success = false
				if checker.fatal {
					break
				}
			}
		}

		if checker.checkRPC != nil {
			checkRsp, err := checker.checkRPC()
			observer(checker.category, checker.description, err)
			if err != nil {
				success = false
				if checker.fatal {
					break
				}
				continue
			}

			for _, check := range checkRsp.Results {
				category := fmt.Sprintf("%s[%s]", checker.category, check.SubsystemName)
				var err error
				if check.Status != healthcheckPb.CheckStatus_OK {
					success = false
					err = fmt.Errorf(check.FriendlyMessageToUser)
				}
				observer(category, check.CheckDescription, err)
			}
		}
	}

	return success
}

// PublicAPIClient returns a fully configured public API client. This client
// is only configured if the AddKubernetesAPIChecks, AddLinkerdAPIChecks, and
// RunChecks functions have already been called.
func (hc *HealthChecker) PublicAPIClient() pb.ApiClient {
	return hc.apiClient
}
