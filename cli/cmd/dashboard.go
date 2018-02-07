package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/pkg/browser"
	"github.com/runconduit/conduit/pkg/k8s"
	"github.com/runconduit/conduit/pkg/shell"
	"github.com/spf13/cobra"
)

var dashboardProxyPort int
var dashboardSkipBrowser bool

var dashboardCmd = &cobra.Command{
	Use:   "dashboard [flags]",
	Short: "Open the Conduit dashboard in a web browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dashboardProxyPort < 0 {
			return fmt.Errorf("port must be greater than or equal to zero, was %d", dashboardProxyPort)
		}

		shellHomeDir := shell.NewUnixShell().HomeDir()
		kubernetesProxy, err := k8s.InitK8sProxy(shellHomeDir, kubeconfigPath, dashboardProxyPort)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize proxy: %s\n", err)
			os.Exit(1)
		}

		kubeAPI, err := k8s.NewK8sAPI(shellHomeDir, kubeconfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize kubernetes API: %+v", err)
			os.Exit(1)
		}

		url, err := kubernetesProxy.URLFor(controlPlaneNamespace, "/services/web:http/proxy/")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate URL for dashboard: %s\n", err)
			os.Exit(1)
		}

		dashboardAvailable, err := isDashboardAvailable(kubeAPI)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed while checking availability of dashboard: %+v\n", err)
		}

		if !dashboardAvailable {
			fmt.Printf("Conduit dashboard is not installed in cluster")
			os.Exit(1)
		}

		fmt.Printf("Conduit dashboard available at:\n%s\n", url.String())

		if !dashboardSkipBrowser {
			fmt.Println("Opening the default browser")

			err = browser.OpenURL(url.String())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open URL %s in the default browser: %s", url, err)
				os.Exit(1)
			}
		}

		// blocks until killed
		err = kubernetesProxy.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running proxy: %s", err)
			os.Exit(1)
		}

		return nil
	},
}

func isDashboardAvailable(kubeAPI k8s.KubernetesApi) (bool, error) {
	client, err := kubeAPI.NewClient()
	if err != nil {
		return false, err
	}
	fullUrl, err := kubeAPI.UrlFor(controlPlaneNamespace, "/services/web")
	if err != nil {
		return false, err
	}
	req, err := client.Get(fullUrl.String())
	if err != nil {
		return false, err
	}
	if req.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, nil

}

func init() {
	RootCmd.AddCommand(dashboardCmd)
	addControlPlaneNetworkingArgs(dashboardCmd)
	dashboardCmd.Args = cobra.NoArgs

	// This is identical to what `kubectl proxy --help` reports, `--port 0`
	// indicates a random port.
	dashboardCmd.PersistentFlags().IntVarP(&dashboardProxyPort, "port", "p", 8001, "The port on which to run the proxy. Set to 0 to pick a random port.")
	dashboardCmd.PersistentFlags().BoolVar(&dashboardSkipBrowser, "url", false, "Display the Conduit dashboard URL in the CLI instead of opening it in the default browser")
}
