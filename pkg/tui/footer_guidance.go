package tui

type footerGuidanceReason string

const (
	footerMissingCredentials footerGuidanceReason = "missing-credentials"
	footerNoTargets          footerGuidanceReason = "no-targets-selected"
	footerPlanError          footerGuidanceReason = "plan-error"
	footerApplying           footerGuidanceReason = "applying"
	footerScanError          footerGuidanceReason = "scan-error"
	footerManagerMissing     footerGuidanceReason = "manager-missing"
)

type footerGuidanceKey struct {
	screen dashboardScreen
	reason footerGuidanceReason
}

var footerGuidance = map[footerGuidanceKey]string{
	{screenProviderReady, footerMissingCredentials}: "Add credentials to enable [Enter] - press [k]",
	{screenTargetSelect, footerMissingCredentials}:  "Credentials needed - press [k] to add",
	{screenTargetSelect, footerNoTargets}:           "Select at least one target to enable [Enter]",
	{screenTargetSelect, footerPlanError}:           "Plan failed - fix selection or press [Esc]",
	{screenPlanPreview, footerApplying}:             "Applying...",
	{screenDoctor, footerScanError}:                 "Scan failed - press [r] to retry or [w] for wizard",
	{screenDoctor, footerManagerMissing}:            "Manager unavailable - press [w] for wizard",
}

func guidanceFor(screen dashboardScreen, reason footerGuidanceReason) string {
	return footerGuidance[footerGuidanceKey{screen: screen, reason: reason}]
}
