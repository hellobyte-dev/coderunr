package e2e

// API Response Types

type PackageInfo struct {
	Language        string `json:"language"`
	LanguageVersion string `json:"language_version"`
	Installed       bool   `json:"installed"`
}

type Runtime struct {
	Language string   `json:"language"`
	Version  string   `json:"version"`
	Aliases  []string `json:"aliases"`
	Runtime  string   `json:"runtime"`
}

type ExecutionRequest struct {
	Language           string   `json:"language"`
	Version            string   `json:"version"`
	Files              []File   `json:"files"`
	Args               []string `json:"args,omitempty"`
	Stdin              string   `json:"stdin,omitempty"`
	CompileMemoryLimit *int64   `json:"compile_memory_limit,omitempty"`
	RunMemoryLimit     *int64   `json:"run_memory_limit,omitempty"`
	RunTimeout         *int     `json:"run_timeout,omitempty"`
	CompileTimeout     *int     `json:"compile_timeout,omitempty"`
	RunCPUTime         *int     `json:"run_cpu_time,omitempty"`
	CompileCPUTime     *int     `json:"compile_cpu_time,omitempty"`
}

type File struct {
	Name    string `json:"name,omitempty"`
	Content string `json:"content"`
}

type ExecutionResult struct {
	Run      RunResult `json:"run"`
	Language string    `json:"language"`
	Version  string    `json:"version"`
}

type RunResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Output   string `json:"output"`
	Code     int    `json:"code"`
	Memory   int64  `json:"memory"`
	CPUTime  int64  `json:"cpu_time"`
	WallTime int64  `json:"wall_time"`
}
