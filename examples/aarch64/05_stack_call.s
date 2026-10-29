// Pila y llamadas: factorial(5) recursivo con bl/ret y stp/ldp
// arch: aarch64
.globl _start
.text
_start:
    mov     x0, #5
    bl      factorial           // x30 (registro de enlace) = direccion de retorno
    mov     x8, #93             // codigo de salida = 120
    svc     #0

factorial:                      // x0 = n, devuelve x0 = n!
    cmp     x0, #1
    b.ls    base
    stp     x0, x30, [sp, #-16]!    // apila n y la direccion de retorno (pre-indexado)
    sub     x0, x0, #1
    bl      factorial
    ldp     x1, x30, [sp], #16      // las desapila (post-indexado)
    mul     x0, x0, x1
    ret                         // salta a x30
base:
    mov     x0, #1
    ret
