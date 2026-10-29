// Hola mundo: las llamadas al sistema write y exit en AArch64 Linux
// arch: aarch64
.globl _start

.section .rodata
msg:    .ascii "Hola, Arquitectura!\n"
.set    len, . - msg

.text
_start:
    mov     x0, #1              // fd 1 = stdout
    adr     x1, msg             // direccion del buffer (relativa a PC)
    mov     x2, #len            // numero de bytes
    mov     x8, #64             // numero de syscall: write
    svc     #0
    mov     x0, #0              // estado 0
    mov     x8, #93             // numero de syscall: exit
    svc     #0
