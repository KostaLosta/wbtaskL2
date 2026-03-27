package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type ioStreams struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type interruptManager struct {
	mu   sync.Mutex
	pids map[int]struct{}
}

func newInterruptManager() *interruptManager {
	return &interruptManager{pids: make(map[int]struct{})}
}

func (m *interruptManager) add(pid int) {
	m.mu.Lock()
	m.pids[pid] = struct{}{}
	m.mu.Unlock()
}

func (m *interruptManager) remove(pid int) {
	m.mu.Lock()
	delete(m.pids, pid)
	m.mu.Unlock()
}

func (m *interruptManager) killAll(sig syscall.Signal) {
	m.mu.Lock()
	pids := make([]int, 0, len(m.pids))
	for pid := range m.pids {
		pids = append(pids, pid)
	}
	m.mu.Unlock()

	for _, pid := range pids {
		// Process may already exit; ignore ESRCH.
		_ = syscall.Kill(pid, sig)
	}
}

type SimpleCmd struct {
	argv       []string
	stdinFile  string
	stdoutFile string
}

type Pipeline struct {
	cmds []SimpleCmd
}

type Conditional struct {
	pipelines []Pipeline
	ops       []string // "&&" or "||", length == len(pipelines)-1
}

type Shell struct {
	io     ioStreams
	inters *interruptManager
}

func main() {
	inters := newInterruptManager()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		for range sigCh {
			inters.killAll(syscall.SIGINT)
		}
	}()

	sh := &Shell{
		io: ioStreams{
			stdin:  os.Stdin,
			stdout: os.Stdout,
			stderr: os.Stderr,
		},
		inters: inters,
	}

	// If a file path is provided, treat it as a script.
	// Otherwise, run interactively until EOF (Ctrl+D).
	var r io.Reader = os.Stdin
	if len(os.Args) > 1 {
		f, err := os.Open(os.Args[1])
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "minishell: cannot open script: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		r = f
	}

	exitCode := sh.runAllLines(r)
	os.Exit(exitCode)
}

func (sh *Shell) runAllLines(r io.Reader) int {
	rd := bufio.NewReader(r)
	lastStatus := 0

	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				if strings.TrimSpace(line) != "" {
					lastStatus = sh.runLine(strings.TrimRight(line, "\r\n"))
				}
				return lastStatus
			}
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			_, _ = fmt.Fprintf(sh.io.stderr, "minishell: read error: %v\n", err)
			return 1
		}
		line = strings.TrimRight(line, "\r\n")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lastStatus = sh.runLine(line)
	}
}

func (sh *Shell) runLine(line string) int {
	toks, err := tokenize(line)
	if err != nil {
		_, _ = fmt.Fprintf(sh.io.stderr, "minishell: %v\n", err)
		return 2
	}
	if len(toks) == 0 {
		return 0
	}

	cond, err := parseConditional(toks)
	if err != nil {
		_, _ = fmt.Fprintf(sh.io.stderr, "minishell: syntax error: %v\n", err)
		return 2
	}

	status := 0
	for i := 0; i < len(cond.pipelines); i++ {
		if i > 0 {
			op := cond.ops[i-1]
			switch op {
			case "&&":
				if status != 0 {
					continue
				}
			case "||":
				if status == 0 {
					continue
				}
			default:
				return 2
			}
		}

		status = sh.runPipeline(cond.pipelines[i])
	}
	return status
}

func tokenize(line string) ([]string, error) {
	// Very small lexer: no quotes/escapes; operators must be separated or appear next to words.
	// Supports: |, &&, ||, <, >
	var out []string
	for i := 0; i < len(line); {
		switch line[i] {
		case ' ', '\t', '\n', '\r':
			i++
			continue
		case '&':
			if i+1 < len(line) && line[i+1] == '&' {
				out = append(out, "&&")
				i += 2
				continue
			}
			return nil, fmt.Errorf("unexpected '&' (did you mean '&&'?)")
		case '|':
			if i+1 < len(line) && line[i+1] == '|' {
				out = append(out, "||")
				i += 2
				continue
			}
			out = append(out, "|")
			i++
			continue
		case '<':
			out = append(out, "<")
			i++
			continue
		case '>':
			out = append(out, ">")
			i++
			continue
		default:
			j := i
			for j < len(line) {
				c := line[j]
				if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
					break
				}
				if c == '|' || c == '&' || c == '<' || c == '>' {
					break
				}
				j++
			}
			out = append(out, expandVars(line[i:j]))
			i = j
			continue
		}
	}
	return out, nil
}

func parseConditional(toks []string) (Conditional, error) {
	p := &condParser{toks: toks}
	return p.parse()
}

type condParser struct {
	toks []string
	pos  int
}

func (p *condParser) peek() (string, bool) {
	if p.pos >= len(p.toks) {
		return "", false
	}
	return p.toks[p.pos], true
}

func (p *condParser) next() (string, bool) {
	t, ok := p.peek()
	if !ok {
		return "", false
	}
	p.pos++
	return t, true
}

func (p *condParser) parse() (Conditional, error) {
	var pipelines []Pipeline
	var ops []string

	first, err := p.parsePipeline()
	if err != nil {
		return Conditional{}, err
	}
	pipelines = append(pipelines, first)

	for {
		t, ok := p.peek()
		if !ok {
			break
		}
		switch t {
		case "&&", "||":
			p.pos++
			nextPipe, err := p.parsePipeline()
			if err != nil {
				return Conditional{}, err
			}
			ops = append(ops, t)
			pipelines = append(pipelines, nextPipe)
		default:
			return Conditional{}, fmt.Errorf("unexpected token %q", t)
		}
	}

	return Conditional{pipelines: pipelines, ops: ops}, nil
}

func (p *condParser) parsePipeline() (Pipeline, error) {
	var cmds []SimpleCmd
	for {
		cmd, err := p.parseSimpleCmd()
		if err != nil {
			return Pipeline{}, err
		}
		cmds = append(cmds, cmd)

		t, ok := p.peek()
		if !ok || t != "|" {
			break
		}
		p.pos++
	}
	if len(cmds) == 0 {
		return Pipeline{}, fmt.Errorf("empty pipeline")
	}
	return Pipeline{cmds: cmds}, nil
}

func (p *condParser) parseSimpleCmd() (SimpleCmd, error) {
	var cmd SimpleCmd

	for {
		t, ok := p.peek()
		if !ok {
			break
		}

		switch t {
		case "|", "&&", "||":
			goto done
		case "<":
			p.pos++
			f, ok := p.next()
			if !ok || f == "" {
				return SimpleCmd{}, fmt.Errorf("missing file after '<'")
			}
			cmd.stdinFile = f
			continue
		case ">":
			p.pos++
			f, ok := p.next()
			if !ok || f == "" {
				return SimpleCmd{}, fmt.Errorf("missing file after '>'")
			}
			cmd.stdoutFile = f
			continue
		default:
			p.pos++
			if t != "" {
				cmd.argv = append(cmd.argv, t)
			}
			continue
		}
	}

done:
	if len(cmd.argv) == 0 {
		return SimpleCmd{}, fmt.Errorf("empty command")
	}
	return cmd, nil
}

func expandVars(s string) string {
	// Replace $VAR occurrences with environment value.
	// Undefined variables expand to empty string.
	if !strings.Contains(s, "$") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		if s[i] != '$' {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i + 1
		if j >= len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		if !isVarStart(s[j]) {
			b.WriteByte(s[i])
			i++
			continue
		}
		for j < len(s) && (isVarChar(s[j])) {
			j++
		}
		varName := s[i+1 : j]
		b.WriteString(os.Getenv(varName))
		i = j
	}
	return b.String()
}

func isVarStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

func isVarChar(b byte) bool {
	return isVarStart(b) || (b >= '0' && b <= '9')
}

func (sh *Shell) runPipeline(pl Pipeline) int {
	if len(pl.cmds) == 0 {
		return 0
	}
	if len(pl.cmds) == 1 {
		return sh.runSingleCmd(pl.cmds[0], true)
	}

	// Multiple stages: build pipes and run stages concurrently.
	n := len(pl.cmds)
	pipeR := make([]*os.File, n-1)
	pipeW := make([]*os.File, n-1)
	for i := 0; i < n-1; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			_, _ = fmt.Fprintf(sh.io.stderr, "minishell: pipe error: %v\n", err)
			// Close any previously opened pipes.
			for k := 0; k < i; k++ {
				_ = pipeR[k].Close()
				_ = pipeW[k].Close()
			}
			return 1
		}
		pipeR[i] = r
		pipeW[i] = w
	}

	// Close unused pipe ends if redirected stdin/stdout overrides pipeline connection.
	for i := 0; i < n-1; i++ {
		if pl.cmds[i].stdoutFile != "" {
			_ = pipeW[i].Close()
		}
		if pl.cmds[i+1].stdinFile != "" {
			_ = pipeR[i].Close()
		}
	}

	var (
		wg        sync.WaitGroup
		exitCodes = make([]int, n)
	)

	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()

			var inPipe *os.File
			if i > 0 && pl.cmds[i].stdinFile == "" {
				inPipe = pipeR[i-1]
			}
			var outPipe *os.File
			if i < n-1 && pl.cmds[i].stdoutFile == "" {
				outPipe = pipeW[i]
			}

			applyCD := false // avoid changing shell working directory concurrently
			code := sh.runStage(pl.cmds[i], inPipe, outPipe, i == 0, i == n-1, applyCD)
			exitCodes[i] = code
		}()
	}

	wg.Wait()
	return exitCodes[n-1]
}

func (sh *Shell) runSingleCmd(cmd SimpleCmd, applyCD bool) int {
	// Determine stdin.
	var stdin io.Reader = sh.io.stdin
	var stdinFile *os.File
	if cmd.stdinFile != "" {
		f, err := os.Open(cmd.stdinFile)
		if err != nil {
			_, _ = fmt.Fprintf(sh.io.stderr, "minishell: %v\n", err)
			return 1
		}
		stdinFile = f
		stdin = f
	}
	defer func() {
		if stdinFile != nil {
			_ = stdinFile.Close()
		}
	}()

	// Determine stdout.
	var stdout io.Writer = sh.io.stdout
	var stdoutFile *os.File
	if cmd.stdoutFile != "" {
		f, err := os.OpenFile(cmd.stdoutFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			_, _ = fmt.Fprintf(sh.io.stderr, "minishell: %v\n", err)
			return 1
		}
		stdoutFile = f
		stdout = f
	}
	defer func() {
		if stdoutFile != nil {
			_ = stdoutFile.Close()
		}
	}()

	return sh.runCmdWithIO(cmd, stdin, stdout, applyCD)
}

func (sh *Shell) runStage(cmd SimpleCmd, inPipe *os.File, outPipe *os.File, isFirst bool, isLast bool, applyCD bool) int {
	argv := cmd.argv
	if len(argv) == 0 {
		return 0
	}

	name := argv[0]
	isBuiltin := isBuiltinName(name)

	var (
		stdinReader io.Reader = sh.io.stdin
		stdinCloser io.Closer
	)
	if cmd.stdinFile != "" {
		f, err := os.Open(cmd.stdinFile)
		if err != nil {
			_, _ = fmt.Fprintf(sh.io.stderr, "minishell: %v\n", err)
			return 1
		}
		stdinReader = f
		stdinCloser = f
	} else if !isFirst {
		if inPipe == nil {
			// Connection already overridden; treat as empty input.
			stdinReader = strings.NewReader("")
		} else {
			stdinReader = inPipe
			stdinCloser = inPipe
		}
	}

	// stdout
	var (
		stdoutWriter io.Writer = sh.io.stdout
		stdoutCloser io.Closer
	)
	if cmd.stdoutFile != "" {
		f, err := os.OpenFile(cmd.stdoutFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			_, _ = fmt.Fprintf(sh.io.stderr, "minishell: %v\n", err)
			if stdinCloser != nil {
				_ = stdinCloser.Close()
			}
			return 1
		}
		stdoutWriter = f
		stdoutCloser = f
	} else if !isLast {
		if outPipe == nil {
			// Connection already overridden; discard output.
			stdoutWriter = io.Discard
		} else {
			stdoutWriter = outPipe
			stdoutCloser = outPipe
		}
	}

	defer func() {
		if stdinCloser != nil {
			_ = stdinCloser.Close()
		}
		if stdoutCloser != nil {
			_ = stdoutCloser.Close()
		}
	}()

	// Для встроенных команд в конвейере нужно правильно обработать stdin
	if isBuiltin {
		// Встроенные команды читают stdin, но если они не читают,
		// нужно прочитать весь stdin, чтобы не заблокировать конвейер
		if !isFirst && stdinCloser != nil {
			// Запускаем чтение в горутине, чтобы не блокироваться
			go func() {
				_, _ = io.Copy(io.Discard, stdinReader)
			}()
		}
		return sh.runBuiltin(name, argv[1:], stdoutWriter, sh.io.stderr, applyCD)
	}

	// External command.
	cmdExec := exec.Command(argv[0], argv[1:]...)
	cmdExec.Env = os.Environ()
	cmdExec.Stdin = stdinReader
	cmdExec.Stdout = stdoutWriter
	cmdExec.Stderr = sh.io.stderr

	// Start, register pid for Ctrl+C, wait.
	if err := cmdExec.Start(); err != nil {
		// Close pipes so downstream gets EOF.
		return sh.externalStartErrorCode(err)
	}
	pid := cmdExec.Process.Pid
	sh.inters.add(pid)
	err := cmdExec.Wait()
	sh.inters.remove(pid)

	if err == nil {
		return 0
	}
	return exitCodeFromError(err)
}

func (sh *Shell) runCmdWithIO(cmd SimpleCmd, stdin io.Reader, stdout io.Writer, applyCD bool) int {
	argv := cmd.argv
	if len(argv) == 0 {
		return 0
	}

	name := argv[0]
	if isBuiltinName(name) {
		if name == "cd" {
			return sh.builtinCD(argv[1:], applyCD, sh.io.stderr)
		}
		return sh.runBuiltin(name, argv[1:], stdout, sh.io.stderr, applyCD)
	}

	cmdExec := exec.Command(argv[0], argv[1:]...)
	cmdExec.Env = os.Environ()
	cmdExec.Stdin = stdin
	cmdExec.Stdout = stdout
	cmdExec.Stderr = sh.io.stderr

	if err := cmdExec.Start(); err != nil {
		return sh.externalStartErrorCode(err)
	}
	pid := cmdExec.Process.Pid
	sh.inters.add(pid)
	err := cmdExec.Wait()
	sh.inters.remove(pid)
	if err == nil {
		return 0
	}
	return exitCodeFromError(err)
}

func (sh *Shell) externalStartErrorCode(err error) int {
	var ee *exec.Error
	if errors.As(err, &ee) {
		if errors.Is(ee.Err, exec.ErrNotFound) {
			return 127
		}
	}
	return 1
}

func exitCodeFromError(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ee.ExitCode() >= 0 {
			return ee.ExitCode()
		}
		// Some platforms may require manual decoding.
		if st, ok := ee.ProcessState.Sys().(syscall.WaitStatus); ok {
			if st.Signaled() {
				return 128 + int(st.Signal())
			}
			if st.ExitStatus() != -1 {
				return st.ExitStatus()
			}
		}
		return 1
	}
	return 1
}

func isBuiltinName(s string) bool {
	switch s {
	case "cd", "pwd", "echo", "kill", "ps":
		return true
	default:
		return false
	}
}

func (sh *Shell) runBuiltin(name string, args []string, out io.Writer, errOut io.Writer, applyCD bool) int {
	switch name {
	case "cd":
		// applyCD controls whether we mutate working directory.
		return sh.builtinCD(args, applyCD, errOut)
	case "pwd":
		dir, err := os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintf(errOut, "minishell: %v\n", err)
			return 1
		}
		_, werr := fmt.Fprintln(out, dir)
		if werr != nil {
			_, _ = fmt.Fprintf(errOut, "minishell: %v\n", werr)
			return 1
		}
		return 0
	case "echo":
		s := strings.Join(args, " ")
		if _, err := fmt.Fprintln(out, s); err != nil {
			_, _ = fmt.Fprintf(errOut, "minishell: %v\n", err)
			return 1
		}
		return 0
	case "kill":
		return sh.builtinKill(args, errOut)
	case "ps":
		return builtinPs(args, out, errOut)
	default:
		_, _ = fmt.Fprintf(errOut, "minishell: unknown builtin %q\n", name)
		return 1
	}
}

func (sh *Shell) builtinCD(args []string, sideEffect bool, errOut io.Writer) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(errOut, "minishell: cd: missing path")
		return 1
	}
	path := args[0]
	// validate it exists and is a directory
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		_, _ = fmt.Fprintf(errOut, "minishell: cd: %s: %v\n", path, err)
		return 1
	}
	if !sideEffect {
		return 0
	}
	if err := os.Chdir(path); err != nil {
		_, _ = fmt.Fprintf(errOut, "minishell: cd: %s: %v\n", path, err)
		return 1
	}
	return 0
}

func (sh *Shell) builtinKill(args []string, errOut io.Writer) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(errOut, "minishell: kill: missing pid")
		return 1
	}
	pid, err := strconv.Atoi(args[0])
	if err != nil || pid <= 0 {
		_, _ = fmt.Fprintf(errOut, "minishell: kill: invalid pid: %q\n", args[0])
		return 1
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		_, _ = fmt.Fprintf(errOut, "minishell: kill: %v\n", err)
		return 1
	}
	return 0
}

func builtinPs(args []string, out io.Writer, errOut io.Writer) int {
	_ = args
	if runtime.GOOS == "linux" {
		return psFromProc(out, errOut)
	}
	return psFallback(out, errOut)
}

func psFromProc(out io.Writer, errOut io.Writer) int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "minishell: ps: %v\n", err)
		return 1
	}

	type procRow struct {
		pid  int
		comm string
	}
	rows := make([]procRow, 0, 256)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		pid, err := strconv.Atoi(name)
		if err != nil || pid <= 0 {
			continue
		}
		b, err := os.ReadFile("/proc/" + name + "/comm")
		if err != nil {
			continue
		}
		comm := strings.TrimSpace(string(b))
		rows = append(rows, procRow{pid: pid, comm: comm})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].pid < rows[j].pid })

	for _, r := range rows {
		if _, err := fmt.Fprintf(out, "%d\t%s\n", r.pid, r.comm); err != nil {
			_, _ = fmt.Fprintf(errOut, "minishell: ps: %v\n", err)
			return 1
		}
	}
	return 0
}

func psFallback(out io.Writer, errOut io.Writer) int {
	// Fallback for non-Linux: use system ps.
	cmd := exec.Command("ps", "-e", "-o", "pid=,comm=")
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintf(errOut, "minishell: ps: %v\n", err)
		return 1
	}
	return 0
}
