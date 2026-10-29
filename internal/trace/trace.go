// Package trace convierte instantáneas de CPU en una explicación por instrucción.
package trace

import (
	"fmt"
	"strings"

	"ensalucin/internal/qemulog"
	"ensalucin/internal/toolchain"
)

type RegChange struct {
	Reg  string `json:"reg"`
	From uint64 `json:"from"`
	To   uint64 `json:"to"`
}

type FlagChange struct {
	Flag string `json:"flag"`
	From bool   `json:"from"`
	To   bool   `json:"to"`
}

// Step describe una instrucción ejecutada y lo que hizo a la máquina.
type Step struct {
	N       int               `json:"n"`
	PC      uint64            `json:"pc"`
	Label   string            `json:"label,omitempty"`
	Bytes   string            `json:"bytes"`
	Text    string            `json:"text"`
	Regs    []RegChange       `json:"regs,omitempty"`
	Flags   []FlagChange      `json:"flags,omitempty"`
	Branch  string            `json:"branch,omitempty"` // tomado / no tomado / destino del salto
	Watch   map[string]uint64 `json:"watch,omitempty"`  // valores *después* del paso
	FlagsOn []string          `json:"flags_set"`        // flags activos después del paso
}

// Build empareja cada instantánea con la siguiente: la diferencia es el efecto
// de la instrucción situada en el PC de la primera.
func Build(arch qemulog.Arch, states []qemulog.State, lst *toolchain.Listing, rows int, watch []string) []Step {
	var steps []Step
	for i := 0; i < len(states) && i < rows; i++ {
		cur := states[i]
		in, _ := lst.At(cur.PC)
		st := Step{N: i + 1, PC: cur.PC, Label: lst.Labels[cur.PC], Bytes: in.Hex, Text: in.Text}
		if i+1 < len(states) {
			next := states[i+1]
			st.Regs, st.Flags = diff(arch, cur, next)
			st.Branch = branchNote(arch, in, next.PC, lst)
			st.FlagsOn = setFlags(arch, next)
			if len(watch) > 0 {
				st.Watch = map[string]uint64{}
				for _, w := range watch {
					st.Watch[w] = next.Regs[w]
				}
			}
		} else {
			st.FlagsOn = setFlags(arch, cur)
		}
		steps = append(steps, st)
	}
	return steps
}

func setFlags(arch qemulog.Arch, s qemulog.State) []string {
	on := []string{}
	for _, f := range qemulog.FlagOrder(arch) {
		if s.Flags[f] {
			on = append(on, f)
		}
	}
	return on
}

func diff(arch qemulog.Arch, a, b qemulog.State) ([]RegChange, []FlagChange) {
	var rc []RegChange
	for _, r := range qemulog.RegOrder(arch) {
		if a.Regs[r] != b.Regs[r] {
			rc = append(rc, RegChange{r, a.Regs[r], b.Regs[r]})
		}
	}
	var fc []FlagChange
	for _, f := range qemulog.FlagOrder(arch) {
		if a.Flags[f] != b.Flags[f] {
			fc = append(fc, FlagChange{f, a.Flags[f], b.Flags[f]})
		}
	}
	return rc, fc
}

func isConditional(arch qemulog.Arch, m string) bool {
	if arch == qemulog.X8664 {
		return strings.HasPrefix(m, "j") && m != "jmp" && m != "jmpq"
	}
	switch m {
	case "cbz", "cbnz", "tbz", "tbnz":
		return true
	}
	return strings.HasPrefix(m, "b.")
}

func branchNote(arch qemulog.Arch, in toolchain.Instr, nextPC uint64, lst *toolchain.Listing) string {
	if in.Text == "" {
		return ""
	}
	seq := in.Addr + uint64(len(in.Bytes))
	cond := isConditional(arch, in.Mnemonic())
	if nextPC == seq {
		if cond {
			return "no tomado, continúa en secuencia"
		}
		return ""
	}
	dst := fmt.Sprintf("0x%x", nextPC)
	if l, ok := lst.Labels[nextPC]; ok {
		dst = l
	}
	if cond {
		return "tomado -> " + dst
	}
	return "salta -> " + dst
}

// Val formatea un valor de registro: hexadecimal, más la lectura decimal cuando
// aporta información (complemento a dos para valores que parecen negativos).
func Val(v uint64) string {
	switch {
	case v < 10:
		return fmt.Sprintf("%d", v)
	case v >= 0xFFFFFFFFFFFF0000:
		return fmt.Sprintf("0x%x (%d)", v, int64(v))
	case v < 1<<32:
		return fmt.Sprintf("0x%x (%d)", v, v)
	}
	return fmt.Sprintf("0x%x", v)
}
