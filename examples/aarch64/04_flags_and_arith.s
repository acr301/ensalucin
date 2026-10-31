// Por dentro: como se producen N, Z, C, V (ojo: en la resta C significa "no hubo prestamo")
// arch: aarch64
.globl _start
.text
_start:
    mov     x0, #-1             // 0xFFFF...FFFF
    adds    x0, x0, #1          // da la vuelta a 0: Z=1, C=1 (acarreo de salida)
    movz    x1, #0x7fff, lsl #48
    movk    x1, #0xffff, lsl #32
    movk    x1, #0xffff, lsl #16
    movk    x1, #0xffff         // x1 = 0x7FFFFFFFFFFFFFFF
    adds    x1, x1, #1          // desbordamiento con signo: V=1, N=1
    mov     x2, #5
    subs    x2, x2, #5          // Z=1, C=1 (no hubo prestamo)
    mov     x3, #3
    subs    x3, x3, #5          // -2: N=1, C=0 (prestamo)
    // suma de 128 bits: adds en las mitades bajas, adc (suma con acarreo) en las altas
    mov     x4, #-1             // parte baja de A
    mov     x5, #0              // parte alta de A
    adds    x4, x4, #1          // baja = 0, con acarreo de salida
    adc     x5, x5, xzr         // la alta recibe el acarreo: x5 = 1
    // el producto completo de 128 bits necesita dos instrucciones: mul (baja) y umulh (alta)
    movz    x6, #0x8000, lsl #48
    mov     x7, #4
    mul     x9, x6, x7          // 64 bits bajos = 0
    umulh   x10, x6, x7         // 64 bits altos = 2
    mov     x0, #0
    mov     x8, #93
    svc     #0
