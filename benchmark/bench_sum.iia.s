.text
.balign 16
.globl sim_main
sim_main:
	endbr64
	movl $0, %ecx
	movl $0, %eax
.p2align 4
.Lbb2:
	cmpl $100000000, %ecx
	jg .Lbb4
	addl %ecx, %eax
	addl $1, %ecx
	jmp .Lbb2
.Lbb4:
	ret
.type sim_main, @function
.size sim_main, .-sim_main
/* end function sim_main */

.section .note.GNU-stack,"",@progbits
