package trylet

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func (s *Store) hasToken() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.state.Token) != 0
}

func (s *Store) saveToken(token string) error {
	var encrypted []byte
	if token != "" {
		plain := []byte(token)
		defer clear(plain)
		input := windows.DataBlob{Size: uint32(len(plain)), Data: &plain[0]}
		var output windows.DataBlob
		if err := windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
			return fmt.Errorf("加密访问令牌失败：%w", err)
		}
		defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
		encrypted = append([]byte(nil), unsafe.Slice(output.Data, output.Size)...)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.state.Token
	s.state.Token = encrypted
	if err := s.save(); err != nil {
		s.state.Token = previous
		return err
	}
	return nil
}

func (s *Store) readToken() (string, error) {
	s.mu.Lock()
	encrypted := append([]byte(nil), s.state.Token...)
	s.mu.Unlock()
	if len(encrypted) == 0 {
		return "", nil
	}
	input := windows.DataBlob{Size: uint32(len(encrypted)), Data: &encrypted[0]}
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return "", fmt.Errorf("当前 Windows 账户无法解密令牌，请在设置中重新填写：%w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	plain := unsafe.Slice(output.Data, output.Size)
	defer clear(plain)
	return string(plain), nil
}
