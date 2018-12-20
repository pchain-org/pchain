// +build arm64,!generic

#define storeBlock(a0,a1,a2,a3, r) \
	MOVD a0,  0+r \
	MOVD a1,  8+r \
	MOVD a2, 16+r \
	MOVD a3, 24+r \

#define loadBlock(r, a0,a1,a2,a3) \
	MOVD  0+r, a0 \
	MOVD  8+r, a1 \
	MOVD 16+r, a2 \
	MOVD 24+r, a3 \

#define loadModulus(p0,p1,p2,p3) \
	MOVD ·p2+0(SB), p0 \
	MOVD ·p2+8(SB), p1 \
	MOVD ·p2+16(SB), p2 \
	MOVD ·p2+24(SB), p3 \

#define mul(c0,c1,c2,c3,c4,c5,c6,c7) \
	MUL R1, R5, c0 \
	UMULH R1, R5, c1 \
	MUL R1, R6, R0 \
	ADDS R0, c1 \
	UMULH R1, R6, c2 \
	MUL R1, R7, R0 \
	ADCS R0, c2 \
	UMULH R1, R7, c3 \
	MUL R1, R8, R0 \
	ADCS R0, c3 \
	UMULH R1, R8, c4 \
	ADCS ZR, c4 \
	MUL R2, R5, R25 \
	UMULH R2, R5, R26 \
	MUL R2, R6, R0 \
	ADDS R0, R26 \
	UMULH R2, R6, R27 \
	MUL R2, R7, R0 \
	ADCS R0, R27 \
	UMULH R2, R7, R29 \
	MUL R2, R8, R0 \
	ADCS R0, R29 \
	UMULH R2, R8, c5 \
	ADCS ZR, c5 \
	ADDS R25, c1 \
	ADCS R26, c2 \
	ADCS R27, c3 \
	ADCS R29, c4 \
	ADCS  ZR, c5 \
	MUL R3, R5, R25 \
	UMULH R3, R5, R26 \
	MUL R3, R6, R0 \
	ADDS R0, R26 \
	UMULH R3, R6, R27 \
	MUL R3, R7, R0 \
	ADCS R0, R27 \
	UMULH R3, R7, R29 \
	MUL R3, R8, R0 \
	ADCS R0, R29 \
	UMULH R3, R8, c6 \
	ADCS ZR, c6 \
	ADDS R25, c2 \
	ADCS R26, c3 \
	ADCS R27, c4 \
	ADCS R29, c5 \
	ADCS  ZR, c6 \
	MUL R4, R5, R25 \
	UMULH R4, R5, R26 \
	MUL R4, R6, R0 \
	ADDS R0, R26 \
	UMULH R4, R6, R27 \
	MUL R4, R7, R0 \
	ADCS R0, R27 \
	UMULH R4, R7, R29 \
	MUL R4, R8, R0 \
	ADCS R0, R29 \
	UMULH R4, R8, c7 \
	ADCS ZR, c7 \
	ADDS R25, c3 \
	ADCS R26, c4 \
	ADCS R27, c5 \
	ADCS R29, c6 \
	ADCS  ZR, c7 \

#define gfpReduce() \
	MOVD ·np+0(SB), R17 \
	MOVD ·np+8(SB), R18 \
	MOVD ·np+16(SB), R19 \
	MOVD ·np+24(SB), R20 \
	MUL R9, R17, R1 \
	UMULH R9, R17, R2 \
	MUL R9, R18, R0 \
	ADDS R0, R2 \
	UMULH R9, R18, R3 \
	MUL R9, R19, R0 \
	ADCS R0, R3 \
	UMULH R9, R19, R4 \
	MUL R9, R20, R0 \
	ADCS R0, R4 \
	MUL R10, R17, R21 \
	UMULH R10, R17, R22 \
	MUL R10, R18, R0 \
	ADDS R0, R22 \
	UMULH R10, R18, R23 \
	MUL R10, R19, R0 \
	ADCS R0, R23 \
	ADDS R21, R2 \
	ADCS R22, R3 \
	ADCS R23, R4 \
	MUL R11, R17, R21 \
	UMULH R11, R17, R22 \
	MUL R11, R18, R0 \
	ADDS R0, R22 \
	ADDS R21, R3 \
	ADCS R22, R4 \
	MUL R12, R17, R21 \
	ADDS R21, R4 \
	loadModulus(R5,R6,R7,R8) \
	mul(R17,R18,R19,R20,R21,R22,R23,R24) \
	MOVD  ZR, R25 \
	ADDS  R9, R17 \
	ADCS R10, R18 \
	ADCS R11, R19 \
	ADCS R12, R20 \
	ADCS R13, R21 \
	ADCS R14, R22 \
	ADCS R15, R23 \
	ADCS R16, R24 \
	ADCS  ZR, R25 \
	SUBS R5, R21, R10 \
	SBCS R6, R22, R11 \
	SBCS R7, R23, R12 \
	SBCS R8, R24, R13 \
	CSEL CS, R10, R21, R1 \
	CSEL CS, R11, R22, R2 \
	CSEL CS, R12, R23, R3 \
	CSEL CS, R13, R24, R4 \

TEXT ·gfpNeg(SB),0,$0-16
	MOVD a+8(FP), R0
	loadBlock(0(R0), R1,R2,R3,R4)
	loadModulus(R5,R6,R7,R8)

	SUBS R1, R5, R1
	SBCS R2, R6, R2
	SBCS R3, R7, R3
	SBCS R4, R8, R4

	SUBS R5, R1, R5
	SBCS R6, R2, R6
	SBCS R7, R3, R7
	SBCS R8, R4, R8

	CSEL CS, R5, R1, R1
	CSEL CS, R6, R2, R2
	CSEL CS, R7, R3, R3
	CSEL CS, R8, R4, R4

	MOVD c+0(FP), R0
	storeBlock(R1,R2,R3,R4, 0(R0))
	RET

TEXT ·gfpAdd(SB),0,$0-24
	MOVD a+8(FP), R0
	loadBlock(0(R0), R1,R2,R3,R4)
	MOVD b+16(FP), R0
	loadBlock(0(R0), R5,R6,R7,R8)
	loadModulus(R9,R10,R11,R12)
	MOVD ZR, R0

	ADDS R5, R1
	ADCS R6, R2
	ADCS R7, R3
	ADCS R8, R4
	ADCS ZR, R0

	SUBS  R9, R1, R5
	SBCS R10, R2, R6
	SBCS R11, R3, R7
	SBCS R12, R4, R8
	SBCS  ZR, R0, R0

	CSEL CS, R5, R1, R1
	CSEL CS, R6, R2, R2
	CSEL CS, R7, R3, R3
	CSEL CS, R8, R4, R4

	MOVD c+0(FP), R0
	storeBlock(R1,R2,R3,R4, 0(R0))
	RET

TEXT ·gfpSub(SB),0,$0-24
	MOVD a+8(FP), R0
	loadBlock(0(R0), R1,R2,R3,R4)
	MOVD b+16(FP), R0
	loadBlock(0(R0), R5,R6,R7,R8)
	loadModulus(R9,R10,R11,R12)

	SUBS R5, R1
	SBCS R6, R2
	SBCS R7, R3
	SBCS R8, R4

	CSEL CS, ZR,  R9,  R9
	CSEL CS, ZR, R10, R10
	CSEL CS, ZR, R11, R11
	CSEL CS, ZR, R12, R12

	ADDS  R9, R1
	ADCS R10, R2
	ADCS R11, R3
	ADCS R12, R4

	MOVD c+0(FP), R0
	storeBlock(R1,R2,R3,R4, 0(R0))
	RET

TEXT ·gfpMul(SB),0,$0-24
	MOVD a+8(FP), R0
	loadBlock(0(R0), R1,R2,R3,R4)
	MOVD b+16(FP), R0
	loadBlock(0(R0), R5,R6,R7,R8)

	mul(R9,R10,R11,R12,R13,R14,R15,R16)
	gfpReduce()

	MOVD c+0(FP), R0
	storeBlock(R1,R2,R3,R4, 0(R0))
	RET
