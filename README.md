# ensalucin / asmlab

Laboratorio de ensamblador para Arquitectura de Computadores. Escribe
ensamblador x86_64 (Intel o AT&T) o AArch64, ensámblalo con `llvm-mc`, enlázalo
con `ld.lld` y ejecútalo con `qemu-user`. `asmlab` convierte el volcado
`-d cpu` de QEMU en una traza por instrucción: qué cambió cada instrucción
(registros y flags), si los saltos se tomaron y la codificación de máquina.

Todo corre dentro de un contenedor: no necesitas instalar LLVM ni QEMU.

## Primeros pasos (onboarding)

### 1. Requisitos
Solo [Docker](https://docs.docker.com/get-docker/) (Docker Desktop en macOS o
Windows). Comprueba que funciona con `docker info`.
En Apple Silicon el contenedor es arm64 y QEMU emula x86_64 sin problema.

### 2. Construir la imagen
Desde la raíz del repositorio (solo la primera vez, o tras cambiar el código):

```sh
docker build -t ensalucin .
```

La construcción compila el programa Go, ejecuta sus tests y copia el resultado
a una imagen Debian con `llvm`, `lld` y `qemu-user`. La imagen incluye los
ejemplos en `/examples`.

### 3. Ejecutar tu primer programa
El script `bin/asmlab` monta el directorio actual como `/work` dentro del
contenedor y ejecuta `asmlab`:

```sh
bin/asmlab examples                  # lista los ejemplos incluidos
bin/asmlab run x86_64/03_loop.s      # ejemplo incluido (la ruta es relativa a /examples)
```

Para usar tus propios ficheros, créalos en el directorio actual y ejecútalos:

```sh
bin/asmlab run mi_programa.s
```

Sin el script, el equivalente es:

```sh
docker run --rm -i -v "$PWD":/work ensalucin run mi_programa.s
```

(el punto de entrada de la imagen ya es `asmlab`; `-v` hace visibles tus
ficheros dentro del contenedor y permite que `-out` los escriba en tu
directorio). Para trabajar dentro del contenedor con una shell y las
herramientas LLVM/QEMU a mano:

```sh
docker run --rm -it -v "$PWD":/work --entrypoint bash ensalucin
```

### 4. Opciones útiles

```sh
bin/asmlab run mio.s -watch rax,rcx      # muestra esos registros tras cada paso
bin/asmlab run mio.s -listing            # añade direcciones y codificaciones
bin/asmlab run mio.s -json               # salida legible por máquinas
bin/asmlab run mio.s -no-trace           # solo salida del programa y estado final
bin/asmlab build mio.s -out build        # solo genera mio.o, mio.elf y mio.bin
```

Otras: `-steps` (filas de traza mostradas, 200 por defecto), `-limit` (tope de
instrucciones, 1M), `-timeout`, `-color never`, `-arch`, `-entry`.

La arquitectura se elige con `-arch`; si no, con un comentario
`# arch: aarch64` en el fichero; si no, se detecta a partir del código
(`.intel_syntax` implica x86_64). La sintaxis es la que diga el fuente
(`.intel_syntax noprefix` para Intel).

Reglas del programa: debe exportar `_start` y terminar con la syscall `exit`
(x86_64: `rax=60; syscall`; AArch64: `x8=93; svc #0`). El código de salida se
muestra al final.

### 5. Leer la traza

```
paso  pc          instrucción                efecto
sum_loop:
    3  0x0020112a  add rax, rcx               rax: 0 -> 5; ZF desactivado
    5  0x00201130  jne 0x20112a <sum_loop>    tomado -> sum_loop
```

Cada fila es una instrucción; "efecto" lista los registros y flags que
cambió y, en los saltos, si se tomó o no. El "estado final" es el de justo
antes de la última instrucción (la syscall de salida): QEMU imprime el estado
antes de cada instrucción, así que de la última no hay estado posterior.

## Desarrollo: ejecutar Go en el contenedor

La imagen final (`ensalucin`) solo contiene el binario compilado, no el
compilador de Go. Para desarrollar sin instalar Go en tu máquina, usa la
imagen oficial de Go montando el repositorio:

```sh
# tests
docker run --rm -v "$PWD":/src -w /src golang:1.25-bookworm go test ./...

# vet / formato
docker run --rm -v "$PWD":/src -w /src golang:1.25-bookworm sh -c 'go vet ./... && gofmt -l .'

# compilar el binario para Linux en ./asmlab-linux
docker run --rm -v "$PWD":/src -w /src -e CGO_ENABLED=0 golang:1.25-bookworm \
  go build -o asmlab-linux ./cmd/asmlab
```

Con Go instalado en local, `go test ./...` y `go build ./cmd/asmlab` funcionan
igual, pero el binario resultante solo es útil dentro del contenedor (necesita
`llvm-mc`, `ld.lld` y `qemu-*` en el PATH).

Ciclo habitual al modificar el código:

```sh
docker build -t ensalucin .     # recompila, pasa los tests y rehace la imagen
bin/asmlab run x86_64/05_stack_call.s
```

Para probar cambios sin reconstruir la imagen cada vez, monta el binario
compilado sobre el de la imagen:

```sh
docker run --rm -v "$PWD":/work -v "$PWD/asmlab-linux":/usr/local/bin/asmlab ensalucin run x86_64/03_loop.s
```

## Cómo funciona la traza

`qemu-<arch> -one-insn-per-tb -d cpu,nochain -D /dev/fd/3` hace que QEMU
vuelque todos los registros antes de cada instrucción. El volcado se transmite
por una tubería, de modo que un bucle infinito se corta en `-limit` en vez de
llenar el disco. El efecto de la instrucción N es la diferencia entre las
instantáneas N y N+1; el texto y las codificaciones vienen de `llvm-objdump`.

Estructura del código: `cmd/asmlab` (CLI e informe), `internal/qemulog`
(analizador del volcado), `internal/toolchain` (llvm-mc, lld, objdump, qemu),
`internal/trace` (cálculo de diferencias y saltos).

## Ejemplos (`examples/`)

Cada arquitectura tiene: `01_hello` (syscalls), `02_conditionals`
(cmp/jcc/cmov o cmp/b.cond/csel), `03_loop`, `04_flags_and_arith` (acarreo,
desbordamiento, suma con acarreo, producto de 128 bits) y `05_stack_call`
(factorial recursivo).

Nota Intel: en sintaxis Intel, `mov rdx, len` carga de memoria; para usar la
constante escribe `mov rdx, offset len`.
