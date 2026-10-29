// asmlab ensambla, enlaza y ejecuta ensamblador bajo qemu-user, y explica la
// ejecución instrucción a instrucción.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"ensalucin/internal/qemulog"
	"ensalucin/internal/toolchain"
	"ensalucin/internal/trace"
)

const usage = `asmlab - laboratorio de ensamblador para Arquitectura de Computadores (llvm-mc + qemu-user)

Uso:
  asmlab run   [flags] prog.s     ensambla, enlaza, ejecuta y traza
  asmlab build [flags] prog.s     solo ensambla y enlaza; genera .o, .elf, .bin
  asmlab examples                 lista los ejemplos incluidos

Usa "asmlab run -h" para ver las opciones. Los ejemplos se pueden nombrar
relativos al directorio de ejemplos, p. ej.  asmlab run x86_64/03_loop.s
`

type options struct {
	arch    string
	out     string
	rows    int
	limit   int
	timeout time.Duration
	asJSON  bool
	noTrace bool
	listing bool
	watch   string
	color   string
	entry   string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "run":
		err = cmdRun(os.Args[2:], true)
	case "build":
		err = cmdRun(os.Args[2:], false)
	case "examples":
		err = cmdExamples()
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "asmlab:", err)
		os.Exit(1)
	}
}

func examplesDir() string {
	if d := os.Getenv("ASMLAB_EXAMPLES"); d != "" {
		return d
	}
	for _, d := range []string{"/examples", "examples"} {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d
		}
	}
	return "examples"
}

func cmdExamples() error {
	files, _ := filepath.Glob(filepath.Join(examplesDir(), "*", "*.s"))
	if len(files) == 0 {
		return fmt.Errorf("no se encontraron ejemplos en %s", examplesDir())
	}
	sort.Strings(files)
	for _, f := range files {
		rel, _ := filepath.Rel(examplesDir(), f)
		fmt.Printf("%-32s %s\n", rel, firstComment(f))
	}
	return nil
}

func firstComment(path string) string {
	b, _ := os.ReadFile(path)
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "#") || strings.HasPrefix(l, "//") {
			if t := strings.TrimSpace(strings.TrimLeft(l, "#/")); t != "" && !strings.HasPrefix(t, "arch:") {
				return t
			}
		}
	}
	return ""
}

func cmdRun(args []string, execute bool) error {
	var o options
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.StringVar(&o.arch, "arch", "", "x86_64 o aarch64 (por defecto: se detecta a partir del código)")
	fs.StringVar(&o.out, "out", "", "directorio donde guardar prog.o / prog.elf / prog.bin (por defecto: directorio temporal; con 'build': .)")
	fs.IntVar(&o.rows, "steps", 200, "máximo de filas de traza a imprimir")
	fs.IntVar(&o.limit, "limit", 1_000_000, "detiene el programa tras este número de instrucciones")
	fs.DurationVar(&o.timeout, "timeout", 10*time.Second, "límite de tiempo real")
	fs.BoolVar(&o.asJSON, "json", false, "salida legible por máquinas (JSON)")
	fs.BoolVar(&o.noTrace, "no-trace", false, "omite la traza por instrucción")
	fs.BoolVar(&o.listing, "listing", false, "imprime el listado desensamblado con las codificaciones")
	fs.StringVar(&o.watch, "watch", "", "registros separados por comas a mostrar tras cada paso, p. ej. rax,rcx o x0,x1")
	fs.StringVar(&o.color, "color", "auto", "auto, always (siempre) o never (nunca)")
	fs.StringVar(&o.entry, "entry", "_start", "símbolo del punto de entrada")
	// permite opciones después del nombre del fichero
	var file string
	var rest []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && file == "" && !isFlagValue(rest) {
			file = a
		} else {
			rest = append(rest, a)
		}
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if file == "" {
		return fmt.Errorf("falta el fichero de ensamblador")
	}
	src, err := resolve(file)
	if err != nil {
		return err
	}
	text, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	arch, err := detectArch(o.arch, src, string(text))
	if err != nil {
		return err
	}
	intel := strings.Contains(string(text), ".intel_syntax")

	dir := o.out
	if dir == "" && !execute {
		dir = "."
	}
	if dir == "" {
		if dir, err = os.MkdirTemp("", "asmlab"); err != nil {
			return err
		}
		defer os.RemoveAll(dir)
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := filepath.Join(dir, strings.TrimSuffix(filepath.Base(src), filepath.Ext(src)))
	obj, elf, bin := base+".o", base+".elf", base+".bin"

	if err := toolchain.Assemble(arch, src, obj); err != nil {
		return err
	}
	if err := toolchain.Link(obj, elf, o.entry); err != nil {
		return err
	}
	if err := toolchain.RawBinary(elf, bin); err != nil {
		return err
	}
	lst, err := toolchain.Disassemble(arch, elf, intel)
	if err != nil {
		return err
	}

	rep := &report{Source: src, Arch: string(arch), Intel: intel, Listing: lst, opts: o}
	if !execute {
		rep.Files = []string{obj, elf, bin}
		rep.opts.listing = true
		return rep.print()
	}
	var watch []string
	for _, w := range strings.Split(o.watch, ",") {
		if w = strings.ToLower(strings.TrimSpace(w)); w != "" {
			watch = append(watch, w)
		}
	}
	res, err := toolchain.Run(context.Background(), arch, elf, toolchain.RunOpts{
		Timeout: o.timeout, Keep: o.rows, Limit: o.limit, Stdin: os.Stdin})
	if err != nil {
		return err
	}
	rep.Result = res
	rep.Steps = trace.Build(arch, res.Head, lst, o.rows, watch)
	rep.Watch = watch
	if o.out != "" {
		rep.Files = []string{obj, elf, bin}
	}
	if o.asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep.jsonView())
	}
	return rep.print()
}

// isFlagValue indica si el último argumento recogido es una opción que espera
// un valor (para que "-arch aarch64 prog.s" no confunda aarch64 con el fichero).
func isFlagValue(rest []string) bool {
	if len(rest) == 0 {
		return false
	}
	last := rest[len(rest)-1]
	if strings.Contains(last, "=") || !strings.HasPrefix(last, "-") {
		return false
	}
	switch strings.TrimLeft(last, "-") {
	case "json", "no-trace", "listing":
		return false
	}
	return true
}

func resolve(f string) (string, error) {
	if _, err := os.Stat(f); err == nil {
		return f, nil
	}
	for _, c := range []string{filepath.Join(examplesDir(), f), filepath.Join(examplesDir(), f+".s")} {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("%s: no existe el fichero (prueba 'asmlab examples')", f)
}

var (
	pragmaRE = regexp.MustCompile(`(?m)^\s*(?:#|//|;|@)\s*arch:\s*(\w+)`)
	a64RE    = regexp.MustCompile(`(?m)^\s*(?:svc|bl|b\.\w+|cbn?z|ldp|stp)\b|\b[xw](?:[0-9]|[12][0-9]|30)\b|\bxzr\b`)
)

func detectArch(flagVal, path, src string) (qemulog.Arch, error) {
	pick := func(s string) (qemulog.Arch, error) {
		switch strings.ToLower(s) {
		case "x86_64", "x86-64", "amd64", "x64":
			return qemulog.X8664, nil
		case "aarch64", "arm64":
			return qemulog.AArch64, nil
		}
		return "", fmt.Errorf("arquitectura no soportada %q (usa x86_64 o aarch64)", s)
	}
	if flagVal != "" {
		return pick(flagVal)
	}
	if m := pragmaRE.FindStringSubmatch(src); m != nil {
		return pick(m[1])
	}
	switch {
	case strings.Contains(src, ".intel_syntax"):
		return qemulog.X8664, nil
	case strings.Contains(filepath.ToSlash(path), "/aarch64/"), a64RE.MatchString(src):
		return qemulog.AArch64, nil
	}
	return qemulog.X8664, nil
}
