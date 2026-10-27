package trylet

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const gitMediaLimit = 8 * 1024 * 1024

type gitSide struct {
	bytes     []byte
	size      int64
	exists    bool
	truncated bool
}

func (g *GitReader) blob(root, revision, path string) (gitSide, error) {
	var result gitSide
	if revision == "" {
		return result, nil
	}
	spec := revision + ":" + path
	if strings.HasPrefix(revision, ":") {
		spec = revision + path
	}
	oid, _, err := g.run(root, []string{"rev-parse", "--verify", "--end-of-options", spec}, nil, 32000, 128)
	if err != nil {
		return result, err
	}
	hash := strings.TrimSpace(string(oid))
	if !validCommit(hash) {
		return result, nil
	}
	size, err := g.text(root, "cat-file", "-s", hash)
	if err != nil {
		return result, err
	}
	result.size, err = strconv.ParseInt(size, 10, 64)
	if err != nil {
		return result, err
	}
	result.exists = true
	if result.size > gitMediaLimit {
		result.truncated = true
		return result, nil
	}
	b, large, err := g.run(root, []string{"cat-file", "blob", hash}, nil, gitMediaLimit)
	result.bytes = b
	result.truncated = large
	return result, err
}
func worktreeSide(root, path string) (gitSide, error) {
	var result gitSide
	if _, err := safeGitFile(root, path); err != nil {
		return result, err
	}
	boundary, err := os.OpenRoot(root)
	if err != nil {
		return result, err
	}
	defer boundary.Close()
	f, err := boundary.Open(filepath.FromSlash(path))
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() {
		return result, gitError("unsupported", "该路径不是可预览的普通文件。")
	}
	result.exists = true
	result.size = info.Size()
	if result.size > gitMediaLimit {
		result.truncated = true
		b, err := io.ReadAll(io.LimitReader(f, 4096))
		result.bytes = b
		return result, err
	}
	result.bytes, err = io.ReadAll(io.LimitReader(f, gitMediaLimit+1))
	if len(result.bytes) > gitMediaLimit {
		result.bytes = result.bytes[:gitMediaLimit]
		result.truncated = true
	}
	return result, err
}
func (g *GitReader) commitDetail(id, hash, expected string) (GitCommitDetail, error) {
	var result GitCommitDetail
	if !validCommit(hash) {
		return result, gitError("invalid", "提交 ID 无效。")
	}
	r, err := g.repo(id)
	if err != nil {
		return result, err
	}
	if expected != "" {
		refs, raw, err := g.refs(r.root)
		_ = refs
		if err != nil {
			return result, err
		}
		head, _, err := g.run(r.root, []string{"rev-parse", "--verify", "HEAD"}, nil, 32000, 128)
		if err != nil {
			return result, err
		}
		sum := sha256.New()
		_, _ = sum.Write(head)
		_, _ = sum.Write(raw)
		if hex.EncodeToString(sum.Sum(nil)) != expected {
			return result, gitError("changing", "仓库历史已变化，请刷新后重试。")
		}
	}
	b, large, err := g.run(r.root, []string{"show", "--no-patch", "--format=" + gitCommitFormat, "--end-of-options", hash}, nil, gitReadLimit)
	if err != nil {
		return result, err
	}
	commits := parseGitCommits(b)
	if large || len(commits) != 1 {
		return result, gitError("invalid", "无法读取该提交。")
	}
	result.GitCommit = commits[0]
	result.Files = []GitCommitFile{}
	args := []string{"diff-tree", "--root", "--no-commit-id", "--name-status", "-r", "-M", "-z", hash, "--"}
	if len(result.Parents) > 0 {
		result.Comparison = result.Parents[0]
		args = []string{"diff", "--name-status", "-M", "-z", "--no-ext-diff", "--no-textconv", result.Comparison, hash, "--"}
	}
	b, large, err = g.run(r.root, args, nil, gitReadLimit)
	if err != nil {
		return result, err
	}
	if large {
		return result, gitError("tooLarge", "提交文件列表超过读取上限。")
	}
	parts := bytes.Split(b, []byte{0})
	for i := 0; i+1 < len(parts); {
		status, path := string(parts[i]), string(parts[i+1])
		i += 2
		if status == "" {
			continue
		}
		file := GitCommitFile{Path: path, Status: status}
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			if i >= len(parts) {
				break
			}
			file.OldPath = &path
			file.Path = string(parts[i])
			i++
		}
		result.Files = append(result.Files, file)
	}
	return result, nil
}
func (g *GitReader) GetCommitView(id, hash, path, expected string) (GitCommitView, error) {
	detail, err := g.commitDetail(id, hash, expected)
	result := GitCommitView{Detail: detail}
	if err != nil {
		return result, err
	}
	if path == "" && len(detail.Files) > 0 {
		path = detail.Files[0].Path
	}
	if path == "" {
		return result, nil
	}
	found := false
	for _, file := range detail.Files {
		if file.Path == path {
			found = true
		}
	}
	if !found {
		return result, gitError("invalid", "提交中没有该文件。")
	}
	diff, err := g.diffFile(id, "commit", path, hash, &detail)
	if err != nil {
		return result, err
	}
	result.FilePath = &path
	result.Diff = &diff
	return result, nil
}
func (g *GitReader) GetDiff(id, mode, path, commit, expectedHistory, expectedChanges string) (GitDiff, error) {
	s, err := g.GetSnapshot(id)
	if err != nil {
		return GitDiff{}, err
	}
	if (expectedHistory != "" && expectedHistory != s.HistoryRevision) || (expectedChanges != "" && expectedChanges != s.ChangesRevision) {
		return GitDiff{}, gitError("changing", "文件状态已变化，请等待刷新或重试。")
	}
	return g.diffFile(id, mode, path, commit, nil)
}
func (g *GitReader) sides(id, mode, path, commit string, detail *GitCommitDetail) (gitSide, gitSide, error) {
	var before, after gitSide
	r, err := g.repo(id)
	if err != nil {
		return before, after, err
	}
	if _, err = safeGitFile(r.root, path); err != nil {
		return before, after, err
	}
	switch mode {
	case "commit":
		if detail == nil {
			value, e := g.commitDetail(id, commit, "")
			if e != nil {
				return before, after, e
			}
			detail = &value
		}
		old := path
		found := false
		for _, file := range detail.Files {
			if file.Path == path {
				found = true
				if file.OldPath != nil {
					old = *file.OldPath
				}
			}
		}
		if !found {
			return before, after, gitError("invalid", "提交中不存在该文件。")
		}
		before, err = g.blob(r.root, detail.Comparison, old)
		if err == nil {
			after, err = g.blob(r.root, commit, path)
		}
	case "staged":
		snapshot, e := g.GetSnapshot(id)
		if e != nil {
			return before, after, e
		}
		old := path
		found := false
		for _, file := range snapshot.Files {
			if file.Path == path && file.Staged {
				found = true
				if file.OldPath != nil {
					old = *file.OldPath
				}
			}
		}
		if !found {
			return before, after, gitError("invalid", "文件不在已暂存更改中。")
		}
		if snapshot.Head != nil {
			before, err = g.blob(r.root, *snapshot.Head, old)
		}
		if err == nil {
			after, err = g.blob(r.root, ":0:", path)
		}
	case "unstaged", "conflict":
		snapshot, e := g.GetSnapshot(id)
		if e != nil {
			return before, after, e
		}
		old := path
		found := false
		for _, file := range snapshot.Files {
			if file.Path == path && (file.Unstaged || file.Conflict || file.Untracked) {
				found = true
				if file.OldPath != nil && len(file.XY) == 2 && strings.ContainsAny(file.XY[1:], "RC") {
					old = *file.OldPath
				}
			}
		}
		if !found {
			return before, after, gitError("invalid", "文件不在工作区更改中。")
		}
		stage := ":0:"
		if mode == "conflict" {
			stage = ":2:"
		}
		before, err = g.blob(r.root, stage, old)
		if err == nil {
			after, err = worktreeSide(r.root, path)
		}
	default:
		return before, after, gitError("invalid", "差异范围无效。")
	}
	return before, after, err
}
func decodeGitText(b []byte) (string, string, bool) {
	if len(b) >= 2 && ((b[0] == 255 && b[1] == 254) || (b[0] == 254 && b[1] == 255)) {
		var order binary.ByteOrder = binary.LittleEndian
		encoding := "UTF-16 LE"
		if b[0] == 254 {
			order = binary.BigEndian
			encoding = "UTF-16 BE"
		}
		if len(b)%2 != 0 {
			return "", "", false
		}
		words := make([]uint16, 0, (len(b)-2)/2)
		for i := 2; i+1 < len(b); i += 2 {
			words = append(words, order.Uint16(b[i:i+2]))
		}
		return string(utf16.Decode(words)), encoding, true
	}
	if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		return "", "", false
	}
	return string(b), "UTF-8", true
}
func lfsPointer(b []byte) *GitLFSPointer {
	if len(b) > 2048 || !bytes.HasPrefix(b, []byte("version https://git-lfs.github.com/spec/v1\n")) {
		return nil
	}
	var pointer GitLFSPointer
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if strings.HasPrefix(line, "oid sha256:") {
			pointer.OID = strings.TrimPrefix(line, "oid sha256:")
		}
		if strings.HasPrefix(line, "size ") {
			pointer.Size, _ = strconv.ParseInt(strings.TrimPrefix(line, "size "), 10, 64)
		}
	}
	hash, err := hex.DecodeString(pointer.OID)
	if err != nil || len(hash) != 32 || pointer.Size < 0 {
		return nil
	}
	return &pointer
}
func GitLFSClean(input io.Reader, output io.Writer) error {
	prefix := make([]byte, 2049)
	n, err := io.ReadFull(input, prefix)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return err
	}
	prefix = prefix[:n]
	if n <= 2048 && lfsPointer(prefix) != nil {
		_, err = output.Write(prefix)
		return err
	}
	hash := sha256.New()
	_, _ = hash.Write(prefix)
	rest, err := io.Copy(hash, input)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "version https://git-lfs.github.com/spec/v1\noid sha256:%x\nsize %d\n", hash.Sum(nil), int64(n)+rest)
	return err
}
func previewGitSide(side gitSide, path string) *GitPreviewSide {
	if !side.exists {
		return nil
	}
	kind := "binary"
	mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	result := &GitPreviewSide{Size: side.size, Mime: mediaType, Kind: kind, Hex: hex.Dump(side.bytes[:min(len(side.bytes), 4096)]), HexTruncated: side.size > 4096}
	if side.truncated {
		note := "文件超过 8 MiB，只展示元信息与可用字节预览。"
		result.Note = &note
		return result
	}
	if strings.HasPrefix(mediaType, "image/") {
		if config, _, err := image.DecodeConfig(bytes.NewReader(side.bytes)); err == nil && config.Width > 0 && config.Height > 0 && int64(config.Width)*int64(config.Height) <= 16000000 {
			result.Width = &config.Width
			result.Height = &config.Height
			result.Kind = "image"
		} else {
			note := "图片格式或尺寸超过安全预览范围，显示字节预览。"
			result.Note = &note
		}
	} else if strings.HasPrefix(mediaType, "audio/") {
		result.Kind = "audio"
	} else if strings.HasPrefix(mediaType, "video/") {
		result.Kind = "video"
	}
	if result.Kind != "binary" {
		value := "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(side.bytes)
		result.DataURL = &value
	}
	return result
}
func (g *GitReader) diffFile(id, mode, path, commit string, detail *GitCommitDetail) (GitDiff, error) {
	var result GitDiff
	before, after, err := g.sides(id, mode, path, commit, detail)
	if err != nil {
		return result, err
	}
	lfsBefore, lfsAfter := lfsPointer(before.bytes), lfsPointer(after.bytes)
	if lfsBefore != nil || lfsAfter != nil {
		state := func(side gitSide, pointer *GitLFSPointer) string {
			if !side.exists {
				return "missing"
			}
			if pointer != nil {
				return "pointer"
			}
			return "regular"
		}
		result.LFS = &GitLFSDiff{Conflict: mode == "conflict", Before: lfsBefore, After: lfsAfter, BeforeState: state(before, lfsBefore), AfterState: state(after, lfsAfter)}
		return result, nil
	}
	left, leftEncoding, leftText := decodeGitText(before.bytes)
	right, rightEncoding, rightText := decodeGitText(after.bytes)
	mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if before.truncated || after.truncated || !leftText || !rightText || strings.HasPrefix(mediaType, "image/") || strings.HasPrefix(mediaType, "audio/") || strings.HasPrefix(mediaType, "video/") {
		result.Binary = true
		result.Truncated = before.truncated || after.truncated
		result.Preview = &GitFilePreview{Before: previewGitSide(before, path), After: previewGitSide(after, path), Conflict: mode == "conflict"}
		return result, nil
	}
	result.Encoding = leftEncoding
	if leftEncoding != rightEncoding {
		result.Encoding = leftEncoding + " → " + rightEncoding
	}
	if leftEncoding != "UTF-8" || rightEncoding != "UTF-8" {
		note := "文本已从 UTF-16 解码后比较；行归属仅支持原始 UTF-8 内容。"
		result.Note = &note
	}
	if mode == "conflict" {
		result.Conflict = parseGitConflict(right)
	}
	if err = os.MkdirAll(g.tempRoot, 0700); err != nil {
		return result, err
	}
	dir, err := os.MkdirTemp(g.tempRoot, "git-diff-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(dir)
	a, b := filepath.Join(dir, "before"), filepath.Join(dir, "after")
	if err = os.WriteFile(a, []byte(left), 0600); err != nil {
		return result, err
	}
	if err = os.WriteFile(b, []byte(right), 0600); err != nil {
		return result, err
	}
	patch, truncated, err := g.run(g.tempRoot, []string{"diff", "--no-index", "--no-color", "--no-ext-diff", "--no-textconv", "--src-prefix=a/", "--dst-prefix=b/", "--", a, b}, nil, 240*1024, 1)
	if err != nil {
		return result, err
	}
	text := string(patch)
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "")
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			lines[i] = "diff --git a/" + path + " b/" + path
		}
		if strings.HasPrefix(line, "--- ") {
			lines[i] = "--- a/" + path
		}
		if strings.HasPrefix(line, "+++ ") {
			lines[i] = "+++ b/" + path
		}
	}
	result.Patch = strings.Join(lines, "\n")
	result.Truncated = truncated
	if truncated {
		note := "文本差异超过 240 KiB，已截断显示。"
		result.Note = &note
	}
	return result, nil
}
func parseGitConflict(text string) *GitConflict {
	result := &GitConflict{Kind: "text", Blocks: []GitConflictBlock{}}
	var block *GitConflictBlock
	section := ""
	for i, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "<<<<<<< "):
			block = &GitConflictBlock{StartLine: i + 1, Ours: []GitConflictLine{}, Base: []GitConflictLine{}, Theirs: []GitConflictLine{}}
			section = "ours"
		case block != nil && strings.HasPrefix(line, "||||||| "):
			section = "base"
		case block != nil && line == "=======":
			section = "theirs"
		case block != nil && strings.HasPrefix(line, ">>>>>>> "):
			block.EndLine = i + 1
			block.Incoming = strings.TrimPrefix(line, ">>>>>>> ")
			result.Blocks = append(result.Blocks, *block)
			block = nil
		case block != nil:
			entry := GitConflictLine{i + 1, line}
			switch section {
			case "ours":
				block.Ours = append(block.Ours, entry)
			case "base":
				block.Base = append(block.Base, entry)
			case "theirs":
				block.Theirs = append(block.Theirs, entry)
			}
		}
	}
	if len(result.Blocks) == 0 {
		note := "Git 仍记录此文件存在冲突，工作区中没有完整的文本冲突标记。"
		result.Note = &note
	}
	return result
}
func (g *GitReader) GetLineOrigin(id string, request GitLineRequest) (GitLineOrigin, error) {
	result := GitLineOrigin{Line: request.Line, OriginalLine: request.Line, Path: request.Path}
	if request.Line < 1 || request.Line > 10000000 || (request.Side != "before" && request.Side != "after") {
		return result, gitError("invalid", "行号或比较侧无效。")
	}
	r, err := g.repo(id)
	if err != nil {
		return result, err
	}
	if _, err = safeGitFile(r.root, request.Path); err != nil {
		return result, err
	}
	var snapshot GitSnapshot
	var file GitFileState
	if request.Mode != "commit" {
		snapshot, err = g.GetSnapshot(id)
		if err != nil {
			return result, err
		}
		if (request.ChangesRevision != "" && request.ChangesRevision != snapshot.ChangesRevision) || (request.HistoryRevision != "" && request.HistoryRevision != snapshot.HistoryRevision) {
			return result, gitError("changing", "文件或暂存区已变化，请刷新差异后查看行归属。")
		}
		for _, candidate := range snapshot.Files {
			if candidate.Path == request.Path {
				file = candidate
				break
			}
		}
		if file.Conflict {
			return result, gitError("conflict", "冲突解决前无法确定行归属。")
		}
	}
	before, after, err := g.sides(id, request.Mode, request.Path, request.Commit, nil)
	if err != nil {
		return result, err
	}
	chosen := after
	if request.Side == "before" {
		chosen = before
	}
	_, encoding, text := decodeGitText(chosen.bytes)
	if !chosen.exists || chosen.truncated || !text || encoding != "UTF-8" {
		return result, gitError("unsupported", "此编码或二进制文件不提供行归属。")
	}
	revision := "HEAD"
	path := request.Path
	useContents := request.Mode != "commit" && !(request.Mode == "staged" && request.Side == "before")
	if request.Mode == "commit" {
		detail, e := g.commitDetail(id, request.Commit, request.HistoryRevision)
		if e != nil {
			return result, e
		}
		revision = request.Commit
		if request.Side == "before" {
			revision = detail.Comparison
			for _, file := range detail.Files {
				if file.Path == path && file.OldPath != nil {
					path = *file.OldPath
				}
			}
		}
	} else if file.OldPath != nil {
		path = *file.OldPath
	}
	uncommitted := snapshot.Head == nil && request.Mode != "commit"
	if useContents && !uncommitted {
		ancestor, e := g.blob(r.root, "HEAD", path)
		if e != nil {
			return result, e
		}
		uncommitted = !ancestor.exists
	}
	if revision == "" || uncommitted {
		result.Author = "尚未提交"
		result.Subject = "尚未提交的内容"
		return result, nil
	}
	args := []string{"blame", "--line-porcelain", "--no-textconv", "-L", fmt.Sprintf("%d,%d", request.Line, request.Line)}
	var input []byte
	if useContents {
		args = append(args, "--contents", "-")
		input = chosen.bytes
	} else {
		args = append(args, revision)
	}
	args = append(args, "--", path)
	b, large, err := g.run(r.root, args, input, 1024*1024)
	if err != nil {
		return result, err
	}
	if large {
		return result, gitError("tooLarge", "此行过长，无法完整读取作者信息。")
	}
	for i, line := range strings.Split(string(b), "\n") {
		if i == 0 {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				hash := strings.TrimPrefix(fields[0], "^")
				if strings.Trim(hash, "0") != "" {
					result.Hash = &hash
				}
				result.OriginalLine, _ = strconv.Atoi(fields[1])
			}
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "author":
			result.Author = value
		case "author-mail":
			result.Email = strings.Trim(value, "<>")
		case "author-time":
			timestamp, _ := strconv.ParseInt(value, 10, 64)
			result.Timestamp = &timestamp
		case "summary":
			result.Subject = value
		}
	}
	if result.Hash == nil {
		result.Author = "尚未提交"
		result.Subject = "此行尚未提交"
		result.Email = ""
		result.Timestamp = nil
	}
	return result, nil
}
