package mrv

import (
	"testing"

	"github.com/baharilink/baharimrv/internal/satellite"
)

// The marker rule is an integrity boundary: a figure may only appear on a plot
// marker when it is genuinely that one plot's live measurement. These cases pin
// down every way a multi-plot aggregate, a sandbox synthetic, or an absent figure
// could otherwise leak onto a marker.
func TestSinglePlotMarkerNDVI(t *testing.T) {
	mean := 0.42
	live := satellite.ChangeView{
		MeanNDVI: &mean, PlotCount: 1, AdapterMode: satellite.ModeLive, Quality: "complete",
	}

	cases := []struct {
		name      string
		change    satellite.ChangeView
		plotCount int
		err       error
		want      *float64
	}{
		{"one live plot is attributed", live, 1, nil, &mean},
		{"cloudy still available", satellite.ChangeView{MeanNDVI: &mean, PlotCount: 1, AdapterMode: satellite.ModeLive, Quality: "cloudy"}, 1, nil, &mean},
		{"two plots never attributed", live, 2, nil, nil},
		{"summary spans two plots", satellite.ChangeView{MeanNDVI: &mean, PlotCount: 2, AdapterMode: satellite.ModeLive, Quality: "complete"}, 1, nil, nil},
		{"summary spans no plots", satellite.ChangeView{MeanNDVI: &mean, PlotCount: 0, AdapterMode: satellite.ModeLive, Quality: "complete"}, 1, nil, nil},
		{"sandbox is never attributed", satellite.ChangeView{MeanNDVI: &mean, PlotCount: 1, AdapterMode: satellite.ModeSandbox, Quality: "synthetic"}, 1, nil, nil},
		{"unavailable is not attributed", satellite.ChangeView{MeanNDVI: &mean, PlotCount: 1, AdapterMode: satellite.ModeLive, Quality: "unavailable"}, 1, nil, nil},
		{"partial is not attributed", satellite.ChangeView{MeanNDVI: &mean, PlotCount: 1, AdapterMode: satellite.ModeLive, Quality: "partial"}, 1, nil, nil},
		{"synthetic quality is not attributed", satellite.ChangeView{MeanNDVI: &mean, PlotCount: 1, AdapterMode: satellite.ModeLive, Quality: "synthetic"}, 1, nil, nil},
		{"absent mean is not attributed", satellite.ChangeView{PlotCount: 1, AdapterMode: satellite.ModeLive, Quality: "complete"}, 1, nil, nil},
		{"missing summary is not attributed", live, 1, satellite.ErrSummaryMissing, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := singlePlotMarkerNDVI(tc.change, tc.plotCount, tc.err)
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("got %v, want no marker figure", *got)
			case tc.want != nil && got == nil:
				t.Fatalf("got nil, want marker figure %v", *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Fatalf("got %v, want %v", *got, *tc.want)
			}
		})
	}
}

// The returned figure is a copy, so a caller mutating the marker value cannot
// reach back into the shared project summary.
func TestSinglePlotMarkerNDVICopiesTheFigure(t *testing.T) {
	mean := 0.42
	change := satellite.ChangeView{MeanNDVI: &mean, PlotCount: 1, AdapterMode: satellite.ModeLive, Quality: "complete"}
	got := singlePlotMarkerNDVI(change, 1, nil)
	if got == nil {
		t.Fatal("expected a marker figure")
	}
	if got == change.MeanNDVI {
		t.Fatal("marker figure aliases the project summary pointer")
	}
	*got = 0.99
	if *change.MeanNDVI != 0.42 {
		t.Fatalf("mutating the marker changed the summary: %v", *change.MeanNDVI)
	}
}
