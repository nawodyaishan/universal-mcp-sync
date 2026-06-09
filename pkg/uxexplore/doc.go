// Package uxexplore is the state-space explorer for the dashboard TUI.
//
// The package intentionally treats the dashboard as the production surface:
// fixtures create scanner/manager/profile inputs, drivers use Init/Update/View,
// and reports describe reachable user states. Keep dependencies limited to the
// dashboard domain packages so the explorer does not grow into a second app.
package uxexplore
