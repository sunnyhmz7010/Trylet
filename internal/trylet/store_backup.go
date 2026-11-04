package trylet

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func readStoreBackup(root string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(root, "Trylet.backup.json"))
	if err != nil {
		return nil, err
	}
	var state diskState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.Version != 1 || (state.Current != "" && state.Drafts[state.Current].ID != state.Current) {
		return nil, errors.New("备份的数据版本或当前草稿无效")
	}
	return data, nil
}

func CanRestoreStoreBackup(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "Trylet.json"))
	if err != nil {
		return false
	}
	var state diskState
	if json.Unmarshal(data, &state) == nil {
		return false
	}
	_, err = readStoreBackup(root)
	return err == nil
}

func RestoreStoreBackup(root string) (string, error) {
	backup, err := readStoreBackup(root)
	if err != nil {
		return "", fmt.Errorf("读取备份失败：%w", err)
	}
	path := filepath.Join(root, "Trylet.json")
	current, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var state diskState
	if json.Unmarshal(current, &state) == nil {
		return "", errors.New("当前数据文件仍可读取，未覆盖原文件")
	}
	preserved := filepath.Join(root, "Trylet.corrupt-"+newID()+".json")
	if err := atomicWrite(preserved, root, current); err != nil {
		return "", fmt.Errorf("保留损坏数据失败，未执行恢复：%w", err)
	}
	if err := atomicWrite(path, root, backup); err != nil {
		return preserved, fmt.Errorf("恢复失败，原文件另存于 %s：%w", preserved, err)
	}
	return preserved, nil
}
