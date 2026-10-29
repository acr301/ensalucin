package qemulog

import (
	"strings"
	"testing"
)

const x86Dump = `
RAX=0000000000000000 RBX=0000000000000000 RCX=0000000000000005 RDX=0000000000000000
RSI=0000000000000000 RDI=0000000000000000 RBP=0000000000000000 RSP=00007ffe00000000
R8 =0000000000000000 R9 =0000000000000000 R10=0000000000000000 R11=0000000000000000
R12=0000000000000000 R13=0000000000000000 R14=0000000000000000 R15=0000000000000000
RIP=0000000000201120 RFL=00000246 [---Z-P-] CPL=3 II=0 A20=1 SMM=0 HLT=0
ES =0000 0000000000000000 00000000 00000000
RAX=000000000000000f RBX=0000000000000000 RCX=0000000000000000 RDX=0000000000000000
RSI=0000000000000000 RDI=0000000000000000 RBP=0000000000000000 RSP=00007ffe00000000
R8 =0000000000000000 R9 =0000000000000000 R10=0000000000000000 R11=0000000000000000
R12=0000000000000000 R13=0000000000000000 R14=0000000000000000 R15=0000000000000000
RIP=0000000000201125 RFL=00000203 [-------] CPL=3 II=0 A20=1 SMM=0 HLT=0
`

func TestX86(t *testing.T) {
	var got []State
	if err := Stream(strings.NewReader(x86Dump), X8664, func(s State) bool { got = append(got, s); return true }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(got))
	}
	if got[0].Regs["r8"] != 0 || got[0].Regs["rcx"] != 5 || got[0].PC != 0x201120 {
		t.Errorf("bad first block: %+v", got[0])
	}
	if !got[0].Flags["ZF"] || !got[0].Flags["PF"] || got[0].Flags["CF"] {
		t.Errorf("bad flags: %+v", got[0].Flags)
	}
	if got[1].Regs["rax"] != 15 || !got[1].Flags["CF"] || got[1].Flags["ZF"] {
		t.Errorf("bad second block: %+v", got[1])
	}
}

func a64Dump(pc string, nzcv string) string {
	var b strings.Builder
	b.WriteString(" PC=" + pc + " ")
	for i := 0; i < 31; i++ {
		b.WriteString("X" + pad(i) + "=0000000000000000 ")
		if i%4 == 3 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n SP=0000004000800000\nPSTATE=" + nzcv + "0000000 -ZC- EL0t\n")
	return b.String()
}

func pad(i int) string {
	if i < 10 {
		return "0" + string(rune('0'+i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

func TestAArch64AndTruncation(t *testing.T) {
	in := a64Dump("0000000000210120", "6") + a64Dump("0000000000210124", "8") + " PC=0000000000210128 X00=0000000000000001\n"
	var got []State
	Stream(strings.NewReader(in), AArch64, func(s State) bool { got = append(got, s); return true })
	if len(got) != 2 {
		t.Fatalf("want 2 complete blocks (partial dropped), got %d", len(got))
	}
	if !got[0].Flags["Z"] || !got[0].Flags["C"] || got[0].Flags["N"] {
		t.Errorf("flags: %+v", got[0].Flags)
	}
	if !got[1].Flags["N"] || got[1].PC != 0x210124 {
		t.Errorf("second: %+v", got[1])
	}
}
