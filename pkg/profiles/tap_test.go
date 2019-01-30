package profiles

import (
	"testing"
	"time"

	"github.com/linkerd/linkerd2/controller/api/public"
	"github.com/linkerd/linkerd2/controller/api/util"
	sp "github.com/linkerd/linkerd2/controller/gen/apis/serviceprofile/v1alpha1"
	pb "github.com/linkerd/linkerd2/controller/gen/public"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// func TestProfileFromTap(t *testing.T) {
// 	var buf bytes.Buffer
// 	options := newProfileOptions()
// 	options.name = "service-name"
// 	options.namespace = "service-namespace"
// 	options.tap = "not-a-resource/web"

// 	err := renderTapOutputProfile(options, controlPlaneNamespace, &buf)
// 	exp := errors.New("target resource invalid: cannot find Kubernetes canonical name from friendly name [not-a-resource]")

// 	if err.Error() != exp.Error() {
// 		t.Fatalf("renderTapOutputProfile returned unexpected error: %s (expected: %s)", err, exp)
// 	}
// }
func TestTapToServiceProfile(t *testing.T) {
	name := "service-name"
	namespace := "service-namespace"
	tapDuration := 5 * time.Second
	routeLimit := 20

	controlPlaneNamespace := "linkerd"

	params := util.TapRequestParams{
		Resource:  "deploy/" + name,
		Namespace: namespace,
	}

	tapReq, err := util.BuildTapByResourceRequest(params)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	event1 := util.CreateTapEvent(
		&pb.TapEvent_Http{
			Event: &pb.TapEvent_Http_RequestInit_{
				RequestInit: &pb.TapEvent_Http_RequestInit{
					Id: &pb.TapEvent_Http_StreamId{
						Base: 1,
					},
					Authority: "",
					Path:      "/emojivoto.v1.VotingService/VoteFire",
					Method: &pb.HttpMethod{
						Type: &pb.HttpMethod_Registered_{
							Registered: pb.HttpMethod_POST,
						},
					},
				},
			},
		},
		map[string]string{},
	)

	event2 := util.CreateTapEvent(
		&pb.TapEvent_Http{
			Event: &pb.TapEvent_Http_RequestInit_{
				RequestInit: &pb.TapEvent_Http_RequestInit{
					Id: &pb.TapEvent_Http_StreamId{
						Base: 2,
					},
					Authority: "",
					Path:      "/my/path/hi",
					Method: &pb.HttpMethod{
						Type: &pb.HttpMethod_Registered_{
							Registered: pb.HttpMethod_GET,
						},
					},
				},
			},
		},
		map[string]string{},
	)

	mockAPIClient := &public.MockAPIClient{}
	mockAPIClient.APITapByResourceClientToReturn = &public.MockAPITapByResourceClient{
		TapEventsToReturn: []pb.TapEvent{event1, event2},
	}

	expectedServiceProfile := sp.ServiceProfile{
		TypeMeta: ServiceProfileMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "." + namespace + ".svc.cluster.local",
			Namespace: controlPlaneNamespace,
		},
		Spec: sp.ServiceProfileSpec{
			Routes: []*sp.RouteSpec{
				&sp.RouteSpec{
					Name: "GET /my/path/hi",
					Condition: &sp.RequestMatch{
						PathRegex: `/my/path/hi`,
						Method:    "GET",
					},
				},
				&sp.RouteSpec{
					Name: "POST /emojivoto.v1.VotingService/VoteFire",
					Condition: &sp.RequestMatch{
						PathRegex: `/emojivoto\.v1\.VotingService/VoteFire`,
						Method:    "POST",
					},
				},
			},
		},
	}

	actualServiceProfile, err := tapToServiceProfile(mockAPIClient, tapReq, controlPlaneNamespace, tapDuration, int(routeLimit))
	if err != nil {
		t.Fatalf("Failed to create ServiceProfile: %v", err)
	}

	err = ServiceProfileYamlEquals(actualServiceProfile, expectedServiceProfile)
	if err != nil {
		t.Fatalf("ServiceProfiles are not equal: %v", err)
	}
}
