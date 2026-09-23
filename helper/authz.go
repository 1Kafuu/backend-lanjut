package helper

import "sort"

type PermissionSet struct {
	byRole map[string]map[string]struct{}
}

func NewPermissionSet(raw map[string][]string) *PermissionSet {
	byRole := make(map[string]map[string]struct{}, len(raw))
	for role, perms := range raw {
		set := make(map[string]struct{}, len(perms))
		for _, p := range perms {
			set[p] = struct{}{}
		}
		byRole[role] = set
	}
	return &PermissionSet{byRole: byRole}
}

func (p *PermissionSet) Can(role, permission string) bool {
	if p == nil {
		return false
	}
	perms, ok := p.byRole[role]
	if !ok {
		return false
	}
	_, granted := perms[permission]
	return granted
}

func (p *PermissionSet) PermissionsOf(role string) []string {
	result := []string{}
	if p == nil {
		return result
	}
	for perm := range p.byRole[role] {
		result = append(result, perm)
	}
	sort.Strings(result)
	return result
}

func (p *PermissionSet) KnownRoles() []string {
	result := []string{}
	if p == nil {
		return result
	}
	for role := range p.byRole {
		result = append(result, role)
	}
	sort.Strings(result)
	return result
}

func (p *PermissionSet) IsKnownRole(role string) bool {
	if p == nil {
		return false
	}
	_, ok := p.byRole[role]
	return ok
}
