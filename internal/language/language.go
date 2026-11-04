package language

func Extension(language string) string {
	switch language {
	case "c":
		return ".c"
	case "cpp":
		return ".cpp"
	case "python":
		return ".py"
	}
	return ""
}
