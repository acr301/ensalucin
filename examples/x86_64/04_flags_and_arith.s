# Por dentro: como se producen los flags de acarreo, desbordamiento, signo y cero
# arch: x86_64
.intel_syntax noprefix
.globl _start
.text
_start:
    mov     rax, -1             # 0xFFFF...FFFF
    add     rax, 1              # da la vuelta a 0: ZF=1, CF=1 (acarreo sin signo)
    mov     rbx, 0x7FFFFFFFFFFFFFFF
    add     rbx, 1              # maximo con signo + 1: OF=1, SF=1 (desbordamiento con signo)
    mov     rcx, 5
    sub     rcx, 5              # ZF=1
    mov     rdx, 3
    sub     rdx, 5              # -2: SF=1, CF=1 (prestamo)
    # suma de 128 bits: mitades bajas con add, altas con adc (add + acarreo)
    mov     r8, -1              # parte baja de A
    mov     r9, 0               # parte alta de A
    add     r8, 1               # baja = 0, con acarreo de salida
    adc     r9, 0               # la alta recibe el acarreo: r9 = 1
    # mul: rdx:rax = rax * operando (producto completo de 128 bits)
    mov     rax, 0x8000000000000000
    mov     rsi, 4
    mul     rsi                 # rdx = 2, rax = 0
    xor     rdi, rdi
    mov     rax, 60
    syscall
