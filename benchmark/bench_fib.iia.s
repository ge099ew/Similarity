.text
.balign 16
fibonacci:
	endbr64
	pushq %rbp
	movq %rsp, %rbp
	pushq %rbx
	pushq %r12
	movl %edi, %ebx
	cmpl $1, %ebx
	jle .Lbb2
	subq $16, %rsp
	movq %rsp, %r12
	movl %ebx, %edi
	subl $1, %edi
	callq fibonacci
	xchgl %eax, %ebx
	movl %ebx, (%r12)
	subq $16, %rsp
	movq %rsp, %r12
	movl %eax, %edi
	subl $2, %edi
	callq fibonacci
	movl %eax, (%r12)
	addl %ebx, %eax
	jmp .Lbb3
.Lbb2:
	movl %ebx, %eax
.Lbb3:
	movq %rbp, %rsp
	subq $16, %rsp
	popq %r12
	popq %rbx
	leave
	ret
.type fibonacci, @function
.size fibonacci, .-fibonacci
/* end function fibonacci */

.text
.balign 16
.globl sim_main
sim_main:
	endbr64
	pushq %rbp
	movq %rsp, %rbp
	movl $10, %edi
	callq fibonacci
	leave
	ret
.type sim_main, @function
.size sim_main, .-sim_main
/* end function sim_main */

.section .note.GNU-stack,"",@progbits
