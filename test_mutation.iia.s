.text
.balign 16
.globl sim_main
sim_main:
	endbr64
	movl $30, %eax
	ret
.type sim_main, @function
.size sim_main, .-sim_main
/* end function sim_main */

.section .note.GNU-stack,"",@progbits
