package types

type DocciOpts struct {
	HideBackgroundLogs bool
	KeepRunning        bool
	DebugMode          bool
}

// Validation is what docci checks a block's output against after the script runs.
//
// Contains is a hidden tripwire: the reader never sees it, and it exists so the
// page fails when the step stops working. Expect is a checkpoint: the same text
// renders on the page for the reader to compare their terminal against, with
// `<...>` standing for spans that change between runs.
type Validation struct {
	// Label locates the block in the page for the failure message.
	Label string

	Contains string
	Expect   string
}
