.text
.balign 16
fibonacci:
	endbr64
	movl %edi, %eax
	cmpl $1, %eax
	jle .Lbb2
	movl $0, %eax
.Lbb2:
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
