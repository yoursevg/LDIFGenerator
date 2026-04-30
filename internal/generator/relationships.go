package generator

import "math/rand"

type AttributeAssignment struct {
	Name   string
	Values []string
}

type Relationships struct {
	GroupMembers    map[string][]string
	UserGroups      map[string][]string
	ExtraAttributes map[string][]AttributeAssignment
}

func BuildPlan(cfg GeneratorConfig, rng *rand.Rand) []EntryType {
	plan := make([]EntryType, 0, cfg.Count)
	groups := percentCount(cfg.Count, cfg.Tree.GroupPercent)
	computers := percentCount(cfg.Count, cfg.Tree.ComputerPercent)
	services := percentCount(cfg.Count, cfg.Tree.ServicePercent)
	privileged := percentCount(cfg.Count, cfg.Tree.PrivilegedPercent)
	users := cfg.Count - groups - computers - services - privileged
	if users < 0 {
		users = 0
	}
	for i := 0; i < computers; i++ {
		plan = append(plan, EntryTypeComputer)
	}
	for i := 0; i < services; i++ {
		plan = append(plan, EntryTypeService)
	}
	for i := 0; i < privileged; i++ {
		plan = append(plan, EntryTypePrivileged)
	}
	for i := 0; i < users; i++ {
		plan = append(plan, EntryTypeUser)
	}
	shuffleEntryTypes(plan[:computers+services+privileged+users], rng)
	for i := 0; i < groups; i++ {
		plan = append(plan, EntryTypeGroup)
	}
	return plan
}

func shuffleEntryTypes(types []EntryType, rng *rand.Rand) {
	rng.Shuffle(len(types), func(i, j int) { types[i], types[j] = types[j], types[i] })
}

func BuildRelationships(cfg GeneratorConfig, plan []EntryType, rng *rand.Rand) Relationships {
	rel := Relationships{GroupMembers: map[string][]string{}, UserGroups: map[string][]string{}, ExtraAttributes: map[string][]AttributeAssignment{}}
	var users, managedUsers, groups []string
	for i, typ := range plan {
		ec := baseEntryContext(cfg, typ, i, rel, rng)
		if typ == EntryTypeGroup {
			groups = append(groups, ec.DN)
		}
		if typ == EntryTypeUser || typ == EntryTypePrivileged || typ == EntryTypeService || typ == EntryTypeComputer {
			users = append(users, ec.DN)
		}
		if typ == EntryTypeUser || typ == EntryTypePrivileged {
			managedUsers = append(managedUsers, ec.DN)
		}
	}
	if len(groups) == 0 || len(users) == 0 {
		return rel
	}
	maxMembers := cfg.Relationships.MaxMembersPerGroup
	if maxMembers <= 0 {
		maxMembers = 200
	}
	for _, userDN := range users {
		if rng.Intn(100) >= cfg.Relationships.UsersInGroupsPercent {
			continue
		}
		groupDN := groups[rng.Intn(len(groups))]
		if len(rel.GroupMembers[groupDN]) >= maxMembers {
			continue
		}
		rel.GroupMembers[groupDN] = append(rel.GroupMembers[groupDN], userDN)
		rel.UserGroups[userDN] = append(rel.UserGroups[userDN], groupDN)
	}
	for _, groupDN := range groups {
		if len(rel.GroupMembers[groupDN]) == 0 {
			userDN := users[rng.Intn(len(users))]
			rel.GroupMembers[groupDN] = append(rel.GroupMembers[groupDN], userDN)
			rel.UserGroups[userDN] = append(rel.UserGroups[userDN], groupDN)
		}
	}
	for _, groupDN := range groups {
		if len(groups) < 2 || rng.Intn(100) >= cfg.Relationships.NestedGroupsPercent {
			continue
		}
		parent := groups[rng.Intn(len(groups))]
		if parent == groupDN {
			continue
		}
		rel.GroupMembers[parent] = append(rel.GroupMembers[parent], groupDN)
	}
	for _, userDN := range managedUsers {
		if len(managedUsers) < 2 || rng.Intn(100) >= cfg.Relationships.ManagersPercent {
			continue
		}
		manager := managedUsers[rng.Intn(len(managedUsers))]
		if manager == userDN {
			continue
		}
		rel.ExtraAttributes[userDN] = append(rel.ExtraAttributes[userDN], AttributeAssignment{Name: "manager", Values: []string{manager}})
	}
	return rel
}

func percentCount(total, percent int) int {
	if percent <= 0 {
		return 0
	}
	if percent > 100 {
		percent = 100
	}
	return total * percent / 100
}
