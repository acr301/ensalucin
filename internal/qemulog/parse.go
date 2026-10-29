// Package qemulog convierte los volcados de CPU que `qemu-<arch> -d cpu` escribe
// en su log en instantáneas de registros independientes de la arquitectura.
package qemulog

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type Arch string

const (
	X8664   Arch = "x86_64"
	AArch64 Arch = "aarch64"
)

// State es la CPU tal como QEMU la imprimió al inicio de un bloque de traducción.
// Con -one-insn-per-tb es el estado *antes* de ejecutar la instrucción en PC.
type State struct {
	PC    uint64
	Regs  map[string]uint64 // nombres en minúsculas, p. ej. rax, x0, sp
	Flags map[string]bool   // see FlagOrder
}

type spec struct {
	start string   // primer registro de cada volcado; marca un bloque nuevo
	regs  []string // orden de visualización
	flags []string
}

var specs = map[Arch]*spec{
	X8664: {
		start: "rax",
		regs: []string{"rax", "rbx", "rcx", "rdx", "rsi", "rdi", "rbp", "rsp",
			"r8", "r9", "r10", "r11", "r12", "r13", "r14", "r15", "rip", "rfl"},
		flags: []string{"ZF", "SF", "CF", "OF", "PF", "AF", "DF"},
	},
	AArch64: {
		start: "pc",
		regs:  aarch64Regs(),
		flags: []string{"N", "Z", "C", "V"},
	},
}

func aarch64Regs() []string {
	var r []string
	for i := 0; i <= 30; i++ {
		r = append(r, "x"+strconv.Itoa(i))
	}
	return append(r, "sp", "pc", "pstate")
}

// RegOrder lista los registros arquitectónicos en orden de visualización, sin el
// contador de programa ni los registros de flags.
func RegOrder(a Arch) []string {
	var out []string
	for _, r := range specs[a].regs {
		switch r {
		case "rip", "rfl", "pc", "pstate":
		default:
			out = append(out, r)
		}
	}
	return out
}

func FlagOrder(a Arch) []string { return specs[a].flags }

var tokenRE = regexp.MustCompile(`([A-Za-z][A-Za-z0-9]*)\s*=\s*([0-9a-fA-F]+)`)

func normalise(a Arch, name string) string {
	n := strings.ToLower(name)
	if a == AArch64 && len(n) > 1 && n[0] == 'x' {
		if v, err := strconv.Atoi(n[1:]); err == nil {
			return "x" + strconv.Itoa(v) // X00 -> x0
		}
	}
	return n
}

// Stream lee volcados de r y llama a fn una vez por bloque completo. Si fn
// devuelve false, la lectura se detiene.
func Stream(r io.Reader, arch Arch, fn func(State) bool) error {
	sp, ok := specs[arch]
	if !ok {
		return fmt.Errorf("unsupported architecture %q", arch)
	}
	want := map[string]bool{}
	for _, n := range sp.regs {
		want[n] = true
	}
	raw := map[string]uint64{}

	flush := func() bool {
		defer clear(raw)
		if len(raw) < len(sp.regs) { // bloque truncado (el proceso fue terminado)
			return true
		}
		return fn(finish(arch, raw))
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)
	for sc.Scan() {
		for _, m := range tokenRE.FindAllStringSubmatch(sc.Text(), -1) {
			name := normalise(arch, m[1])
			if !want[name] {
				continue
			}
			if name == sp.start && len(raw) > 0 && !flush() {
				return nil
			}
			v, err := strconv.ParseUint(m[2], 16, 64)
			if err != nil {
				continue
			}
			raw[name] = v
		}
	}
	if len(raw) > 0 {
		flush()
	}
	return sc.Err()
}

func finish(arch Arch, raw map[string]uint64) State {
	s := State{Regs: map[string]uint64{}, Flags: map[string]bool{}}
	for k, v := range raw {
		s.Regs[k] = v
	}
	bit := func(v uint64, n uint) bool { return v>>n&1 == 1 }
	switch arch {
	case X8664:
		s.PC = raw["rip"]
		f := raw["rfl"]
		s.Flags = map[string]bool{"CF": bit(f, 0), "PF": bit(f, 2), "AF": bit(f, 4),
			"ZF": bit(f, 6), "SF": bit(f, 7), "DF": bit(f, 10), "OF": bit(f, 11)}
	case AArch64:
		s.PC = raw["pc"]
		p := raw["pstate"]
		s.Flags = map[string]bool{"N": bit(p, 31), "Z": bit(p, 30), "C": bit(p, 29), "V": bit(p, 28)}
	}
	return s
}
