// Package toolchain maneja llvm-mc, ld.lld, llvm-objdump/objcopy y qemu-user.
package toolchain

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ensalucin/internal/qemulog"
)

// find localiza una herramienta en PATH, con respaldo en los directorios
// versionados /usr/lib/llvm-N/bin de Debian.
func find(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	m, _ := filepath.Glob("/usr/lib/llvm-*/bin/" + name)
	if len(m) > 0 {
		return m[len(m)-1], nil
	}
	return "", fmt.Errorf("%s not found (ejecuta dentro de la imagen del contenedor)", name)
}

func tool(name string, args ...string) error {
	p, err := find(name)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd := exec.Command(p, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s falló:\n%s", name, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func triple(a qemulog.Arch) string { return string(a) + "-unknown-linux-gnu" }

// Assemble convierte texto ensamblador en un objeto ELF reubicable.
func Assemble(arch qemulog.Arch, src, obj string) error {
	return tool("llvm-mc", "-filetype=obj", "-triple="+triple(arch), "-o", obj, src)
}

// Link produce un ejecutable estático cuyo punto de entrada es entry.
func Link(obj, elf, entry string) error {
	return tool("ld.lld", "-static", "-e", entry, "-o", elf, obj)
}

// RawBinary extrae la sección .text como una imagen binaria plana.
func RawBinary(elf, bin string) error {
	return tool("llvm-objcopy", "-O", "binary", "--only-section=.text", elf, bin)
}

type Instr struct {
	Addr  uint64 `json:"addr"`
	Bytes []byte `json:"-"`
	Hex   string `json:"bytes"`
	Text  string `json:"text"`
}

func (i Instr) Mnemonic() string {
	f := strings.Fields(i.Text)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

type Listing struct {
	Instrs []Instr
	Labels map[uint64]string
	byAddr map[uint64]int
}

func (l *Listing) At(addr uint64) (Instr, bool) {
	i, ok := l.byAddr[addr]
	if !ok {
		return Instr{}, false
	}
	return l.Instrs[i], true
}

var (
	labelRE = regexp.MustCompile(`^([0-9a-f]+) <(.+)>:$`)
	instrRE = regexp.MustCompile(`^\s*([0-9a-f]+):\s+([0-9a-f ]*?)\s*\t(.+)$`)
	contRE  = regexp.MustCompile(`^\s*[0-9a-f]+:\s+([0-9a-f ]+)$`)
)

// Disassemble lista el código de elf tal como lo verá la CPU.
func Disassemble(arch qemulog.Arch, elf string, intel bool) (*Listing, error) {
	p, err := find("llvm-objdump")
	if err != nil {
		return nil, err
	}
	args := []string{"-d", "--print-imm-hex"}
	if intel && arch == qemulog.X8664 {
		args = append(args, "-M", "intel")
	}
	var out, stderr bytes.Buffer
	cmd := exec.Command(p, append(args, elf)...)
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("llvm-objdump falló: %s", strings.TrimSpace(stderr.String()))
	}
	l := &Listing{Labels: map[uint64]string{}, byAddr: map[uint64]int{}}
	for _, line := range strings.Split(out.String(), "\n") {
		if m := labelRE.FindStringSubmatch(line); m != nil {
			a, _ := strconv.ParseUint(m[1], 16, 64)
			l.Labels[a] = m[2]
		} else if m := instrRE.FindStringSubmatch(line); m != nil {
			a, _ := strconv.ParseUint(m[1], 16, 64)
			l.add(Instr{Addr: a, Text: strings.Join(strings.Fields(m[3]), " ")}, m[2])
		} else if m := contRE.FindStringSubmatch(line); m != nil && len(l.Instrs) > 0 {
			last := &l.Instrs[len(l.Instrs)-1] // las codificaciones x86 largas ocupan varias líneas
			last.Bytes = append(last.Bytes, parseHex(m[1])...)
			last.Hex += " " + strings.Join(strings.Fields(m[1]), " ")
		}
	}
	return l, nil
}

func (l *Listing) add(in Instr, hex string) {
	in.Bytes = parseHex(hex)
	in.Hex = strings.Join(strings.Fields(hex), " ")
	l.byAddr[in.Addr] = len(l.Instrs)
	l.Instrs = append(l.Instrs, in)
}

func parseHex(s string) []byte {
	var b []byte
	for _, f := range strings.Fields(s) {
		for i := 0; i+2 <= len(f); i += 2 {
			v, _ := strconv.ParseUint(f[i:i+2], 16, 8)
			b = append(b, byte(v))
		}
	}
	return b
}

type RunOpts struct {
	Timeout time.Duration
	Keep    int // instantáneas guardadas desde el inicio de la ejecución
	Limit   int // tope de instrucciones ejecutadas
	Stdin   io.Reader
}

type Result struct {
	Head     []qemulog.State // las primeras Keep+1 instantáneas
	Last     qemulog.State   // instantánea de la última instrucción ejecutada
	Total    int             // instrucciones ejecutadas
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Stopped  string // "", "timeout" o "step-limit"
}

// Run ejecuta elf bajo qemu-user, una instrucción por bloque de traducción, y
// transmite el log de CPU por una tubería para que un bucle infinito no llene el disco.
func Run(ctx context.Context, arch qemulog.Arch, elf string, o RunOpts) (*Result, error) {
	name := "qemu-" + string(arch)
	bin, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("%s not found (ejecuta dentro de la imagen del contenedor)", name)
	}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer pr.Close()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "-one-insn-per-tb", "-d", "cpu,nochain", "-D", "/dev/fd/3", elf)
	cmd.ExtraFiles = []*os.File{pw}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = o.Stdin, &stdout, &stderr
	if err := cmd.Start(); err != nil {
		pw.Close()
		return nil, err
	}
	pw.Close()

	res := &Result{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		qemulog.Stream(pr, arch, func(s qemulog.State) bool {
			res.Total++
			if len(res.Head) <= o.Keep {
				res.Head = append(res.Head, s)
			}
			res.Last = s
			if res.Total >= o.Limit {
				res.Stopped = "step-limit"
				cancel()
				return false
			}
			return true
		})
		io.Copy(io.Discard, pr)
	}()
	waitErr := cmd.Wait()
	<-done

	res.Stdout, res.Stderr = stdout.Bytes(), stderr.Bytes()
	res.ExitCode = cmd.ProcessState.ExitCode()
	if res.Stopped == "" && ctx.Err() == context.DeadlineExceeded {
		res.Stopped = "timeout"
	}
	if _, isExit := waitErr.(*exec.ExitError); waitErr != nil && !isExit {
		return nil, waitErr
	}
	return res, nil
}
