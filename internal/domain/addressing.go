package domain

import "strings"

// TemplateRef represents the deterministic addressing: tenantCode:workspaceCode:templateType
type TemplateRef struct {
	TenantCode    string
	WorkspaceCode string
	TemplateType  string
}

// ParseRef parses "latam:acme:welcome" into a TemplateRef.
func ParseRef(ref string) (*TemplateRef, error) {
	parts := strings.SplitN(ref, ":", 3)
	if len(parts) != 3 {
		return nil, ErrInvalidRef
	}
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, ErrInvalidRef
	}
	return &TemplateRef{
		TenantCode:    parts[0],
		WorkspaceCode: parts[1],
		TemplateType:  parts[2],
	}, nil
}
