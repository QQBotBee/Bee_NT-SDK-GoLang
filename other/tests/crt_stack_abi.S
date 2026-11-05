/* Exercise the compiler helper's unusual EAX/ESP ABI at page boundaries. */
    .text
    .globl _test_alloca_size
    .def _test_alloca_size; .scl 2; .type 32; .endef
_test_alloca_size:
    pushl %ebx
    pushl %esi
    pushl %edi
    pushl %ebp
    movl %esp, %ebx
    movl 20(%esp), %esi
    movl $0x12345678, %ecx
    movl $0x34567812, %edx
    movl $0x56781234, %edi
    movl $0x78123456, %ebp
    movl %esi, %eax
    call __alloca
    cmpl %esi, %eax
    jne 1f
    subl %esi, %ebx
    cmpl %ebx, %esp
    leal (%ebx,%esi), %ebx
    jne 1f
    cmpl $0x12345678, %ecx
    jne 1f
    cmpl $0x34567812, %edx
    jne 1f
    cmpl $0x56781234, %edi
    jne 1f
    cmpl $0x78123456, %ebp
    jne 1f
    movl $1, %eax
    jmp 2f
1:
    xorl %eax, %eax
2:
    movl %ebx, %esp
    popl %ebp
    popl %edi
    popl %esi
    popl %ebx
    ret
