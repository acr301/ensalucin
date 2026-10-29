// Condicionales: cmp fija NZCV, b.cond lo lee; csel lo hace sin saltar
// arch: aarch64
.globl _start
.text
_start:
    mov     x0, #7
    mov     x1, #12
    cmp     x0, x1              // subs xzr, x0, x1: solo flags
    b.ge    a_is_max            // salta si x0 >= x1 (con signo): N == V
    mov     x2, x1              // b es el maximo
    b       done
a_is_max:
    mov     x2, x0
done:
    // la misma idea sin salto: x3 = min(x0, x1)
    cmp     x0, x1
    csel    x3, x0, x1, lt      // x3 = (lt) ? x0 : x1
    mov     x0, x2              // codigo de salida = maximo = 12
    mov     x8, #93
    svc     #0
