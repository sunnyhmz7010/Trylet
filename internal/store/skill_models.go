package store

type SkillCustomLocation struct {
	Path    string `json:"path"`
	Project string `json:"project"`
}

type SkillPreferences struct {
	Scope           string                `json:"scope"`
	Project         string                `json:"project"`
	Location        string                `json:"location"`
	CustomLocations []SkillCustomLocation `json:"customLocations"`
}

type SkillRecord struct {
	ID         string            `json:"id"`
	Root       string            `json:"root"`
	Name       string            `json:"name"`
	Repository string            `json:"repository"`
	SourcePath string            `json:"sourcePath"`
	Mode       string            `json:"mode"`
	Revision   string            `json:"revision"`
	Files      map[string]string `json:"files"`
}

type SkillState struct {
	Preferences SkillPreferences       `json:"preferences"`
	Bindings    map[string]SkillRecord `json:"bindings,omitempty"`
}
