package rdp

type CombineSource uint64

const (
	CombineCombined CombineSource = iota
	CombineTex0
	CombineTex1
	CombinePrimitive
	CombineShade
	CombineEnvironment

	CombineAColorOne   CombineSource = 6
	CombineAColorNoise CombineSource = 7
	CombineAColorZero  CombineSource = 8

	CombineAAlphaOne  CombineSource = 6
	CombineAAlphaZero CombineSource = 7

	CombineBColorCenter CombineSource = 6
	CombineBColorK4     CombineSource = 7
	CombineBColorZero   CombineSource = 8

	CombineBAlphaOne  CombineSource = 6
	CombineBAlphaZero CombineSource = 7

	CombineCColorCenter               CombineSource = 6
	CombineCColorCombinedAlpha        CombineSource = 7
	CombineCColorTex0Alpha            CombineSource = 8
	CombineCColorTex1Alpha            CombineSource = 9
	CombineCColorPrimitiveAlpha       CombineSource = 10
	CombineCColorShadeAlpha           CombineSource = 11
	CombineCColorEnvironmentAlpha     CombineSource = 12
	CombineCColorLODFraction          CombineSource = 13
	CombineCColorPrimitiveLODFraction CombineSource = 14
	CombineCColorK5                   CombineSource = 15
	CombineCColorZero                 CombineSource = 16

	CombineCAlphaPrimitiveLODFraction CombineSource = 6
	CombineCAlphaZero                 CombineSource = 7

	CombineDColorOne  CombineSource = 6
	CombineDColorZero CombineSource = 7

	CombineDAlphaOne  CombineSource = 6
	CombineDAlphaZero CombineSource = 7
)

// The ColorCombiner computes it's output with the equation `(A-B)*C + D`, where
// the inputs A, B, C and D can be choosen from the predefined CombineSource
// values. Color and alpha are calculated separately.
// If CycleTypeTwo is active two passes can be defined, where the second pass
// can use the first pass output as it's input.
type CombineMode uint64

func CombineMode1Cycle(rgbA, rgbB, rgbC, rgbD, alphaA, alphaB, alphaC, alphaD CombineSource) CombineMode {
	// Calling CombineMode2Cycle would prevent inlining
	return CombineMode(0 |
		rgbA<<37 | rgbC<<32 | rgbB<<24 | alphaA<<21 |
		alphaC<<18 | rgbD<<6 | alphaB<<3 | alphaD,
	)
}

func CombineMode2Cycle(
	rgbA, rgbB, rgbC, rgbD, alphaA, alphaB, alphaC, alphaD CombineSource,
	rgbA2, rgbB2, rgbC2, rgbD2, alphaA2, alphaB2, alphaC2, alphaD2 CombineSource,
) CombineMode {
	return CombineMode(0 |
		rgbA<<52 | rgbC<<47 | alphaA<<44 | alphaC<<41 |
		rgbA2<<37 | rgbC2<<32 | rgbB<<28 | rgbB2<<24 |
		alphaA2<<21 | alphaC2<<18 | rgbD<<15 | alphaB<<12 |
		alphaD<<9 | rgbD2<<6 | alphaB2<<3 | alphaD2,
	)
}
