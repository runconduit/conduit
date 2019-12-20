package linkerd2

import (
	"reflect"
	"testing"
)

func TestNewValues(t *testing.T) {
	actual, err := NewValues(false)
	if err != nil {
		t.Fatalf("Unexpected error: %v\n", err)
	}

	testVersion := "linkerd-dev"

	expected := &Values{
		Stage:                       "",
		ControllerImage:             "gcr.io/linkerd-io/controller",
		ControllerImageVersion:      testVersion,
		WebImage:                    "gcr.io/linkerd-io/web",
		PrometheusImage:             "prom/prometheus:v2.11.1",
		GrafanaImage:                "gcr.io/linkerd-io/grafana",
		ControllerReplicas:          1,
		ControllerLogLevel:          "info",
		PrometheusLogLevel:          "info",
		ProxyContainerName:          "linkerd-proxy",
		ControllerUID:               2103,
		EnableH2Upgrade:             true,
		EnablePodAntiAffinity:       false,
		NoInitContainer:             false,
		WebhookFailurePolicy:        "Ignore",
		OmitWebhookSideEffects:      false,
		RestrictDashboardPrivileges: false,
		DisableHeartBeat:            false,
		HeartbeatSchedule:           "0 0 * * *",
		InstallNamespace:            true,
		Global: &Global{
			Namespace:                "linkerd",
			ClusterDomain:            "cluster.local",
			ImagePullPolicy:          "IfNotPresent",
			CliVersion:               "linkerd/cli dev-undefined",
			ControllerComponentLabel: "linkerd.io/control-plane-component",
			ControllerNamespaceLabel: "linkerd.io/control-plane-ns",
			CreatedByAnnotation:      "linkerd.io/created-by",
			ProxyInjectAnnotation:    "linkerd.io/inject",
			ProxyInjectDisabled:      "disabled",
			LinkerdNamespaceLabel:    "linkerd.io/is-control-plane",
			HighAvailability:         false,
			IdentityTrustDomain:      "cluster.local",
			Proxy: &Proxy{
				EnableExternalProfiles: false,
				Image: &Image{
					Name:       "gcr.io/linkerd-io/proxy",
					PullPolicy: "IfNotPresent",
					Version:    testVersion,
				},
				LogLevel: "warn,linkerd2_proxy=info",
				Ports: &Ports{
					Admin:    4191,
					Control:  4190,
					Inbound:  4143,
					Outbound: 4140,
				},
				Resources: &Resources{
					CPU: Constraints{
						Limit:   "",
						Request: "",
					},
					Memory: Constraints{
						Limit:   "",
						Request: "",
					},
				},
				Trace: &Trace{
					CollectorSvcAddr:    "",
					CollectorSvcAccount: "default",
				},
				UID:                   2102,
				WaitBeforeExitSeconds: 0,
			},
			ProxyInit: &ProxyInit{
				Image: &Image{
					Name:       "gcr.io/linkerd-io/proxy-init",
					PullPolicy: "IfNotPresent",
					Version:    testVersion,
				},
				Resources: &Resources{
					CPU: Constraints{
						Limit:   "100m",
						Request: "10m",
					},
					Memory: Constraints{
						Limit:   "50Mi",
						Request: "10Mi",
					},
				},
			},
		},
		Identity: &Identity{
			Issuer: &Issuer{
				ClockSkewAllowance:  "20s",
				IssuanceLifetime:    "86400s",
				CrtExpiryAnnotation: "linkerd.io/identity-issuer-expiry",
				TLS:                 &TLS{},
				Scheme:              "linkerd.io/tls",
			},
		},
		NodeSelector: map[string]string{
			"beta.kubernetes.io/os": "linux",
		},

		Dashboard: &Dashboard{
			Replicas: 1,
		},

		ProxyInjector:    &ProxyInjector{TLS: &TLS{}},
		ProfileValidator: &ProfileValidator{TLS: &TLS{}},
		Tap:              &Tap{TLS: &TLS{}},

		ControlPlaneTracing: false,
	}

	// pin the versions to ensure consistent test result.
	// in non-test environment, the default versions are read from the
	// values.yaml.
	actual.ControllerImageVersion = testVersion
	actual.Global.Proxy.Image.Version = testVersion
	actual.Global.ProxyInit.Image.Version = testVersion

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Mismatch Helm values.\nExpected: %+v\nActual: %+v", expected, actual)
	}

	t.Run("HA", func(t *testing.T) {
		actual, err := NewValues(true)
		if err != nil {
			t.Fatalf("Unexpected error: %v\n", err)
		}

		expected.ControllerReplicas = 3
		expected.EnablePodAntiAffinity = true
		expected.WebhookFailurePolicy = "Fail"

		controllerResources := &Resources{
			CPU: Constraints{
				Limit:   "1",
				Request: "100m",
			},
			Memory: Constraints{
				Limit:   "250Mi",
				Request: "50Mi",
			},
		}
		expected.DestinationResources = controllerResources
		expected.PublicAPIResources = controllerResources
		expected.ProxyInjectorResources = controllerResources
		expected.SPValidatorResources = controllerResources
		expected.TapResources = controllerResources
		expected.WebResources = controllerResources
		expected.HeartbeatResources = controllerResources

		expected.GrafanaResources = &Resources{
			CPU: Constraints{
				Limit:   controllerResources.CPU.Limit,
				Request: controllerResources.CPU.Request,
			},
			Memory: Constraints{
				Limit:   "1024Mi",
				Request: "50Mi",
			},
		}

		expected.IdentityResources = &Resources{
			CPU: Constraints{
				Limit:   controllerResources.CPU.Limit,
				Request: controllerResources.CPU.Request,
			},
			Memory: Constraints{
				Limit:   controllerResources.Memory.Limit,
				Request: "10Mi",
			},
		}

		expected.PrometheusResources = &Resources{
			CPU: Constraints{
				Limit:   "4",
				Request: "300m",
			},
			Memory: Constraints{
				Limit:   "8192Mi",
				Request: "300Mi",
			},
		}

		expected.Global.Proxy.Resources = &Resources{
			CPU: Constraints{
				Limit:   controllerResources.CPU.Limit,
				Request: controllerResources.CPU.Request,
			},
			Memory: Constraints{
				Limit:   controllerResources.Memory.Limit,
				Request: "20Mi",
			},
		}

		// pin the versions to ensure consistent test result.
		// in non-test environment, the default versions are read from the
		// values.yaml.
		actual.ControllerImageVersion = testVersion
		actual.Global.Proxy.Image.Version = testVersion
		actual.Global.ProxyInit.Image.Version = testVersion

		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("Mismatch Helm HA defaults.\nExpected: %+v\nActual: %+v", expected, actual)
		}
	})
}
