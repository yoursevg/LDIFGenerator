package generator

import "time"

type EntryType string

const (
	EntryTypeUser       EntryType = "user"
	EntryTypePrivileged EntryType = "privilegedUser"
	EntryTypeGroup      EntryType = "group"
	EntryTypeComputer   EntryType = "computer"
	EntryTypeService    EntryType = "serviceAccount"
)

type TreeMode string

const (
	TreeModeFlat         TreeMode = "flat"
	TreeModeOU           TreeMode = "ou"
	TreeModeHierarchical TreeMode = "hierarchical"
)

type GeneratorConfig struct {
	BaseDN              string                 `json:"baseDN"`
	Count               int                    `json:"count"`
	Seed                int64                  `json:"seed"`
	BatchSize           int                    `json:"batchSize"`
	OutputPath          string                 `json:"outputPath"`
	StrictMode          bool                   `json:"strictMode"`
	OptionalFillPercent int                    `json:"optionalFillPercent"`
	SelectedAttributes  map[string]bool        `json:"selectedAttributes"`
	ObjectClasses       map[EntryType][]string `json:"objectClasses"`
	Tree                TreeConfig             `json:"tree"`
	Relationships       RelationshipConfig     `json:"relationships"`
}

type TreeConfig struct {
	Mode              TreeMode `json:"mode"`
	UserOU            string   `json:"userOU"`
	PrivilegedOU      string   `json:"privilegedOU"`
	GroupOU           string   `json:"groupOU"`
	ComputerOU        string   `json:"computerOU"`
	ServiceOU         string   `json:"serviceOU"`
	PrivilegedPercent int      `json:"privilegedPercent"`
	GroupPercent      int      `json:"groupPercent"`
	ComputerPercent   int      `json:"computerPercent"`
	ServicePercent    int      `json:"servicePercent"`
}

type RelationshipConfig struct {
	UsersInGroupsPercent int `json:"usersInGroupsPercent"`
	AllUsersGroupCount   int `json:"allUsersGroupCount"`
	NestedGroupsPercent  int `json:"nestedGroupsPercent"`
	ManagersPercent      int `json:"managersPercent"`
	MaxMembersPerGroup   int `json:"maxMembersPerGroup"`
}

type Report struct {
	Records       int           `json:"records"`
	StartedAt     time.Time     `json:"startedAt"`
	FinishedAt    time.Time     `json:"finishedAt"`
	Duration      time.Duration `json:"duration"`
	RecordsPerSec float64       `json:"recordsPerSec"`
	FileBytes     int64         `json:"fileBytes"`
	OutputPath    string        `json:"outputPath"`
	Warnings      []string      `json:"warnings,omitempty"`
}

func DefaultConfig() GeneratorConfig {
	return GeneratorConfig{
		BaseDN:              "dc=example,dc=com",
		Count:               100000,
		Seed:                42,
		BatchSize:           1000,
		OutputPath:          "generated.ldif",
		StrictMode:          true,
		OptionalFillPercent: 45,
		SelectedAttributes:  map[string]bool{},
		ObjectClasses: map[EntryType][]string{
			EntryTypeUser:       {"inetOrgPerson"},
			EntryTypePrivileged: {"privUser"},
			EntryTypeGroup:      {"groupOfNames"},
			EntryTypeComputer:   {"device"},
			EntryTypeService:    {"serviceUser"},
		},
		Tree: TreeConfig{
			Mode:              TreeModeHierarchical,
			UserOU:            "Users",
			PrivilegedOU:      "PrivilegedUsers",
			GroupOU:           "Groups",
			ComputerOU:        "Computers",
			ServiceOU:         "ServiceAccounts",
			PrivilegedPercent: 3,
			GroupPercent:      5,
			ComputerPercent:   5,
			ServicePercent:    2,
		},
		Relationships: RelationshipConfig{
			UsersInGroupsPercent: 70,
			AllUsersGroupCount:   0,
			NestedGroupsPercent:  10,
			ManagersPercent:      15,
			MaxMembersPerGroup:   200,
		},
	}
}
