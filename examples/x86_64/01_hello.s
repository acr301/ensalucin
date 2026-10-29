# Hola mundo: las llamadas al sistema write(2) y exit(2) en x86_64 Linux
# arch: x86_64
.intel_syntax noprefix
.globl _start

.section .rodata
msg:    .ascii "Hola, Arquitectura!\n"
.set    len, . - msg

.text
_start:
    mov     rax, 1              # numero de syscall: write
    mov     rdi, 1              # fd 1 = stdout
    lea     rsi, [rip + msg]    # direccion del buffer (relativa a PC)
    mov     rdx, offset len     # numero de bytes ("offset" = el valor, no una carga de memoria)
    syscall
    mov     rax, 60             # numero de syscall: exit
    xor     rdi, rdi            # estado 0 (xor reg,reg es el modismo para poner a cero)
    syscall
