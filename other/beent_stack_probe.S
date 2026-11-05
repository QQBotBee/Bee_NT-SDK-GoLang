/* x86 Windows allocating stack probe. EAX contains the requested byte count.
 * Touch every guard page before moving ESP; preserve ECX and the return address.
 * Matches the allocating __alloca ABI emitted by Clang for i686-windows-gnu. */
    .text
    .globl __alloca
    .def __alloca; .scl 2; .type 32; .endef
__alloca:
    pushl %ecx
    cmpl $0x1000, %eax
    leal 8(%esp), %ecx
    jb 2f
1:
    subl $0x1000, %ecx
    testl %ecx, (%ecx)
    subl $0x1000, %eax
    cmpl $0x1000, %eax
    ja 1b
2:
    subl %eax, %ecx
    testl %ecx, (%ecx)
    leal 4(%esp), %eax
    movl %ecx, %esp
    movl -4(%eax), %ecx
    pushl (%eax)
    subl %esp, %eax
    ret
