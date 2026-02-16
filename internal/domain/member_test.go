package domain

import "testing"

func TestRole_Level(t *testing.T) {
	tests := []struct {
		role Role
		want int
	}{
		{RoleSuperadmin, 100},
		{RoleTenantAdmin, 80},
		{RoleWorkspaceAdmin, 60},
		{RoleWorkspaceEditor, 40},
		{RoleWorkspaceViewer, 20},
		{Role("unknown"), 0},
		{Role(""), 0},
	}

	for _, tt := range tests {
		got := tt.role.Level()
		if got != tt.want {
			t.Errorf("Role(%q).Level() = %d, want %d", tt.role, got, tt.want)
		}
	}
}

func TestRole_Level_Ordering(t *testing.T) {
	roles := []Role{
		RoleWorkspaceViewer,
		RoleWorkspaceEditor,
		RoleWorkspaceAdmin,
		RoleTenantAdmin,
		RoleSuperadmin,
	}
	for i := 1; i < len(roles); i++ {
		if roles[i].Level() <= roles[i-1].Level() {
			t.Errorf("Expected %q.Level() > %q.Level(), got %d <= %d",
				roles[i], roles[i-1], roles[i].Level(), roles[i-1].Level())
		}
	}
}
