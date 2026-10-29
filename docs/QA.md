# Guía de QA

Objetivo: comprobar en **tu** máquina que `asmlab` funciona y buscar fallos con
**tus propios** programas de ensamblador. Requisitos: Docker y este repositorio.

Anota para cada caso: ✅ pasa, ❌ falla (con la salida) o ⚠️ raro. Al final,
rellena la tabla de entorno y comenta el resultado en el PR.

## 0. Entorno

| Dato | Valor |
|---|---|
| SO y versión | |
| CPU (Intel/AMD, Apple Silicon, ARM) | |
| `docker --version` | |
| Docker Desktop / Engine / Podman | |

## 1. Construcción

1. `docker build -t ensalucin .` termina sin errores (incluye `go test`).
2. `bin/asmlab examples` lista 10 ejemplos (5 por arquitectura).
3. `docker image ls ensalucin` muestra la imagen (anota el tamaño).

## 2. Ejemplos incluidos

Ejecuta cada uno con `bin/asmlab run <ejemplo>` y compara con lo esperado:

| Ejemplo | Esperado |
|---|---|
| `x86_64/01_hello.s`, `aarch64/01_hello.s` | imprime `Hola, Arquitectura!`, código de salida 0 |
| `*/02_conditionals.s` | código de salida 12; el salto `jge` / `b.ge` aparece como *tomado* o *no tomado* |
| `*/03_loop.s` | código de salida 15; la última iteración dice *no tomado, continúa en secuencia* |
| `*/04_flags_and_arith.s` | código de salida 0; en x86 `add rax, 1` activa `ZF` y `CF`; en AArch64 `adds x0, x0, #1` activa `Z` y `C` |
| `*/05_stack_call.s` | código de salida 120; `rsp`/`sp` baja al apilar y vuelve a su valor |

## 3. Tus propios programas

Escribe **al menos tres** programas tuyos en el directorio actual (no en
`examples/`) y ejecútalos con `bin/asmlab run mio.s`. Ideas:

- **x86_64 en sintaxis Intel** (`.intel_syntax noprefix`): fibonacci, máximo de
  un arreglo, invertir una cadena, `shl`/`shr`, `div`/`idiv`, `loop`.
- **x86_64 en sintaxis AT&T** (sin `.intel_syntax`): el mismo programa con
  `%rax`, `$5`, sufijos `movq`. El listado debe salir en AT&T.
- **AArch64**: bucles con `cbnz`, acceso a memoria con `ldr`/`str` y datos en
  `.data`, `ldp`/`stp`, `csel`, `madd`.
- Un programa que escriba con `write` en un bucle (varias líneas de salida).
- Un programa que lea de stdin (`read`) y pruébalo con
  `echo hola | bin/asmlab run mio.s`.

Para cada uno verifica:

1. Los valores de registros de la traza coinciden con lo que calculas a mano.
2. Los flags activados/desactivados tras cada instrucción son los esperados.
3. Los saltos condicionales muestran *tomado* / *no tomado* correctamente.
4. El código de salida coincide con el que programaste.
5. `-listing`: las codificaciones coinciden con tu referencia (manual, `llvm-mc
   --show-encoding` o un ensamblador en línea).

## 4. Opciones

Con uno de tus programas:

| Comando | Esperado |
|---|---|
| `bin/asmlab run mio.s -arch aarch64` sobre código x86 | error de `llvm-mc` claro, código de retorno ≠ 0 |
| `-watch rax,rcx` (o `x0,x1`) | `[rax=…]` al final de cada fila |
| `-steps 5` | solo 5 filas y el aviso `... N instrucciones más sin mostrar` |
| `-no-trace` | solo salida del programa, estado final y código de salida |
| `-json` | JSON válido (`bin/asmlab run mio.s -json \| python3 -m json.tool`) |
| `-color never` / `-color always` | sin / con secuencias ANSI |
| `build mio.s -out build` | crea `build/mio.o`, `.elf`, `.bin`; el `.bin` tiene el tamaño de `.text` |
| `-entry inicio` con `.globl inicio` | el programa arranca en esa etiqueta |

## 5. Casos de error y límites

| Caso | Esperado |
|---|---|
| Error de sintaxis (`mov rax,`) | mensaje de `llvm-mc` con línea y columna; no hay traza |
| Falta `.globl _start` | error del enlazador o aviso claro, sin colgarse |
| Fichero inexistente | `no existe el fichero (prueba 'asmlab examples')` |
| Bucle infinito (`b .` / `jmp .`) con `-limit 5000` | termina con `detenido: límite de instrucciones` |
| Bucle infinito sin `-limit` | termina por límite o `-timeout`, sin llenar el disco |
| Programa que acaba sin `exit` (se cae por el final) | señal/segfault reportado, sin colgarse |
| Acceso a memoria inválida (`mov rax, [0]`) | segmentation fault de QEMU visible en «Salida del programa» |

## 6. Plataforma

- En Apple Silicon o ARM Linux: los ejemplos **x86_64** corren (emulados).
- En Intel/AMD: los ejemplos **aarch64** corren (emulados).
- Windows: probar desde WSL2; `bin/asmlab` es un script `sh`. Si usas
  PowerShell, usa `docker run` directo (ver README) y anota si hay problemas
  con rutas o finales de línea CRLF en los `.s`.
- Los ficheros creados con `-out` pertenecen a tu usuario (en Linux pueden
  quedar como `root`; anótalo si pasa).

## 7. Qué reportar

Abre un comentario en el PR (o un issue) con: caso fallido, comando exacto, el
`.s` mínimo que lo reproduce, la salida completa y la tabla de entorno.
Cualquier discrepancia entre la traza y el comportamiento real de la CPU es
un bug de alta prioridad, aunque sea de un solo flag.
