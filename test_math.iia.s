.text
.balign 16
absolute_value:
	endbr64
	cmpl $0, %edi
	jl .Lbb2
	movl %edi, %eax
	jmp .Lbb3
.Lbb2:
	movl $0, %eax
	subl %edi, %eax
.Lbb3:
	ret
.type absolute_value, @function
.size absolute_value, .-absolute_value
/* end function absolute_value */

.text
.balign 16
maximum:
	endbr64
	movl %esi, %ecx
	movl %edi, %eax
	cmpl %ecx, %eax
	jg .Lbb6
	movl %ecx, %eax
.Lbb6:
	ret
.type maximum, @function
.size maximum, .-maximum
/* end function maximum */

.text
.balign 16
.globl sim_main
sim_main:
	endbr64
	pushq %rbp
	movq %rsp, %rbp
	subq $8, %rsp
	pushq %rbx
	movl $4294967291, %edi
	callq absolute_value
	movl %eax, %ebx
	movl $7, %esi
	movl $3, %edi
	callq maximum
	movl %ebx, %eax
	popq %rbx
	leave
	ret
.type sim_main, @function
.size sim_main, .-sim_main
/* end function sim_main */

.section .note.GNU-stack,"",@progbits
