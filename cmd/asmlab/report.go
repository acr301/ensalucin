package main

import (
	"fmt"
	"os"
	"strings"

	"ensalucin/internal/qemulog"
	"ensalucin/internal/toolchain"
	"ensalucin/internal/trace"
)

type report struct {
	Source  string
	Arch    string
	Intel   bool
	Listing *toolchain.Listing
	Result  *toolchain.Result
	Steps   []trace.Step
	Watch   []string
	Files   []string
	opts    options
}

type style struct{ on bool }

func (s style) c(code, t string) string {
	if !s.on {
		return t
	}
	return "\x1b[" + code + "m" + t + "\x1b[0m"
}

func (r *report) style() style {
	switch r.opts.color {
	case "always":
		return style{true}
	case "never":
		return style{false}
	}
	st, err := os.Stdout.Stat()
	return style{err == nil && st.Mode()&os.ModeCharDevice != 0 && os.Getenv("NO_COLOR") == ""}
}

func (r *report) jsonView() any {
	res := r.Result
	return map[string]any{
		"source": r.Source, "arch": r.Arch,
		"listing":   r.Listing.Instrs,
		"steps":     r.Steps,
		"total":     res.Total,
		"truncated": res.Total > len(r.Steps),
		"stdout":    string(res.Stdout),
		"stderr":    string(res.Stderr),
		"exit_code": res.ExitCode,
		"stopped":   res.Stopped,
		"final":     r.finalView(),
	}
}

func (r *report) finalView() map[string]any {
	last := r.Result.Last
	on := []string{}
	for _, f := range qemulog.FlagOrder(qemulog.Arch(r.Arch)) {
		if last.Flags[f] {
			on = append(on, f)
		}
	}
	return map[string]any{"pc": last.PC, "regs": last.Regs, "flags_set": on}
}

func (r *report) print() error {
	s := r.style()
	h := func(t string) { fmt.Printf("\n%s\n", s.c("1;36", "== "+t+" ==")) }

	syntax := "AT&T syntax"
	if r.Intel {
		syntax = "Intel syntax"
	}
	if r.Arch == "aarch64" {
		syntax = "A64"
	}
	fmt.Printf("%s  (%s, %s)\n", s.c("1", r.Source), r.Arch, syntax)

	if r.opts.listing {
		h("Listado (dirección, codificación, instrucción)")
		for _, in := range r.Listing.Instrs {
			if l, ok := r.Listing.Labels[in.Addr]; ok {
				fmt.Printf("%s\n", s.c("33", l+":"))
			}
			fmt.Printf("  0x%06x  %-24s  %s\n", in.Addr, in.Hex, in.Text)
		}
	}
	for _, f := range r.Files {
		fmt.Printf("escrito %s\n", f)
	}
	if r.Result == nil {
		return nil
	}
	res := r.Result

	if len(res.Stdout) > 0 || len(res.Stderr) > 0 {
		h("Salida del programa")
		os.Stdout.Write(res.Stdout)
		if len(res.Stdout) > 0 && res.Stdout[len(res.Stdout)-1] != '\n' {
			fmt.Println()
		}
		os.Stdout.Write(res.Stderr)
	}

	if !r.opts.noTrace {
		h(fmt.Sprintf("Traza: %d instrucciones ejecutadas", res.Total))
		fmt.Println(s.c("2", "se muestra lo que CAMBIÓ cada instrucción; los registros no listados quedaron intactos"))
		r.printSteps(s)
	}

	h("Estado final (justo antes de la última instrucción, normalmente la syscall de salida)")
	r.printFinal(s)
	switch res.Stopped {
	case "timeout":
		fmt.Println(s.c("31", "detenido: tiempo agotado (¿bucle infinito?)"))
	case "step-limit":
		fmt.Println(s.c("31", fmt.Sprintf("detenido: límite de instrucciones (%d) alcanzado (¿bucle infinito?)", r.opts.limit)))
	default:
		fmt.Printf("código de salida: %d\n", res.ExitCode)
	}
	return nil
}

func (r *report) printSteps(s style) {
	hdr := fmt.Sprintf("%5s  %-10s  %-28s  %s", "paso", "pc", "instrucción", "efecto")
	fmt.Println(s.c("1", hdr))
	for _, st := range r.Steps {
		if st.Label != "" {
			fmt.Println(s.c("33", st.Label+":"))
		}
		var fx []string
		for _, c := range st.Regs {
			fx = append(fx, fmt.Sprintf("%s: %s -> %s", c.Reg, trace.Val(c.From), s.c("32", trace.Val(c.To))))
		}
		for _, c := range st.Flags {
			fx = append(fx, s.c("35", fmt.Sprintf("%s %s", c.Flag, map[bool]string{true: "activado", false: "desactivado"}[c.To])))
		}
		if st.Branch != "" {
			fx = append(fx, s.c("34", st.Branch))
		}
		for _, w := range r.Watch {
			if v, ok := st.Watch[w]; ok {
				fx = append(fx, fmt.Sprintf("[%s=%s]", w, trace.Val(v)))
			}
		}
		fmt.Printf("%5d  0x%08x  %-28s  %s\n", st.N, st.PC, st.Text, strings.Join(fx, "; "))
	}
	if omitted := r.Result.Total - len(r.Steps); omitted > 0 {
		fmt.Printf("%s\n", s.c("2", fmt.Sprintf("... %d instrucciones más sin mostrar (aumenta con -steps N)", omitted)))
	}
}

func (r *report) printFinal(s style) {
	arch := qemulog.Arch(r.Arch)
	last := r.Result.Last
	if last.Regs == nil {
		fmt.Println("(no se capturó el estado de la CPU)")
		return
	}
	for i, reg := range qemulog.RegOrder(arch) {
		fmt.Printf("%-4s %s%s", reg, fmt.Sprintf("0x%016x", last.Regs[reg]), pad(i))
	}
	fmt.Printf("%-4s 0x%016x\n", map[qemulog.Arch]string{qemulog.X8664: "rip", qemulog.AArch64: "pc"}[arch], last.PC)
	var on []string
	for _, f := range qemulog.FlagOrder(arch) {
		if last.Flags[f] {
			on = append(on, s.c("35", f))
		} else {
			on = append(on, s.c("2", strings.ToLower(f)))
		}
	}
	fmt.Printf("flags: %s\n", strings.Join(on, " "))
}

func pad(i int) string {
	if i%4 == 3 {
		return "\n"
	}
	return "  "
}
