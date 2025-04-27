package rdp

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
type CombineMode struct{ One, Two CombinePass }
type CombinePass struct{ RGB, Alpha CombineParams }
type CombineParams struct{ A, B, C, D CombineSource }
type CombineSource uint64
