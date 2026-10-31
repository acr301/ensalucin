# Condicionales: cmp fija los flags, jcc los lee; cmov lo hace sin saltar
# arch: x86_64
.intel_syntax noprefix
.globl _start
.text
_start:
    mov     rax, 7
    mov     rbx, 12
    cmp     rax, rbx            # calcula rax - rbx, descarta el resultado, conserva los flags
    jge     a_is_max            # salta si rax >= rbx (con signo): SF == OF
    mov     rcx, rbx            # b es el maximo
    jmp     done
a_is_max:
    mov     rcx, rax
done:
    # la misma idea sin salto: rdx = min(rax, rbx)
    mov     rdx, rbx
    cmp     rax, rbx
    cmovl   rdx, rax            # copia solo si rax < rbx
    mov     rdi, rcx            # codigo de salida = maximo = 12
    mov     rax, 60
    syscall
