// +build amd64,!generic

#define storeBlock(a0,a1,a2,a3, r) \
	MOVQ a0,  0+r \
	MOVQ a1,  8+r \
	MOVQ a2, 16+r \
	MOVQ a3, 24+r \

#define loadBlock(r, a0,a1,a2,a3) \
	MOVQ  0+r, a0 \
	MOVQ  8+r, a1 \
	MOVQ 16+r, a2 \
	MOVQ 24+r, a3 \

#define gfpCarry(a0,a1,a2,a3,a4, b0,b1,b2,b3,b4) \
	MOVQ a0, b0 \
	MOVQ a1, b1 \
	MOVQ a2, b2 \
	MOVQ a3, b3 \
	MOVQ a4, b4 \
	SUBQ ·p2+0(SB), b0 \
	SBBQ ·p2+8(SB), b1 \
	SBBQ ·p2+16(SB), b2 \
	SBBQ ·p2+24(SB), b3 \
	SBBQ $0, b4 \
	CMOVQCC b0, a0 \
	CMOVQCC b1, a1 \
	CMOVQCC b2, a2 \
	CMOVQCC b3, a3 \

#define mul(a0,a1,a2,a3, rb, stack) \
	MOVQ a0, AX \
	MULQ 0+rb \
	MOVQ AX, R8 \
	MOVQ DX, R9 \
	MOVQ a0, AX \
	MULQ 8+rb \
	ADDQ AX, R9 \
	ADCQ $0, DX \
	MOVQ DX, R10 \
	MOVQ a0, AX \
	MULQ 16+rb \
	ADDQ AX, R10 \
	ADCQ $0, DX \
	MOVQ DX, R11 \
	MOVQ a0, AX \
	MULQ 24+rb \
	ADDQ AX, R11 \
	ADCQ $0, DX \
	MOVQ DX, R12 \
	storeBlock(R8,R9,R10,R11, 0+stack) \
	MOVQ R12, 32+stack \
	MOVQ a1, AX \
	MULQ 0+rb \
	MOVQ AX, R8 \
	MOVQ DX, R9 \
	MOVQ a1, AX \
	MULQ 8+rb \
	ADDQ AX, R9 \
	ADCQ $0, DX \
	MOVQ DX, R10 \
	MOVQ a1, AX \
	MULQ 16+rb \
	ADDQ AX, R10 \
	ADCQ $0, DX \
	MOVQ DX, R11 \
	MOVQ a1, AX \
	MULQ 24+rb \
	ADDQ AX, R11 \
	ADCQ $0, DX \
	MOVQ DX, R12 \
	ADDQ 8+stack, R8 \
	ADCQ 16+stack, R9 \
	ADCQ 24+stack, R10 \
	ADCQ 32+stack, R11 \
	ADCQ $0, R12 \
	storeBlock(R8,R9,R10,R11, 8+stack) \
	MOVQ R12, 40+stack \
	MOVQ a2, AX \
	MULQ 0+rb \
	MOVQ AX, R8 \
	MOVQ DX, R9 \
	MOVQ a2, AX \
	MULQ 8+rb \
	ADDQ AX, R9 \
	ADCQ $0, DX \
	MOVQ DX, R10 \
	MOVQ a2, AX \
	MULQ 16+rb \
	ADDQ AX, R10 \
	ADCQ $0, DX \
	MOVQ DX, R11 \
	MOVQ a2, AX \
	MULQ 24+rb \
	ADDQ AX, R11 \
	ADCQ $0, DX \
	MOVQ DX, R12 \
	ADDQ 16+stack, R8 \
	ADCQ 24+stack, R9 \
	ADCQ 32+stack, R10 \
	ADCQ 40+stack, R11 \
	ADCQ $0, R12 \
	storeBlock(R8,R9,R10,R11, 16+stack) \
	MOVQ R12, 48+stack \
	MOVQ a3, AX \
	MULQ 0+rb \
	MOVQ AX, R8 \
	MOVQ DX, R9 \
	MOVQ a3, AX \
	MULQ 8+rb \
	ADDQ AX, R9 \
	ADCQ $0, DX \
	MOVQ DX, R10 \
	MOVQ a3, AX \
	MULQ 16+rb \
	ADDQ AX, R10 \
	ADCQ $0, DX \
	MOVQ DX, R11 \
	MOVQ a3, AX \
	MULQ 24+rb \
	ADDQ AX, R11 \
	ADCQ $0, DX \
	MOVQ DX, R12 \
	ADDQ 24+stack, R8 \
	ADCQ 32+stack, R9 \
	ADCQ 40+stack, R10 \
	ADCQ 48+stack, R11 \
	ADCQ $0, R12 \
	storeBlock(R8,R9,R10,R11, 24+stack) \
	MOVQ R12, 56+stack \

#define gfpReduce(stack) \
	MOVQ ·np+0(SB), AX \
	MULQ 0+stack \
	MOVQ AX, R8 \
	MOVQ DX, R9 \
	MOVQ ·np+0(SB), AX \
	MULQ 8+stack \
	ADDQ AX, R9 \
	ADCQ $0, DX \
	MOVQ DX, R10 \
	MOVQ ·np+0(SB), AX \
	MULQ 16+stack \
	ADDQ AX, R10 \
	ADCQ $0, DX \
	MOVQ DX, R11 \
	MOVQ ·np+0(SB), AX \
	MULQ 24+stack \
	ADDQ AX, R11 \
	MOVQ ·np+8(SB), AX \
	MULQ 0+stack \
	MOVQ AX, R12 \
	MOVQ DX, R13 \
	MOVQ ·np+8(SB), AX \
	MULQ 8+stack \
	ADDQ AX, R13 \
	ADCQ $0, DX \
	MOVQ DX, R14 \
	MOVQ ·np+8(SB), AX \
	MULQ 16+stack \
	ADDQ AX, R14 \
	ADDQ R12, R9 \
	ADCQ R13, R10 \
	ADCQ R14, R11 \
	MOVQ ·np+16(SB), AX \
	MULQ 0+stack \
	MOVQ AX, R12 \
	MOVQ DX, R13 \
	MOVQ ·np+16(SB), AX \
	MULQ 8+stack \
	ADDQ AX, R13 \
	ADDQ R12, R10 \
	ADCQ R13, R11 \
	MOVQ ·np+24(SB), AX \
	MULQ 0+stack \
	ADDQ AX, R11 \
	storeBlock(R8,R9,R10,R11, 64+stack) \
	mul(·p2+0(SB),·p2+8(SB),·p2+16(SB),·p2+24(SB), 64+stack, 96+stack) \
	loadBlock(96+stack, R8,R9,R10,R11) \
	loadBlock(128+stack, R12,R13,R14,R15) \
	MOVQ $0, AX \
	ADDQ 0+stack, R8 \
	ADCQ 8+stack, R9 \
	ADCQ 16+stack, R10 \
	ADCQ 24+stack, R11 \
	ADCQ 32+stack, R12 \
	ADCQ 40+stack, R13 \
	ADCQ 48+stack, R14 \
	ADCQ 56+stack, R15 \
	ADCQ $0, AX \
	gfpCarry(R12,R13,R14,R15,AX, R8,R9,R10,R11,BX) \

#define mulBMI2(a0,a1,a2,a3, rb) \
	MOVQ a0, DX \
	MOVQ $0, R13 \
	MULXQ 0+rb, R8, R9 \
	MULXQ 8+rb, AX, R10 \
	ADDQ AX, R9 \
	MULXQ 16+rb, AX, R11 \
	ADCQ AX, R10 \
	MULXQ 24+rb, AX, R12 \
	ADCQ AX, R11 \
	ADCQ $0, R12 \
	ADCQ $0, R13 \
	MOVQ a1, DX \
	MOVQ $0, R14 \
	MULXQ 0+rb, AX, BX \
	ADDQ AX, R9 \
	ADCQ BX, R10 \
	MULXQ 16+rb, AX, BX \
	ADCQ AX, R11 \
	ADCQ BX, R12 \
	ADCQ $0, R13 \
	MULXQ 8+rb, AX, BX \
	ADDQ AX, R10 \
	ADCQ BX, R11 \
	MULXQ 24+rb, AX, BX \
	ADCQ AX, R12 \
	ADCQ BX, R13 \
	ADCQ $0, R14 \
	MOVQ a2, DX \
	MOVQ $0, R15 \
	MULXQ 0+rb, AX, BX \
	ADDQ AX, R10 \
	ADCQ BX, R11 \
	MULXQ 16+rb, AX, BX \
	ADCQ AX, R12 \
	ADCQ BX, R13 \
	ADCQ $0, R14 \
	MULXQ 8+rb, AX, BX \
	ADDQ AX, R11 \
	ADCQ BX, R12 \
	MULXQ 24+rb, AX, BX \
	ADCQ AX, R13 \
	ADCQ BX, R14 \
	ADCQ $0, R15 \
	MOVQ a3, DX \
	MULXQ 0+rb, AX, BX \
	ADDQ AX, R11 \
	ADCQ BX, R12 \
	MULXQ 16+rb, AX, BX \
	ADCQ AX, R13 \
	ADCQ BX, R14 \
	ADCQ $0, R15 \
	MULXQ 8+rb, AX, BX \
	ADDQ AX, R12 \
	ADCQ BX, R13 \
	MULXQ 24+rb, AX, BX \
	ADCQ AX, R14 \
	ADCQ BX, R15 \

#define gfpReduceBMI2() \
	MOVQ ·np+0(SB), DX \
	MULXQ 0(SP), R8, R9 \
	MULXQ 8(SP), AX, R10 \
	ADDQ AX, R9 \
	MULXQ 16(SP), AX, R11 \
	ADCQ AX, R10 \
	MULXQ 24(SP), AX, BX \
	ADCQ AX, R11 \
	MOVQ ·np+8(SB), DX \
	MULXQ 0(SP), AX, BX \
	ADDQ AX, R9 \
	ADCQ BX, R10 \
	MULXQ 16(SP), AX, BX \
	ADCQ AX, R11 \
	MULXQ 8(SP), AX, BX \
	ADDQ AX, R10 \
	ADCQ BX, R11 \
	MOVQ ·np+16(SB), DX \
	MULXQ 0(SP), AX, BX \
	ADDQ AX, R10 \
	ADCQ BX, R11 \
	MULXQ 8(SP), AX, BX \
	ADDQ AX, R11 \
	MOVQ ·np+24(SB), DX \
	MULXQ 0(SP), AX, BX \
	ADDQ AX, R11 \
	storeBlock(R8,R9,R10,R11, 64(SP)) \
	mulBMI2(·p2+0(SB),·p2+8(SB),·p2+16(SB),·p2+24(SB), 64(SP)) \
	MOVQ $0, AX \
	ADDQ 0(SP), R8 \
	ADCQ 8(SP), R9 \
	ADCQ 16(SP), R10 \
	ADCQ 24(SP), R11 \
	ADCQ 32(SP), R12 \
	ADCQ 40(SP), R13 \
	ADCQ 48(SP), R14 \
	ADCQ 56(SP), R15 \
	ADCQ $0, AX \
	gfpCarry(R12,R13,R14,R15,AX, R8,R9,R10,R11,BX) \

TEXT ·gfpNeg(SB),0,$0-16
	MOVQ ·p2+0(SB), R8
	MOVQ ·p2+8(SB), R9
	MOVQ ·p2+16(SB), R10
	MOVQ ·p2+24(SB), R11

	MOVQ a+8(FP), DI
	SUBQ 0(DI), R8
	SBBQ 8(DI), R9
	SBBQ 16(DI), R10
	SBBQ 24(DI), R11

	MOVQ $0, AX
	gfpCarry(R8,R9,R10,R11,AX, R12,R13,R14,R15,BX)

	MOVQ c+0(FP), DI
	storeBlock(R8,R9,R10,R11, 0(DI))
	RET

TEXT ·gfpAdd(SB),0,$0-24
	MOVQ a+8(FP), DI
	MOVQ b+16(FP), SI

	loadBlock(0(DI), R8,R9,R10,R11)
	MOVQ $0, AX

	ADDQ  0(SI), R8
	ADCQ  8(SI), R9
	ADCQ 16(SI), R10
	ADCQ 24(SI), R11
	ADCQ $0, AX

	gfpCarry(R8,R9,R10,R11,AX, R12,R13,R14,R15,BX)

	MOVQ c+0(FP), DI
	storeBlock(R8,R9,R10,R11, 0(DI))
	RET

TEXT ·gfpSub(SB),0,$0-24
	MOVQ a+8(FP), DI
	MOVQ b+16(FP), SI

	loadBlock(0(DI), R8,R9,R10,R11)

	MOVQ ·p2+0(SB), R12
	MOVQ ·p2+8(SB), R13
	MOVQ ·p2+16(SB), R14
	MOVQ ·p2+24(SB), R15
	MOVQ $0, AX

	SUBQ  0(SI), R8
	SBBQ  8(SI), R9
	SBBQ 16(SI), R10
	SBBQ 24(SI), R11

	CMOVQCC AX, R12
	CMOVQCC AX, R13
	CMOVQCC AX, R14
	CMOVQCC AX, R15

	ADDQ R12, R8
	ADCQ R13, R9
	ADCQ R14, R10
	ADCQ R15, R11

	gfpCarry(R8,R9,R10,R11,AX, R12,R13,R14,R15,BX)

	MOVQ c+0(FP), DI
	storeBlock(R8,R9,R10,R11, 0(DI))
	RET

TEXT ·gfpMul(SB),0,$160-24
	MOVQ a+8(FP), DI
	MOVQ b+16(FP), SI

	CMPB ·hasBMI2(SB), $0
	JE   nobmi2Mul

	mulBMI2(0(DI),8(DI),16(DI),24(DI), 0(SI))
	storeBlock( R8, R9,R10,R11,  0(SP))
	storeBlock(R12,R13,R14,R15, 32(SP))
	gfpReduceBMI2()
	JMP end

nobmi2Mul:
	mul(0(DI),8(DI),16(DI),24(DI), 0(SI), 0(SP))
	gfpReduce(0(SP))

end:
	MOVQ c+0(FP), DI
	storeBlock(R12,R13,R14,R15, 0(DI))
	RET
