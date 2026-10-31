# Bucle: suma 1..5 con dec/jnz (el flag de cero controla el bucle)
# arch: x86_64
.intel_syntax noprefix
.globl _start
.text
_start:
    xor     rax, rax            # acumulador
    mov     rcx, 5              # contador
sum_loop:
    add     rax, rcx
    dec     rcx                 # activa ZF cuando llega a 0
    jnz     sum_loop
    mov     rdi, rax            # codigo de salida = 15
    mov     rax, 60
    syscall
