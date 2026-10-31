// Bucle: suma 1..5; subs actualiza los flags, b.ne lee Z
// arch: aarch64
.globl _start
.text
_start:
    mov     x0, #0              // acumulador
    mov     x1, #5              // contador
sum_loop:
    add     x0, x0, x1
    subs    x1, x1, #1          // el sufijo 's' significa "ademas fija los flags"
    b.ne    sum_loop
    mov     x8, #93             // codigo de salida = x0 = 15
    svc     #0
