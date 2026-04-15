package postgres

import "github.com/rendis/senda/internal/port"

// Compile-time interface satisfaction checks.
var (
	_ port.TenantStore               = (*TenantRepo)(nil)
	_ port.WorkspaceStore            = (*WorkspaceRepo)(nil)
	_ port.GlobalConfigStore         = (*GlobalConfigRepo)(nil)
	_ port.InjectorStore             = (*InjectorRepo)(nil)
	_ port.AdapterStore              = (*AdapterRepo)(nil)
	_ port.AdapterGrantStore         = (*AdapterGrantRepo)(nil)
	_ port.AdapterIdentityGrantStore = (*AdapterIdentityGrantRepo)(nil)
	_ port.TemplateStore             = (*TemplateRepo)(nil)
	_ port.TemplateTypeUsageStore    = (*TemplateTypeUsageRepo)(nil)
	_ port.MemberStore               = (*MemberRepo)(nil)
	_ port.APIKeyStore               = (*APIKeyRepo)(nil)
	_ port.EmailStore                = (*EmailRepo)(nil)
	_ port.SuppressionStore          = (*SuppressionRepo)(nil)
	_ port.AuditLogStore             = (*AuditRepo)(nil)
	_ port.WebhookStore              = (*WebhookRepo)(nil)
	_ port.SNSReplayStore            = (*SNSReplayRepo)(nil)
	_ port.DashboardStore            = (*DashboardRepo)(nil)
)
