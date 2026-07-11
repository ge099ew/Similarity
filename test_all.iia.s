.text
.balign 16
test_pointer:
	endbr64
	movl $42, %eax
	ret
.type test_pointer, @function
.size test_pointer, .-test_pointer
/* end function test_pointer */

.text
.balign 16
test_cast:
	endbr64
	movl $0, %eax
	ret
.type test_cast, @function
.size test_cast, .-test_cast
/* end function test_cast */

.text
.balign 16
test_risk:
	endbr64
	subq $16, %rsp
	movl $99, 0(%rsp)
	movl $0, %eax
	addq $16, %rsp
	ret
.type test_risk, @function
.size test_risk, .-test_risk
/* end function test_risk */

.text
.balign 16
test_overflow_check:
	endbr64
	movl $2147483647, %eax
	ret
.type test_overflow_check, @function
.size test_overflow_check, .-test_overflow_check
/* end function test_overflow_check */

.text
.balign 16
.globl sim_main
sim_main:
	endbr64
	pushq %rbp
	movq %rsp, %rbp
	subq $8, %rsp
	pushq %rbx
	callq test_pointer
	movl %eax, %ebx
	callq test_cast
	callq test_risk
	callq test_overflow_check
	movl %ebx, %eax
	popq %rbx
	leave
	ret
.type sim_main, @function
.size sim_main, .-sim_main
/* end function sim_main */

.section .note.GNU-stack,"",@progbits
