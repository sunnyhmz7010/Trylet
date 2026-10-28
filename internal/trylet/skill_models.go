package trylet

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

type SkillLocation struct {
	ID       string `json:"id"`
	Scope    string `json:"scope"`
	Tool     string `json:"tool"`
	Path     string `json:"path"`
	Exists   bool   `json:"exists"`
	ReadOnly bool   `json:"readOnly"`
}

type SkillWorkspace struct {
	Preferences SkillPreferences `json:"preferences"`
	Locations   []SkillLocation  `json:"locations"`
}

type SkillEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type SkillInstallation struct {
	ID         string       `json:"id"`
	LocationID string       `json:"locationID"`
	Name       string       `json:"name"`
	Path       string       `json:"path"`
	Skills     []SkillEntry `json:"skills"`
	Repository string       `json:"repository"`
	SourcePath string       `json:"sourcePath"`
	Mode       string       `json:"mode"`
	Revision   string       `json:"revision"`
	ReadOnly   bool         `json:"readOnly"`
	Linked     bool         `json:"linked"`
	Missing    bool         `json:"missing"`
	Issue      string       `json:"issue"`
}

type SkillPreview struct {
	ID         string       `json:"id"`
	Repository string       `json:"repository"`
	Revision   string       `json:"revision"`
	Mode       string       `json:"mode"`
	Candidates []SkillEntry `json:"candidates"`
}

type SkillInstallResult struct {
	SourcePath string `json:"sourcePath"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Error      string `json:"error"`
	Warning    string `json:"warning"`
}

type SkillUpdateResult struct {
	NeedsConfirmation bool     `json:"needsConfirmation"`
	Confirmation      string   `json:"confirmation"`
	Changes           []string `json:"changes"`
	Message           string   `json:"message"`
	Warning           string   `json:"warning"`
}

type skillRecord struct {
	ID         string            `json:"id"`
	Root       string            `json:"root"`
	Name       string            `json:"name"`
	Repository string            `json:"repository"`
	SourcePath string            `json:"sourcePath"`
	Mode       string            `json:"mode"`
	Revision   string            `json:"revision"`
	Files      map[string]string `json:"files"`
}

type skillState struct {
	Preferences SkillPreferences       `json:"preferences"`
	Bindings    map[string]skillRecord `json:"bindings,omitempty"`
}
