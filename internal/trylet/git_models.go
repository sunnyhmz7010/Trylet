package trylet

type GitReference struct {
	Name     string `json:"name"`
	FullName string `json:"fullName"`
	Hash     string `json:"hash"`
	Kind     string `json:"kind"`
}
type GitFileState struct {
	Path      string  `json:"path"`
	OldPath   *string `json:"oldPath"`
	XY        string  `json:"xy"`
	Untracked bool    `json:"untracked"`
	Conflict  bool    `json:"conflict"`
	Staged    bool    `json:"staged"`
	Unstaged  bool    `json:"unstaged"`
}
type GitWorktree struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Branch    string `json:"branch"`
	Current   bool   `json:"current"`
	Available bool   `json:"available"`
	Bare      bool   `json:"bare"`
	Detached  bool   `json:"detached"`
}
type GitStash struct {
	Reference string `json:"reference"`
	Subject   string `json:"subject"`
}
type GitSnapshot struct {
	Path            string         `json:"path"`
	Name            string         `json:"name"`
	Branch          *string        `json:"branch"`
	Head            *string        `json:"head"`
	Refs            []GitReference `json:"refs"`
	Files           []GitFileState `json:"files"`
	HistoryRevision string         `json:"historyRevision"`
	ChangesRevision string         `json:"changesRevision"`
	Upstream        *string        `json:"upstream"`
	Ahead           int            `json:"ahead"`
	Behind          int            `json:"behind"`
	Operation       *string        `json:"operation"`
	Worktrees       []GitWorktree  `json:"worktrees"`
	Stashes         []GitStash     `json:"stashes"`
}
type GitProject struct {
	RepoID   string      `json:"repoId"`
	Snapshot GitSnapshot `json:"snapshot"`
}
type GitRecent struct {
	Path string `json:"path"`
	Name string `json:"name"`
}
type GitCommit struct {
	Hash           string   `json:"hash"`
	Parents        []string `json:"parents"`
	Author         string   `json:"author"`
	AuthorEmail    string   `json:"authorEmail"`
	Committer      string   `json:"committer"`
	CommitterEmail string   `json:"committerEmail"`
	Date           string   `json:"date"`
	Subject        string   `json:"subject"`
}
type GitHistoryPage struct {
	Commits  []GitCommit `json:"commits"`
	Offset   int         `json:"offset"`
	HasMore  bool        `json:"hasMore"`
	Revision string      `json:"revision"`
}
type GitCommitFile struct {
	Path    string  `json:"path"`
	OldPath *string `json:"oldPath"`
	Status  string  `json:"status"`
}
type GitCommitDetail struct {
	GitCommit
	Files      []GitCommitFile `json:"files"`
	Comparison string          `json:"comparison"`
}
type GitPreviewSide struct {
	Size         int64   `json:"size"`
	Mime         string  `json:"mime"`
	Kind         string  `json:"kind"`
	DataURL      *string `json:"dataUrl"`
	Width        *int    `json:"width"`
	Height       *int    `json:"height"`
	Hex          string  `json:"hex"`
	HexTruncated bool    `json:"hexTruncated"`
	Note         *string `json:"note"`
}
type GitFilePreview struct {
	Before   *GitPreviewSide `json:"before"`
	After    *GitPreviewSide `json:"after"`
	Conflict bool            `json:"conflict"`
}
type GitConflictLine struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}
type GitConflictBlock struct {
	StartLine int               `json:"startLine"`
	EndLine   int               `json:"endLine"`
	Ours      []GitConflictLine `json:"ours"`
	Base      []GitConflictLine `json:"base"`
	Theirs    []GitConflictLine `json:"theirs"`
	Incoming  string            `json:"incoming"`
}
type GitConflict struct {
	Kind   string             `json:"kind"`
	Note   *string            `json:"note"`
	Blocks []GitConflictBlock `json:"blocks"`
}
type GitLFSPointer struct {
	OID  string `json:"oid"`
	Size int64  `json:"size"`
}
type GitLFSDiff struct {
	Conflict    bool           `json:"conflict"`
	Before      *GitLFSPointer `json:"before"`
	After       *GitLFSPointer `json:"after"`
	BeforeState string         `json:"beforeState"`
	AfterState  string         `json:"afterState"`
}
type GitDiff struct {
	Patch     string          `json:"patch"`
	Truncated bool            `json:"truncated"`
	Binary    bool            `json:"binary"`
	Preview   *GitFilePreview `json:"preview,omitempty"`
	Encoding  string          `json:"encoding,omitempty"`
	Note      *string         `json:"note"`
	NoteKey   string          `json:"noteKey,omitempty"`
	Conflict  *GitConflict    `json:"conflict"`
	LFS       *GitLFSDiff     `json:"lfs"`
}
type GitCommitView struct {
	Detail   GitCommitDetail `json:"detail"`
	FilePath *string         `json:"filePath"`
	Diff     *GitDiff        `json:"diff"`
}
type GitLineRequest struct {
	Path            string `json:"path"`
	Mode            string `json:"mode"`
	Commit          string `json:"commit"`
	Side            string `json:"side"`
	Line            int    `json:"line"`
	HistoryRevision string `json:"historyRevision"`
	ChangesRevision string `json:"changesRevision"`
}
type GitLineOrigin struct {
	Hash         *string `json:"hash"`
	Author       string  `json:"author"`
	Email        string  `json:"email"`
	Timestamp    *int64  `json:"timestamp"`
	Subject      string  `json:"subject"`
	OriginalLine int     `json:"originalLine"`
	Line         int     `json:"line"`
	Path         string  `json:"path"`
}
