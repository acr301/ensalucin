# Pila y llamadas: factorial(5) recursivo con call/ret/push/pop
# arch: x86_64
.intel_syntax noprefix
.globl _start
.text
_start:
    mov     rdi, 5
    call    factorial           # apila la direccion de retorno y salta
    mov     rdi, rax            # codigo de salida = 120
    mov     rax, 60
    syscall

factorial:                      # rdi = n, devuelve rax = n!
    cmp     rdi, 1
    jbe     base
    push    rdi                 # guarda n en la pila (rsp -= 8)
    dec     rdi
    call    factorial
    pop     rdi                 # recupera n (rsp += 8)
    imul    rax, rdi
    ret                         # desapila la direccion de retorno en rip
base:
    mov     rax, 1
    ret
