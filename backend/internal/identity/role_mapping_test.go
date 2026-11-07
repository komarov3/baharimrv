package identity

import "testing"

// mrvCapSlice builds a []mrvHubCap for table tests.
func mrvCapSlice(types ...string) []mrvHubCap {
	caps := make([]mrvHubCap, 0, len(types))
	for _, t := range types {
		caps = append(caps, mrvHubCap{Type: t, ObjectID: "test-obj-id"})
	}
	return caps
}

func TestMapCapsToMRVRole(t *testing.T) {
	tests := []struct {
		name string
		caps []mrvHubCap
		want string
	}{
		{name: "FisherCap → field_monitor", caps: mrvCapSlice("FisherCap"), want: "field_monitor"},
		{name: "OfficerCap → independent_verifier", caps: mrvCapSlice("OfficerCap"), want: "independent_verifier"},
		{name: "ManagerCap → project_manager", caps: mrvCapSlice("ManagerCap"), want: "project_manager"},
		{name: "MonitorCap → field_monitor", caps: mrvCapSlice("MonitorCap"), want: "field_monitor"},
		{name: "VerifierCap → independent_verifier", caps: mrvCapSlice("VerifierCap"), want: "independent_verifier"},
		{name: "empty caps → empty string", caps: mrvCapSlice(), want: ""},
		{name: "nil caps → empty string", caps: nil, want: ""},
		{name: "unknown cap type → empty string", caps: mrvCapSlice("BuyerCap"), want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapCapsToMRVRole(tt.caps)
			if got != tt.want {
				t.Errorf("MapCapsToMRVRole(%v) = %q, want %q", tt.caps, got, tt.want)
			}
		})
	}
}
