package core

// segmenter is one sub-segmenter: it inspects the character under the cursor
// and contributes candidate lexemes to the context.
type segmenter interface {
	// analyze reads the character at the context's cursor.
	analyze(context *analyzeContext)
	// reset returns the sub-segmenter to its initial state.
	reset()
}
